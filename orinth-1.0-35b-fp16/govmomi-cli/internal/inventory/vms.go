package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	mo "github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vim25"
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

	var infos []VMInfo

	for _, ref := range vmRefs {
		info, err := fetchVMSummary(ctx, pc, ref)
		if err != nil {
			return nil, fmt.Errorf("fetching summary for VM %s: %w", ref.String(), err)
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// fetchVMSummary pulls Summary from a single VM via one property call. The
// summary already contains Name, NumCpu, MemorySizeMB and Storage.Committed so
// we do not need to retrieve the full Config.
func fetchVMSummary(ctx context.Context, pc *property.Collector, ref types.ManagedObjectReference) (VMInfo, error) {
	var vm mo.VirtualMachine
	if err := pc.RetrieveOne(ctx, ref, []string{"summary"}, &vm); err != nil {
		return VMInfo{}, fmt.Errorf("retrieve summary: %w", err)
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
		Name:      name,
		VCPUs:     vcpu,
		MemoryMB:  memMB,
		StorageB:  storedB,
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

	dvsPgMap, err := listDVSPortGroupNames(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("resolving DVS port group names: %w", err)
	}

	// Pre-fetch summaries for every VM so matchingVM can return full info without
	// an extra property call per reference.
	vmInfoCache := make(map[string]VMInfo, len(vmRefs))
	for _, ref := range vmRefs {
		info, err := fetchVMSummary(ctx, pc, ref)
		if err != nil {
			continue
		}
		vmInfoCache[ref.Value] = info
	}

	var matched []VMInfo

	for _, ref := range vmRefs {
		nics, err := fetchVMMacBackings(ctx, pc, ref)
		if err != nil {
			continue // skip VMs we cannot read; do not fail the whole query
		}

		matched = append(matched, nics.matchingVM(c, pgName, dvsPgMap, vmInfoCache[ref.Value])...)
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

// nicsResult is the parsed NIC list for a single VM, used to match against a
// target port-group name without needing an additional property lookup per VM.
type nicsResult struct {
	backings []nicBacking
}

func fetchVMMacBackings(ctx context.Context, pc *property.Collector, ref types.ManagedObjectReference) (nicsResult, error) {
	var vm mo.VirtualMachine
	if err := pc.RetrieveOne(ctx, ref, []string{"config.hardware.device"}, &vm); err != nil {
		return nicsResult{}, err
	}

	r := nicsResult{}

	if vm.Config == nil {
		return r, nil
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

		r.backings = append(r.backings, b)
	}

	return r, nil
}

// matchingVM returns a VMInfo (or empty slice if no NIC matches the target PG).
// vmInfo is pre-fetched from fetchVMSummary so we can return a complete record.
func (r nicsResult) matchingVM(c *vim25.Client, n string, pgKeyNames map[string]string, vmInfo VMInfo) []VMInfo {
	pc := property.DefaultCollector(c)

	for _, b := range r.backings {
		if b.networkRef != nil {
			var net mo.Network
			if err := pc.RetrieveOne(context.TODO(), *b.networkRef, []string{"name"}, &net); err == nil && net.Name == n {
				return []VMInfo{vmInfo}
			}
		}
		if b.dvsPortgroupKey != "" {
			if mapped := pgKeyNames[b.dvsPortgroupKey]; mapped == n {
				return []VMInfo{vmInfo}
			}
		}
	}
	return nil
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
	out := make(map[string]string)

	for _, ref := range dvsRefs {
		var dvs mo.DistributedVirtualSwitch
		if err := pc.RetrieveOne(ctx, ref, []string{"name", "portgroup"}, &dvs); err != nil {
			continue // skip unreadable DVS; do not fail whole query
		}

		for _, pgRef := range dvs.Portgroup {
			var pg mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(ctx, pgRef, []string{"name"}, &pg); err == nil && pg.Name != "" {
				out[pgRef.Value] = pg.Name
			}
		}
	}

	return out, nil
}
