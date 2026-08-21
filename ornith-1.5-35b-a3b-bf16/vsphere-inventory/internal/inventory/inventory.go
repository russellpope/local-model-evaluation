// Package inventory retrieves vSphere inventory (virtual machines, datastores,
// and networking) from a govmomi client. The retrieval functions take a
// context and a *vim25.Client and return typed results, keeping them decoupled
// from the CLI wiring and table presentation.
package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMInfo is the reported state of a single virtual machine.
type VMInfo struct {
	Name         string
	VCPUs        int32
	RAMGB        float64
	StorageBytes int64 // committed (consumed) storage across all datastores, in bytes
}

// DatastoreInfo is the reported state of a single datastore.
type DatastoreInfo struct {
	Name           string
	Type           string // transport: FC, iSCSI, NVMe, NFS, or unknown
	CapacityBytes  int64
	AvailableBytes int64
	UsedBytes      int64
	FileSystemType string
}

// SwitchPortGroupInfo is one row of the vswitches table: a port group and the
// switch-level attributes repeated for that row.
type SwitchPortGroupInfo struct {
	Switch     string
	SwitchType string // "standard" or "distributed"
	PortGroup  string
	VLAN       string
	Uplinks    string
	LACP       string // "enabled", "disabled", or "N/A"
	Ports      int32
	UsedPorts  int32
}

// PortGroupVM identifies a virtual machine connected to a port group.
type PortGroupVM struct {
	Name string
}

// newFinder returns a finder with its default datacenter resolved, so that
// wildcard lookups (* ) scope to a single datacenter.
func newFinder(ctx context.Context, client *vim25.Client) (*find.Finder, error) {
	f := find.NewFinder(client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolving default datacenter: %w", err)
	}
	return f.SetDatacenter(dc), nil
}

// ListVMs returns one VMInfo per virtual machine in the inventory, sorted by name.
func ListVMs(ctx context.Context, client *vim25.Client) ([]VMInfo, error) {
	finder, err := newFinder(ctx, client)
	if err != nil {
		return nil, err
	}
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing virtual machines: %w", err)
	}

	result := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		var moVM mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), []string{"summary", "config"}, &moVM); err != nil {
			return nil, fmt.Errorf("reading properties of virtual machine %q: %w", vmName(vm, &moVM), err)
		}

		var committed int64
		if moVM.Summary.Storage != nil {
			committed = moVM.Summary.Storage.Committed
		}

		result = append(result, VMInfo{
			Name:         vmName(vm, &moVM),
			VCPUs:        moVM.Config.Hardware.NumCPU,
			RAMGB:        gbFromMB(int64(moVM.Config.Hardware.MemoryMB)),
			StorageBytes: committed,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func vmName(vm *object.VirtualMachine, moVM *mo.VirtualMachine) string {
	if moVM.Config.Name != "" {
		return moVM.Config.Name
	}
	if moVM.Summary.Config.Name != "" {
		return moVM.Summary.Config.Name
	}
	return vm.InventoryPath
}

// ListDatastores returns one DatastoreInfo per datastore, sorted by name.
func ListDatastores(ctx context.Context, client *vim25.Client) ([]DatastoreInfo, error) {
	finder, err := newFinder(ctx, client)
	if err != nil {
		return nil, err
	}
	dss, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datastores: %w", err)
	}

	result := make([]DatastoreInfo, 0, len(dss))
	for _, ds := range dss {
		var moDS mo.Datastore
		if err := ds.Properties(ctx, ds.Reference(), []string{"summary", "info", "host"}, &moDS); err != nil {
			return nil, fmt.Errorf("reading properties of datastore %q: %w", ds.Name(), err)
		}

		capacity := moDS.Summary.Capacity
		free := moDS.Summary.FreeSpace
		if free > capacity {
			free = capacity
		}

		dsType := deriveTransport(ctx, client, moDS)

		result = append(result, DatastoreInfo{
			Name:           moDS.Summary.Name,
			Type:           dsType,
			FileSystemType: moDS.Summary.Type,
			CapacityBytes:  capacity,
			AvailableBytes: free,
			UsedBytes:      usedBytes(capacity, free),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// deriveTransport determines a datastore's storage transport. NFS is decided by
// the filesystem type; otherwise the backing device's host bus adapter is used,
// degrading to "unknown" when the API does not expose enough topology.
func deriveTransport(ctx context.Context, client *vim25.Client, ds mo.Datastore) string {
	if strings.EqualFold(ds.Summary.Type, "NFS") {
		return "NFS"
	}

	for i := range ds.Host {
		mount := ds.Host[i]
		var moHost mo.HostSystem
		h := object.NewHostSystem(client, mount.Key)
		if err := h.Properties(ctx, mount.Key, []string{"config.storageDevice"}, &moHost); err != nil {
			continue
		}
		if t := transportFromHost(ds.Summary.Name, moHost.Config.StorageDevice); t != "unknown" {
			return t
		}
	}
	return "unknown"
}

// transportFromHost maps a datastore to a transport using a host's storage
// topology: host bus adapters and multipath paths reveal which adapter (and
// thus which transport) presents the datastore's backing device.
func transportFromHost(dsName string, sd *types.HostStorageDeviceInfo) string {
	if sd == nil {
		return "unknown"
	}

	adapterType := make(map[string]string)
	for _, a := range sd.HostBusAdapter {
		switch a.(type) {
		case *types.HostFibreChannelHba:
			adapterType[hostAdapterKey(a)] = "fc"
		case *types.HostInternetScsiHba:
			adapterType[hostAdapterKey(a)] = "iscsi"
		}
	}
	if len(adapterType) == 0 {
		return "unknown"
	}

	// Map each LUN (backing device) to a transport via its multipath paths.
	deviceType := make(map[string]string)
	if sd.MultipathInfo != nil {
		for _, lun := range sd.MultipathInfo.Lun {
			for _, p := range lun.Path {
				if t, ok := adapterType[p.Adapter]; ok {
					deviceType[lun.Lun] = t
				}
			}
		}
	}

	// A VMFS datastore is often named after its backing device.
	if t, ok := deviceType[dsName]; ok {
		return t
	}

	return singleTransport(adapterType)
}

func hostAdapterKey(a types.BaseHostHostBusAdapter) string {
	switch v := a.(type) {
	case *types.HostHostBusAdapter:
		return v.Key
	default:
		return ""
	}
}

// singleTransport returns the unique transport type in the map, or "unknown" if
// the host presents more than one.
func singleTransport(adapterType map[string]string) string {
	t := ""
	for _, v := range adapterType {
		if t != "" && t != v {
			return "unknown"
		}
		t = v
	}
	return t
}
