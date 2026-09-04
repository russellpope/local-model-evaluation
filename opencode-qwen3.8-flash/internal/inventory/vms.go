package inventory

import (
	"context"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMInfo holds the per-VM fields shown by the vms subcommand.
type VMInfo struct {
	Name   string
	NumCPU int32
	// MemoryMB is the configured memory in mebibibytes.
	MemoryMB int64
	// Committed is actual storage consumed by the VM, in bytes. This is
	// committed (used) storage, not provisioned capacity.
	Committed int64
}

// FetchVMs returns every virtual machine in the inventory, sorted by name.
func FetchVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	var vms []mo.VirtualMachine
	if err := retrieve(ctx, c, "VirtualMachine", []string{"name", "summary"}, &vms); err != nil {
		return nil, err
	}
	return collectVMs(vms), nil
}

func collectVMs(vms []mo.VirtualMachine) []VMInfo {
	out := make([]VMInfo, 0, len(vms))
	for i := range vms {
		vm := &vms[i]
		info := VMInfo{Name: vm.Name}
		info.NumCPU = vm.Summary.Config.NumCpu
		info.MemoryMB = int64(vm.Summary.Config.MemorySizeMB)
		if vm.Config != nil {
			if info.NumCPU == 0 {
				info.NumCPU = vm.Config.Hardware.NumCPU
			}
			if info.MemoryMB == 0 {
				info.MemoryMB = int64(vm.Config.Hardware.MemoryMB)
			}
		}
		if vm.Summary.Storage != nil {
			info.Committed = vm.Summary.Storage.Committed
		}
		out = append(out, info)
	}
	sortVMs(out)
	return out
}

func sortVMs(vms []VMInfo) {
	sort.Slice(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })
}

// vmDeviceProps are the properties needed to inspect VM network adapters.
var vmDeviceProps = []string{"name", "summary", "config.hardware.device"}

func fetchVMsWithDevices(ctx context.Context, c *vim25.Client) ([]mo.VirtualMachine, error) {
	var vms []mo.VirtualMachine
	if err := retrieve(ctx, c, "VirtualMachine", vmDeviceProps, &vms); err != nil {
		return nil, err
	}
	return vms, nil
}

// vmEthernetBackings returns the portgroup name(s) each VM adapter is attached
// to. Standard portgroups match on the adapter's device name; distributed
// portgroups resolve through pgKeyToName.
func vmEthernetBackings(vm mo.VirtualMachine, pgKeyToName map[string]string) []string {
	var names []string
	if vm.Config == nil {
		return nil
	}
	for _, dev := range vm.Config.Hardware.Device {
		nic, ok := dev.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		switch backing := nic.GetVirtualEthernetCard().Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if backing.DeviceName != "" {
				names = append(names, backing.DeviceName)
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			if name, ok := pgKeyToName[backing.Port.PortgroupKey]; ok {
				names = append(names, name)
			}
		}
	}
	return names
}
