package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	mo "github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMInfo is the inventory record returned by ListVMs. All byte values are in
// raw bytes; callers format them via FormatBytes / FormatGB for display.
type VMInfo struct {
	Name     string `json:"name"`
	VCPUs    int32  `json:"vcpus"`
	MemoryMB int64  `json:"memory_mb"` // configured RAM in MiB
	StorageB int64  `json:"storage_b"` // committed (consumed) storage on disk, bytes
}

// ListVMs enumerates every virtual machine in the inventory and returns their
// vCPU count, configured memory, and actual consumed (committed) storage. The
// result is sorted by VM name for deterministic tabular output.
func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	m := view.NewManager(c)

	vcv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"VirtualMachine"},
		true, // recursive
	)
	if err != nil {
		return nil, fmt.Errorf("creating VM container view: %w", err)
	}
	defer vcv.Destroy(ctx)

	vmRefs, err := vcv.Find(ctx, []string{"VirtualMachine"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding virtual machines: %w", err)
	}

	pc := property.DefaultCollector(c)

	var vms []mo.VirtualMachine
	if len(vmRefs) > 0 {
		if err := pc.Retrieve(ctx, vmRefs, []string{"summary"}, &vms); err != nil {
			return nil, fmt.Errorf("batch retrieve VM summaries: %w", err)
		}
	}

	var infos []VMInfo
	for i := range vms {
		info, err := sumFromMo(&vms[i])
		if err != nil {
			continue // skip unreadable VMs; do not fail the whole query
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// sumFromMo extracts a VMInfo from a populated mo.VirtualMachine. The summary
// already contains Name, NumCpu, MemorySizeMB and Storage.Committed so we do
// not need to retrieve the full Config.
func sumFromMo(vm *mo.VirtualMachine) (VMInfo, error) {
	if vm == nil {
		return VMInfo{}, fmt.Errorf("nil VM object")
	}

	sum := vm.Summary
	vcpu := int32(0)
	memMB := int64(0)
	name := ""
	if cfg := sum.Config; cfg.NumCpu > 0 || cfg.MemorySizeMB > 0 {
		vcpu = cfg.NumCpu
		memMB = int64(cfg.MemorySizeMB)
		name = cfg.Name
	}

	storedB := int64(0)
	if sum.Storage != nil {
		storedB = sum.Storage.Committed
	}

	return VMInfo{
		Name:     name,
		VCPUs:    vcpu,
		MemoryMB: memMB,
		StorageB: storedB,
	}, nil
}

// ListVMsByPortGroup returns the VMs whose virtual NICs are connected to a
// port group matching pgName. The match is exact on the Network object's name
// (for standard PGs) or on the DVS Portgroup's Name() method (for distributed
// PGs). Both kinds of backing are handled transparently.
func ListVMsByPortGroup(ctx context.Context, c *vim25.Client, pgName string) ([]VMInfo, error) {
	m := view.NewManager(c)

	vcv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"VirtualMachine"},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("creating VM container view: %w", err)
	}
	defer vcv.Destroy(ctx)

	vmRefs, err := vcv.Find(ctx, []string{"VirtualMachine"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding virtual machines: %w", err)
	}

	pc := property.DefaultCollector(c)

	// Step 1 — batch-fetch summaries for every VM.
	var allVms []mo.VirtualMachine
	if len(vmRefs) > 0 {
		if err := pc.Retrieve(ctx, vmRefs, []string{"summary"}, &allVms); err != nil {
			return nil, fmt.Errorf("batch retrieve VM summaries: %w", err)
		}
	}

	vmInfoByRef := make(map[string]VMInfo, len(allVms))
	for i := range allVms {
		info, err := sumFromMo(&allVms[i])
		if err != nil {
			continue
		}
		vmInfoByRef[allVms[i].Self.Value] = info
	}

	// Step 2 — batch-fetch config.hardware.device for every VM.
	var allDevices []mo.VirtualMachine
	if len(vmRefs) > 0 {
		if err := pc.Retrieve(ctx, vmRefs, []string{"config.hardware.device"}, &allDevices); err != nil {
			return nil, fmt.Errorf("batch retrieve NIC backings: %w", err)
		}
	}

	var stdNetRefs []types.ManagedObjectReference
	nicMap := make(map[string][]nicBacking) // vm ref value -> its nic backings
	for i := range vmRefs {
		backings := parseNicBackings(&allDevices[i])
		nicMap[allDevices[i].Self.Value] = backings
		for _, b := range backings {
			if b.networkRef != nil {
				stdNetRefs = append(stdNetRefs, *b.networkRef)
			}
		}
	}

	// Step 3 — batch-fetch names of every standard network referenced.
	netNameByValue := make(map[string]string)
	if len(stdNetRefs) > 0 {
		var nets []mo.Network
		if err := pc.Retrieve(ctx, stdNetRefs, []string{"name"}, &nets); err != nil {
			return nil, fmt.Errorf("batch retrieve network names: %w", err)
		}
		for i, n := range nets {
			if n.Name != "" && i < len(stdNetRefs) {
				netNameByValue[nets[i].Self.Value] = n.Name
			}
		}
	}

	// Step 4 — resolve DVS port group key -> name map.
	dvsPgMap, err := listDVSPortGroupNames(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("resolving DVS port group names: %w", err)
	}

	var matched []VMInfo
	for i := range vmRefs {
		selfValue := allVms[i].Self.Value
		info, ok := vmInfoByRef[selfValue]
		if !ok {
			continue
		}
		if matchAgainstPG(nicMap[selfValue], stdNetRefs, netNameByValue, dvsPgMap, pgName) {
			matched = append(matched, info)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})

	return matched, nil
}

// nicBacking carries the parsed backing reference of a single NIC so that we
// can later check whether it matches a target port-group name.
type nicBacking struct {
	networkRef      *types.ManagedObjectReference // standard PG: ref to Network object
	dvsPortgroupKey string                        // DVS PG: key string (e.g. "grp-12345")
}

// parseNicBackings extracts NIC backings from a populated mo.VirtualMachine.
func parseNicBackings(vm *mo.VirtualMachine) []nicBacking {
	var out []nicBacking
	if vm == nil || vm.Config == nil {
		return out
	}

	for _, dev := range vm.Config.Hardware.Device {
		nic, ok := dev.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}

		vc := nic.GetVirtualEthernetCard()
		if vc == nil || vc.Backing == nil {
			continue
		}

		b := nicBacking{}

		switch binfo := vc.Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if binfo.Network != nil {
				ref := *binfo.Network
				b.networkRef = &ref
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			b.dvsPortgroupKey = binfo.Port.PortgroupKey
		default:
			continue
		}

		out = append(out, b)
	}

	return out
}

// matchAgainstPG returns true if any of the VM's NIC backings connect to the
// target port group — either by exact standard-network name or by DVS portgroup key.
func matchAgainstPG(backings []nicBacking, stdNetRefs []types.ManagedObjectReference, netNameByValue map[string]string, dvsPgMap map[string]string, pg string) bool {
	for _, b := range backings {
		if b.networkRef != nil {
			if name, ok := netNameByValue[b.networkRef.Value]; ok && name == pg {
				return true
			}
		}
		if b.dvsPortgroupKey != "" {
			if mapped := dvsPgMap[b.dvsPortgroupKey]; mapped == pg {
				return true
			}
		}
	}
	return false
}

// listDVSPortGroupNames builds a map of DVS port group key -> name by walking
// every DistributedVirtualSwitch in the inventory and listing its PortGroups.
func listDVSPortGroupNames(ctx context.Context, c *vim25.Client) (map[string]string, error) {
	m := view.NewManager(c)
	cv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"DistributedVirtualSwitch"},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("creating DVS container view: %w", err)
	}
	defer cv.Destroy(ctx)

	dvsRefs, err := cv.Find(ctx, []string{"DistributedVirtualSwitch"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding distributed virtual switches: %w", err)
	}

	pc := property.DefaultCollector(c)

	var dvss []mo.DistributedVirtualSwitch
	if len(dvsRefs) > 0 {
		if err := pc.Retrieve(ctx, dvsRefs, []string{"name", "portgroup"}, &dvss); err != nil {
			return nil, fmt.Errorf("batch retrieve DVS objects: %w", err)
		}
	}

	pgRefs := make([]types.ManagedObjectReference, 0)
	for _, dvs := range dvss {
		pgRefs = append(pgRefs, dvs.Portgroup...)
	}

	out := make(map[string]string)
	if len(pgRefs) > 0 {
		var pgs []mo.DistributedVirtualPortgroup
		if err := pc.Retrieve(ctx, pgRefs, []string{"name"}, &pgs); err != nil {
			return nil, fmt.Errorf("batch retrieve DVS port groups: %w", err)
		}
		for i, pg := range pgs {
			if pg.Name != "" && i < len(pgRefs) {
				out[pgRefs[i].Value] = pg.Name
			}
		}
	}

	return out, nil
}
