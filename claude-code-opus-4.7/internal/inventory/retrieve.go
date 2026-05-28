package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// retrieveAll collects every managed object of the given kind under the root
// folder and decodes the requested properties into dst.
func retrieveAll(ctx context.Context, c *vim25.Client, kind string, props []string, dst interface{}) error {
	mgr := view.NewManager(c)
	v, err := mgr.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{kind}, true)
	if err != nil {
		return fmt.Errorf("creating %s view: %w", kind, err)
	}
	defer func() { _ = v.Destroy(ctx) }()
	if err := v.Retrieve(ctx, []string{kind}, props, dst); err != nil {
		return fmt.Errorf("retrieving %s inventory: %w", kind, err)
	}
	return nil
}

// --- VMs ---

func vmInfoFromMO(v mo.VirtualMachine) VMInfo {
	var committed int64
	if v.Summary.Storage != nil {
		committed = v.Summary.Storage.Committed
	}
	return VMInfo{
		Name:           v.Name,
		NumCPU:         v.Summary.Config.NumCpu,
		MemoryMB:       int64(v.Summary.Config.MemorySizeMB),
		CommittedBytes: committed,
	}
}

// GetVMs returns every virtual machine with its configured vCPU/RAM and the
// storage it actually consumes (committed), sorted by name.
func GetVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	var vms []mo.VirtualMachine
	if err := retrieveAll(ctx, c, "VirtualMachine",
		[]string{"name", "summary.config.numCpu", "summary.config.memorySizeMB", "summary.storage.committed"},
		&vms); err != nil {
		return nil, err
	}
	out := make([]VMInfo, 0, len(vms))
	for _, v := range vms {
		out = append(out, vmInfoFromMO(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- Datastores ---

// GetDatastores returns every datastore with capacity figures and the derived
// backing transport, sorted by name. Transport derivation is best-effort: it
// degrades to unknown when the API does not expose the backing HBA topology
// (as the simulator does not).
func GetDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	var dss []mo.Datastore
	if err := retrieveAll(ctx, c, "Datastore",
		[]string{"name", "summary", "info", "host"}, &dss); err != nil {
		return nil, err
	}
	resolver := newTransportResolver(ctx, c)
	out := make([]DatastoreInfo, 0, len(dss))
	for _, d := range dss {
		out = append(out, DatastoreInfo{
			Name:          d.Name,
			Type:          resolver.classify(d),
			CapacityBytes: d.Summary.Capacity,
			FreeBytes:     d.Summary.FreeSpace,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// transportResolver derives a datastore's transport from the storage device
// topology of the hosts that mount it.
type transportResolver struct {
	byHost map[types.ManagedObjectReference]*types.HostStorageDeviceInfo
}

func newTransportResolver(ctx context.Context, c *vim25.Client) *transportResolver {
	r := &transportResolver{byHost: map[types.ManagedObjectReference]*types.HostStorageDeviceInfo{}}
	var hosts []mo.HostSystem
	if err := retrieveAll(ctx, c, "HostSystem", []string{"config.storageDevice"}, &hosts); err != nil {
		// Transport is best-effort; without host storage data it stays unknown.
		return r
	}
	for _, h := range hosts {
		if h.Config != nil && h.Config.StorageDevice != nil {
			r.byHost[h.Reference()] = h.Config.StorageDevice
		}
	}
	return r
}

func (r *transportResolver) classify(d mo.Datastore) string {
	if isNFSDatastore(d) {
		return TransportNFS
	}
	vmfs, ok := d.Info.(*types.VmfsDatastoreInfo)
	if !ok || vmfs.Vmfs == nil {
		return TransportUnknown
	}
	for _, mount := range d.Host {
		sdi := r.byHost[mount.Key]
		if sdi == nil {
			continue
		}
		for _, ext := range vmfs.Vmfs.Extent {
			desc, ok := descriptorForExtent(sdi, ext.DiskName)
			if !ok {
				continue
			}
			if t := ClassifyTransport(desc); t != TransportUnknown {
				return t
			}
		}
	}
	return TransportUnknown
}

func isNFSDatastore(d mo.Datastore) bool {
	switch strings.ToUpper(d.Summary.Type) {
	case "NFS", "NFS41":
		return true
	}
	if nas, ok := d.Info.(*types.NasDatastoreInfo); ok && nas.Nas != nil {
		switch strings.ToUpper(nas.Nas.Type) {
		case "NFS", "NFS41":
			return true
		}
	}
	return false
}

// descriptorForExtent walks LUN -> multipath path -> HBA to describe the
// adapter backing a VMFS extent identified by its canonical disk name.
func descriptorForExtent(sdi *types.HostStorageDeviceInfo, canonical string) (StorageAdapterDescriptor, bool) {
	lunKey := ""
	for _, l := range sdi.ScsiLun {
		sl := l.GetScsiLun()
		if sl.CanonicalName == canonical {
			lunKey = sl.Key
			break
		}
	}
	if lunKey == "" {
		return StorageAdapterDescriptor{}, false
	}
	adapterKey := ""
	if sdi.MultipathInfo != nil {
		for _, lu := range sdi.MultipathInfo.Lun {
			if lu.Lun != lunKey {
				continue
			}
			for _, p := range lu.Path {
				if p.Adapter != "" {
					adapterKey = p.Adapter
					break
				}
			}
			if adapterKey != "" {
				break
			}
		}
	}
	if adapterKey == "" {
		return StorageAdapterDescriptor{}, false
	}
	for _, hba := range sdi.HostBusAdapter {
		b := hba.GetHostHostBusAdapter()
		if b.Key == adapterKey {
			return StorageAdapterDescriptor{
				AdapterType: shortTypeName(hba),
				Driver:      b.Driver,
				Model:       b.Model,
			}, true
		}
	}
	return StorageAdapterDescriptor{}, false
}

func shortTypeName(v interface{}) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", v), "*types.")
}

// --- Switches ---

// GetSwitches returns every virtual switch — standard host vSwitches and
// distributed switches — with their port groups, sorted by switch name.
func GetSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	standard, err := standardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	distributed, err := distributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	out := append(standard, distributed...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// standardSwitches collects host vSwitches. The same standard switch (e.g.
// vSwitch0) exists independently on every host; we merge them by name into a
// single cluster-wide view, unioning uplinks and port groups, so the listing
// is not flooded with per-host duplicates.
func standardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var hosts []mo.HostSystem
	if err := retrieveAll(ctx, c, "HostSystem", []string{"name", "config.network"}, &hosts); err != nil {
		return nil, err
	}
	byName := map[string]*SwitchInfo{}
	var order []string
	for _, h := range hosts {
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		net := h.Config.Network
		pnicDevice := map[string]string{}
		for _, p := range net.Pnic {
			pnicDevice[p.Key] = p.Device
		}
		for _, sw := range net.Vswitch {
			si, ok := byName[sw.Name]
			if !ok {
				si = &SwitchInfo{Name: sw.Name, Type: SwitchStandard, LACP: LACPNA}
				byName[sw.Name] = si
				order = append(order, sw.Name)
			}
			for _, key := range sw.Pnic {
				dev := pnicDevice[key]
				if dev == "" {
					dev = key
				}
				si.Uplinks = appendUnique(si.Uplinks, dev)
			}
			total := sw.NumPorts
			used := sw.NumPorts - sw.NumPortsAvailable
			for _, pg := range net.Portgroup {
				if pg.Spec.VswitchName != sw.Name {
					continue
				}
				si.PortGroups = upsertPortGroup(si.PortGroups, PortGroupInfo{
					Name:       pg.Spec.Name,
					VLAN:       formatStandardVLAN(pg.Spec.VlanId),
					TotalPorts: total,
					UsedPorts:  used,
				})
			}
		}
	}
	out := make([]SwitchInfo, 0, len(order))
	for _, n := range order {
		si := byName[n]
		sort.Slice(si.PortGroups, func(i, j int) bool { return si.PortGroups[i].Name < si.PortGroups[j].Name })
		out = append(out, *si)
	}
	return out, nil
}

func distributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var dvss []mo.DistributedVirtualSwitch
	if err := retrieveAll(ctx, c, "DistributedVirtualSwitch",
		[]string{"name", "config", "portgroup"}, &dvss); err != nil {
		return nil, err
	}
	pc := property.DefaultCollector(c)
	out := make([]SwitchInfo, 0, len(dvss))
	for _, d := range dvss {
		si := SwitchInfo{
			Name:    d.Name,
			Type:    SwitchDistributed,
			Uplinks: dvsUplinks(d.Config),
			LACP:    dvsLACP(d.Config),
		}
		total, used := dvPortCounts(ctx, c, d)
		if len(d.Portgroup) > 0 {
			var dvpgs []mo.DistributedVirtualPortgroup
			if err := pc.Retrieve(ctx, d.Portgroup, []string{"name", "config", "key"}, &dvpgs); err != nil {
				return nil, fmt.Errorf("retrieving port groups for switch %q: %w", d.Name, err)
			}
			for _, pg := range dvpgs {
				si.PortGroups = append(si.PortGroups, PortGroupInfo{
					Name:       pg.Name,
					VLAN:       formatDVPGVLAN(pg.Config),
					TotalPorts: total[pg.Key],
					UsedPorts:  used[pg.Key],
				})
			}
			sort.Slice(si.PortGroups, func(i, j int) bool { return si.PortGroups[i].Name < si.PortGroups[j].Name })
		}
		out = append(out, si)
	}
	return out, nil
}

func dvsUplinks(cfg types.BaseDVSConfigInfo) []string {
	base := cfg.GetDVSConfigInfo()
	if base == nil {
		return nil
	}
	policy, ok := base.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy)
	if !ok {
		return nil
	}
	return policy.UplinkPortName
}

func dvsLACP(cfg types.BaseDVSConfigInfo) string {
	vcfg, ok := cfg.(*types.VMwareDVSConfigInfo)
	if !ok {
		return LACPDisabled
	}
	for _, g := range vcfg.LacpGroupConfig {
		if g.Mode != "" || g.UplinkNum > 0 {
			return LACPEnabled
		}
	}
	return LACPDisabled
}

// dvPortCounts returns per-port-group total and in-use port counts for a
// distributed switch by fetching its ports. It degrades to empty maps (zero
// counts) when the API does not support the query.
func dvPortCounts(ctx context.Context, c *vim25.Client, d mo.DistributedVirtualSwitch) (total, used map[string]int32) {
	total = map[string]int32{}
	used = map[string]int32{}
	odvs := object.NewDistributedVirtualSwitch(c, d.Reference())
	ports, err := odvs.FetchDVPorts(ctx, &types.DistributedVirtualSwitchPortCriteria{})
	if err != nil {
		return total, used
	}
	for _, p := range ports {
		total[p.PortgroupKey]++
		if p.Connectee != nil {
			used[p.PortgroupKey]++
		}
	}
	return total, used
}

// --- Port group -> VMs ---

// GetPortgroupVMs returns the virtual machines connected to the named port
// group, which may be standard or distributed. It resolves the port group to a
// network object and scans VMs by their network attachments, which works
// regardless of whether the backref Network.vm is populated by the server.
func GetPortgroupVMs(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	var nets []mo.Network
	if err := retrieveAll(ctx, c, "Network", []string{"name"}, &nets); err != nil {
		return nil, err
	}
	var target *types.ManagedObjectReference
	for _, n := range nets {
		if n.Name == name {
			ref := n.Reference()
			target = &ref
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("port group %q not found", name)
	}

	var vms []mo.VirtualMachine
	if err := retrieveAll(ctx, c, "VirtualMachine",
		[]string{"name", "summary.config.numCpu", "summary.config.memorySizeMB", "summary.storage.committed", "network"},
		&vms); err != nil {
		return nil, err
	}
	out := make([]VMInfo, 0)
	for _, v := range vms {
		for _, ref := range v.Network {
			if ref == *target {
				out = append(out, vmInfoFromMO(v))
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- VLAN + slice helpers ---

func formatStandardVLAN(id int32) string {
	switch {
	case id == 0:
		return "none"
	case id == 4095:
		return "trunk 0-4094"
	default:
		return strconv.Itoa(int(id))
	}
}

func formatDVPGVLAN(cfg types.DVPortgroupConfigInfo) string {
	setting, ok := cfg.DefaultPortConfig.(*types.VMwareDVSPortSetting)
	if !ok || setting.Vlan == nil {
		return "none"
	}
	switch v := setting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		if v.VlanId == 0 {
			return "none"
		}
		return strconv.Itoa(int(v.VlanId))
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		return "trunk " + formatVLANRanges(v.VlanId)
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return "pvlan " + strconv.Itoa(int(v.PvlanId))
	default:
		return "unknown"
	}
}

func formatVLANRanges(ranges []types.NumericRange) string {
	if len(ranges) == 0 {
		return "0-4094"
	}
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Start == r.End {
			parts = append(parts, strconv.Itoa(int(r.Start)))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
	}
	return strings.Join(parts, ",")
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func upsertPortGroup(pgs []PortGroupInfo, pg PortGroupInfo) []PortGroupInfo {
	for _, x := range pgs {
		if x.Name == pg.Name {
			return pgs
		}
	}
	return append(pgs, pg)
}
