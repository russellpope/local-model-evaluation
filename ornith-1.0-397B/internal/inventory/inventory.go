package inventory

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/vsphere-cli/internal/formatter"
	"github.com/local-model-evaluation/vsphere-cli/internal/transport"
)

// VMInfo holds information about a virtual machine.
type VMInfo struct {
	Name    string
	VCPU    int32
	RAMMB   int32
	Storage int64 // committed bytes
}

// DatastoreInfo holds information about a datastore.
type DatastoreInfo struct {
	Name      string
	Type      string // FC, iSCSI, NVMe, NFS, unknown
	Used      int64
	Available int64
}

// SwitchInfo holds information about a virtual switch and its port groups.
type SwitchInfo struct {
	SwitchName string
	SwitchType string // "standard" or "distributed"
	PortGroups []PortGroupInfo
	Uplinks    string
	LACP       string
	Ports      int32
	UsedPorts  int32
}

// PortGroupInfo holds information about a port group.
type PortGroupInfo struct {
	Name string
	VLAN string
}

// NewClient creates an authenticated govmomi client.
func NewClient(ctx context.Context, u *url.URL, insecure bool) (*govmomi.Client, error) {
	client, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("create govmomi client: %w", err)
	}
	return client, nil
}

// GetVMs retrieves all virtual machines with their inventory info.
func GetVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	finder := find.NewFinder(c, true)

	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("find default datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	var results []VMInfo
	for _, vm := range vms {
		var props mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), []string{
			"name", "config.hardware.numCPU", "config.hardware.memoryMB",
			"summary.storage.committed",
		}, &props); err != nil {
			return nil, fmt.Errorf("get properties for VM %s: %w", vm.Name(), err)
		}

		info := VMInfo{
			Name:  props.Name,
			VCPU:  props.Config.Hardware.NumCPU,
			RAMMB: props.Config.Hardware.MemoryMB,
		}
		info.Storage = props.Summary.Storage.Committed
		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// GetDatastores retrieves all datastores with capacity and transport info.
func GetDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	finder := find.NewFinder(c, true)

	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("find default datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	dss, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}

	var results []DatastoreInfo
	for _, ds := range dss {
		var props mo.Datastore
		if err := ds.Properties(ctx, ds.Reference(), []string{
			"name", "summary.capacity", "summary.freeSpace",
			"summary.type", "info",
		}, &props); err != nil {
			return nil, fmt.Errorf("get properties for datastore %s: %w", ds.Name(), err)
		}

		info := DatastoreInfo{
			Name:      props.Name,
			Available: props.Summary.FreeSpace,
		}
		if props.Summary.Capacity > 0 {
			info.Used = formatter.UsedCapacity(props.Summary.Capacity, props.Summary.FreeSpace)
		}

		// Determine transport type
		isNFS := props.Summary.Type == "NFS" || props.Summary.Type == "NFS41"
		if isNFS {
			info.Type = "NFS"
		} else {
			hbaTypes, deviceNames := extractBackingInfo(props)
			info.Type = transport.ClassifyDatastoreTransport(false, hbaTypes, deviceNames)
		}

		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

func extractBackingInfo(ds mo.Datastore) (hbaTypes []string, deviceNames []string) {
	if ds.Info == nil {
		return nil, nil
	}
	switch info := ds.Info.(type) {
	case *types.VmfsDatastoreInfo:
		if info.Vmfs != nil {
			for _, extent := range info.Vmfs.Extent {
				deviceNames = append(deviceNames, extent.DiskName)
			}
		}
	case *types.NasDatastoreInfo:
		// NFS — handled at caller level
	}
	return hbaTypes, deviceNames
}

// GetVSwitches retrieves all virtual switches (standard and distributed).
func GetVSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var results []SwitchInfo

	stdSwitches, err := getStandardVSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("get standard vswitches: %w", err)
	}
	results = append(results, stdSwitches...)

	dvSwitches, err := getDistributedVSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("get distributed vswitches: %w", err)
	}
	results = append(results, dvSwitches...)

	return results, nil
}

func getStandardVSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(c, true)

	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, nil
	}
	finder.SetDatacenter(dc)

	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var results []SwitchInfo

	for _, host := range hosts {
		var props mo.HostSystem
		if err := host.Properties(ctx, host.Reference(), []string{
			"name", "config.network.vswitch", "config.network.portgroup",
		}, &props); err != nil {
			continue
		}

		for _, vs := range props.Config.Network.Vswitch {
			name := vs.Name
			if seen[name] {
				continue
			}
			seen[name] = true

			si := SwitchInfo{
				SwitchName: name,
				SwitchType: "standard",
				LACP:       "N/A",
				Uplinks:    strings.Join(vs.Pnic, ", "),
				Ports:      vs.NumPorts,
				UsedPorts:  vs.NumPorts - vs.NumPortsAvailable,
			}

			// Find port groups on this vswitch
			for _, pg := range props.Config.Network.Portgroup {
				if pg.Spec.VswitchName == name {
					pgInfo := PortGroupInfo{
						Name: pg.Spec.Name,
						VLAN: formatVLAN(pg.Spec.VlanId),
					}
					si.PortGroups = append(si.PortGroups, pgInfo)
				}
			}

			results = append(results, si)
		}
	}

	return results, nil
}

func getDistributedVSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	// Use NetworkList to find DVS objects
	finder := find.NewFinder(c, true)

	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, nil
	}
	finder.SetDatacenter(dc)

	nets, err := finder.NetworkList(ctx, "*")
	if err != nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var results []SwitchInfo

	for _, net := range nets {
		// Check if it's a DVS
		dvs, ok := net.(*object.DistributedVirtualSwitch)
		if !ok {
			continue
		}

		var props mo.DistributedVirtualSwitch
		if err := dvs.Properties(ctx, dvs.Reference(), []string{
			"name", "config", "portgroup",
		}, &props); err != nil {
			continue
		}

		if seen[props.Name] {
			continue
		}
		seen[props.Name] = true

		si := SwitchInfo{
			SwitchName: props.Name,
			SwitchType: "distributed",
		}

		// LACP and uplinks require VMwareDVSConfigInfo
		if vmwareCfg, ok := props.Config.(*types.VMwareDVSConfigInfo); ok {
			if vmwareCfg.LacpApiVersion != "" {
				si.LACP = "enabled"
			} else {
				si.LACP = "disabled"
			}

			if uplinkPolicy, ok := vmwareCfg.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok {
				si.Uplinks = strings.Join(uplinkPolicy.UplinkPortName, ", ")
			}

			if vmwareCfg.NumPorts > 0 {
				si.Ports = vmwareCfg.NumPorts
			}
		} else {
			si.LACP = "disabled"
		}

		// Port groups
		for _, pgRef := range props.Portgroup {
			var pgProps mo.DistributedVirtualPortgroup
			if err := property.DefaultCollector(c).RetrieveOne(ctx, pgRef, []string{
				"name",
			}, &pgProps); err != nil {
				continue
			}
			pgInfo := PortGroupInfo{
				Name: pgProps.Name,
			}
			si.PortGroups = append(si.PortGroups, pgInfo)
		}

		results = append(results, si)
	}

	return results, nil
}

func formatVLAN(vlanID int32) string {
	switch {
	case vlanID == 0:
		return "0"
	case vlanID == 4095:
		return "trunk (all)"
	case vlanID == 4094:
		return "private VLAN"
	default:
		return fmt.Sprintf("%d", vlanID)
	}
}

// GetVMsByPortGroup returns VMs connected to a named port group.
func GetVMsByPortGroup(ctx context.Context, c *vim25.Client, portGroupName string) ([]VMInfo, error) {
	finder := find.NewFinder(c, true)

	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, nil
	}
	finder.SetDatacenter(dc)

	var vmRefs []types.ManagedObjectReference

	// Check distributed port groups
	nets, err := finder.NetworkList(ctx, "*")
	if err == nil {
		for _, net := range nets {
			dvs, ok := net.(*object.DistributedVirtualSwitch)
			if !ok {
				continue
			}
			var dvsProps mo.DistributedVirtualSwitch
			if err := dvs.Properties(ctx, dvs.Reference(), []string{"portgroup"}, &dvsProps); err != nil {
				continue
			}
			for _, pgRef := range dvsProps.Portgroup {
				var pgProps mo.DistributedVirtualPortgroup
				if err := property.DefaultCollector(c).RetrieveOne(ctx, pgRef, []string{"name"}, &pgProps); err != nil {
					continue
				}
				if pgProps.Name == portGroupName {
					vms, err := getVMsForDistributedPortGroup(ctx, c, pgRef)
					if err != nil {
						continue
					}
					vmRefs = append(vmRefs, vms...)
				}
			}
		}
	}

	// Check all VMs and their network connections
	allVMs, err := finder.VirtualMachineList(ctx, "*")
	if err == nil {
		for _, vm := range allVMs {
			var props mo.VirtualMachine
			if err := vm.Properties(ctx, vm.Reference(), []string{"network"}, &props); err != nil {
				continue
			}
			for _, net := range props.Network {
				// Check if this network name matches the port group name
				var netProps mo.Network
				if err := property.DefaultCollector(c).RetrieveOne(ctx, net, []string{"name"}, &netProps); err != nil {
					continue
				}
				if netProps.Name == portGroupName {
					vmRefs = append(vmRefs, vm.Reference())
				}
			}
		}
	}

	if len(vmRefs) == 0 {
		return nil, nil
	}

	// Deduplicate
	seen := make(map[types.ManagedObjectReference]bool)
	var uniqueRefs []types.ManagedObjectReference
	for _, ref := range vmRefs {
		if !seen[ref] {
			seen[ref] = true
			uniqueRefs = append(uniqueRefs, ref)
		}
	}

	// Fetch VM info
	pc := property.DefaultCollector(c)
	var vmProps []mo.VirtualMachine
	if err := pc.Retrieve(ctx, uniqueRefs, []string{
		"name", "config.hardware.numCPU", "config.hardware.memoryMB",
		"summary.storage.committed",
	}, &vmProps); err != nil {
		return nil, fmt.Errorf("retrieve VM properties: %w", err)
	}

	var results []VMInfo
	for _, vm := range vmProps {
		info := VMInfo{
			Name:  vm.Name,
			VCPU:  vm.Config.Hardware.NumCPU,
			RAMMB: vm.Config.Hardware.MemoryMB,
		}
		info.Storage = vm.Summary.Storage.Committed
		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

func getVMsForDistributedPortGroup(ctx context.Context, c *vim25.Client, pgRef types.ManagedObjectReference) ([]types.ManagedObjectReference, error) {
	finder := find.NewFinder(c, true)
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var refs []types.ManagedObjectReference
	for _, vm := range vms {
		var props mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), []string{"network"}, &props); err != nil {
			continue
		}
		for _, net := range props.Network {
			if net == pgRef {
				refs = append(refs, vm.Reference())
				break
			}
		}
	}
	return refs, nil
}
