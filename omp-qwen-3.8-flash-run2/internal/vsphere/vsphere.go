// Package vsphere implements the inventory-retrieval logic for each feature.
// Every exported collector takes a context and a connected vSphere client and
// returns typed results, independent of Cobra wiring and tabwriter rendering.
package vsphere

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMInfo is one row of the `vms` report.
type VMInfo struct {
	Name             string
	VCPU             int32
	RAMMB            int64
	CommittedStorage int64 // bytes actually consumed (committed), not provisioned
}

// ListVMs returns all virtual machines with configured CPU, memory, and
// committed storage. Results are sorted by name.
func ListVMs(ctx context.Context, c *govmomi.Client) ([]VMInfo, error) {
	var vms []mo.VirtualMachine
	if err := inventory(ctx, c, []string{"VirtualMachine"},
		[]string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage.committed"}, &vms); err != nil {
		return nil, err
	}

	out := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.VCPU = vm.Config.Hardware.NumCPU
			info.RAMMB = int64(vm.Config.Hardware.MemoryMB)
		}
		// summary.storage.committed is the actual on-disk consumption. vCenter
		// may leave it unpopulated; degrade to 0 rather than faking provisioned size.
		if vm.Summary.Storage != nil {
			info.CommittedStorage = vm.Summary.Storage.Committed
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// DatastoreInfo is one row of the `datastores` report.
type DatastoreInfo struct {
	Name      string
	Transport string // FC | iSCSI | NVMe | NFS | unknown
	Capacity  int64
	Free      int64
}

// Transport protocol labels.
const (
	TransportFC      = "FC"
	TransportiSCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// ListDatastores returns all datastores with capacity, free space, and the
// underlying transport protocol. Results are sorted by name.
//
// NFS-family file systems report as NFS. VMFS-family volumes are classified
// from the host storage topology: a datastore's extent points at a disk
// partition whose backing device (ScsiLun canonical/durable name, NVMe
// namespace/controller, or the owning HBA type for "vmhbaN" style device
// paths) yields the real transport. When no mounted host exposes the volume,
// or its topology is not classifiable (e.g. the simulator), transport
// degrades to "unknown".
func ListDatastores(ctx context.Context, c *govmomi.Client) ([]DatastoreInfo, error) {
	var dss []mo.Datastore
	if err := inventory(ctx, c, []string{"Datastore"}, []string{"name", "summary"}, &dss); err != nil {
		return nil, err
	}

	transports := datastoreTransports(ctx, c)

	out := make([]DatastoreInfo, 0, len(dss))
	for _, ds := range dss {
		info := DatastoreInfo{
			Name:     ds.Name,
			Capacity: ds.Summary.Capacity,
			Free:     ds.Summary.FreeSpace,
		}
		if IsNFSFileSystem(ds.Summary.Type) {
			info.Transport = TransportNFS
		} else if tp, ok := transports[ds.Name]; ok {
			info.Transport = tp
		} else {
			info.Transport = TransportUnknown
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// IsNFSFileSystem reports whether a datastore volume type string denotes an
// NFS-family transport (the network file system is the transport).
func IsNFSFileSystem(volType string) bool {
	switch volType {
	case "NFS", "NFS41", "IFS", "CIFS":
		return true
	default:
		return false
	}
}

// hostStorage is one host's classification input set.
type hostStorage struct {
	adapterTransport map[string]string // "vmhba32" -> FC|iSCSI|NVMe
	lunTransport     map[string]string // LUN key or canonical name -> transport
	nvmeTransport    map[string]string // NVMe namespace key/name -> transport
}

// datastoreTransports scans every host's storage system and returns
// datastore-name → transport. Host storage is best-effort: any failure to
// read it yields an empty map (block datastores degrade to unknown) rather
// than an error, so the datastore listing itself still succeeds.
func datastoreTransports(ctx context.Context, c *govmomi.Client) map[string]string {
	result := map[string]string{}

	var hosts []mo.HostSystem
	if err := inventory(ctx, c, []string{"HostSystem"},
		[]string{"name", "config.storageDevice", "config.fileSystemVolume"}, &hosts); err != nil {
		return result
	}

	for i := range hosts {
		h := &hosts[i]
		if h.Config == nil || h.Config.StorageDevice == nil || h.Config.FileSystemVolume == nil {
			continue
		}
		hs := indexHostStorage(h.Config.StorageDevice)
		for _, mount := range h.Config.FileSystemVolume.MountInfo {
			vol := mount.Volume
			base := vol.GetHostFileSystemVolume()
			if base == nil || base.Name == "" {
				continue
			}
			if IsNFSFileSystem(base.Type) {
				continue // already NFS via summary; no block topology to scan
			}
			// Only formats carrying SCSI extents (VMFS) get block transport
			// here. VVol/VSAN/local formats have no derivable shared
			// transport and stay unknown.
			extents := volumeExtents(vol)
			if len(extents) == 0 {
				continue
			}
			tp := classifyVolume(hs, extents)
			if existing, ok := result[base.Name]; ok && existing != TransportUnknown {
				continue // a confident classification from another host wins
			}
			result[base.Name] = tp
		}
	}
	return result
}

// indexHostStorage builds lookup tables from a HostStorageDeviceInfo.
func indexHostStorage(sd *types.HostStorageDeviceInfo) hostStorage {
	hs := hostStorage{
		adapterTransport: map[string]string{},
		lunTransport:     map[string]string{},
		nvmeTransport:    map[string]string{},
	}
	for _, hba := range sd.HostBusAdapter {
		base := hba.GetHostHostBusAdapter()
		if base == nil {
			continue
		}
		hs.adapterTransport[base.Device] = ClassifyHBA(fmt.Sprintf("%T", hba))
	}
	for _, lun := range sd.ScsiLun {
		base := lun.GetScsiLun()
		if base == nil {
			continue
		}
		cls := ClassifyScsiLun(base.CanonicalName, durableNamespace(base))
		if base.Key != "" {
			hs.lunTransport[base.Key] = cls
		}
		if base.CanonicalName != "" {
			hs.lunTransport[base.CanonicalName] = cls
		}
	}
	if sd.NvmeTopology != nil {
		for _, iface := range sd.NvmeTopology.Adapter {
			for _, ctrl := range iface.ConnectedController {
				cls := ClassifyNVMeController(ctrl.TransportType)
				for _, ns := range ctrl.AttachedNamespace {
					hs.nvmeTransport[ns.Name] = cls
					if ns.Key != "" {
						hs.nvmeTransport[ns.Key] = cls
					}
				}
			}
		}
	}
	return hs
}

func durableNamespace(sl *types.ScsiLun) string {
	if sl.DurableName != nil {
		return sl.DurableName.Namespace
	}
	return ""
}

// volumeExtents extracts the SCSI partition descriptors of a mounted volume,
// when the volume format carries them (VMFS, VFFS).
func volumeExtents(vol types.BaseHostFileSystemVolume) []types.HostScsiDiskPartition {
	switch v := vol.(type) {
	case *types.HostVmfsVolume:
		return v.Extent
	case *types.HostVffsVolume:
		return v.Extent
	default:
		return nil
	}
}

// classifyVolume resolves a volume's transport from its extents. Any extent
// that cannot be classified makes the whole volume unknown; disagreeing
// extents (mixed fabric) are likewise unknown rather than guessed.
func classifyVolume(hs hostStorage, extents []types.HostScsiDiskPartition) string {
	result := ""
	for _, e := range extents {
		tp := hs.deviceTransport(e.DiskName)
		if tp == TransportUnknown {
			return TransportUnknown
		}
		if result != "" && result != tp {
			return TransportUnknown
		}
		result = tp
	}
	if result == "" {
		return TransportUnknown
	}
	return result
}

// deviceTransport classifies one extent disk name such as "naa.6000..." or
// "mpx.vmhba65:C0:T1:L2". Order of attempts: exact match in the LUN or NVMe
// tables; durable-id base name (partition suffix stripped); the "vmhbaN"
// token owning the path resolved against the host's HBA list. Local
// SAS/SATA disks ("mpx.vmhba0", "t10.") classify unknown — they are not an
// FC/iSCSI/NVMe fabric transport.
func (hs hostStorage) deviceTransport(disk string) string {
	if cls, ok := hs.lunTransport[disk]; ok && cls != "" {
		return cls
	}
	if cls, ok := hs.nvmeTransport[disk]; ok && cls != "" {
		return cls
	}
	// Durable-id names may carry a partition suffix "naa.600...:1".
	if base, found := strings.CutPrefix(disk, "naa."); found {
		if i := strings.IndexByte(base, ':'); i > 0 {
			lookup := "naa." + base[:i]
			if cls, ok := hs.lunTransport[lookup]; ok && cls != "" {
				return cls
			}
			if cls := ClassifyCanonicalName(lookup); cls != TransportUnknown {
				return cls
			}
		}
	}
	// mpx.vmhba65:C0:T1:L2 → adapter vmhba65.
	if adapter, ok := adapterToken(disk); ok {
		if cls, isHba := hs.adapterTransport[adapter]; isHba {
			// A LUN reached through a known HBA: the HBA type IS the
			// transport, except for iSCSI-aliasing local adapters.
			if cls == TransportiSCSI || cls == TransportFC || cls == TransportNVMe {
				return cls
			}
		}
	}
	return ClassifyCanonicalName(disk)
}

// adapterToken extracts "vmhbaN" from device-path names like
// "mpx.vmhba65:C0:T1:L2" or "t10.vmhba0:C0:T0:L0".
func adapterToken(name string) (string, bool) {
	i := strings.Index(name, "vmhba")
	if i < 0 {
		return "", false
	}
	rest := name[i:]
	j := strings.IndexAny(rest, ":C")
	if j <= 0 {
		// "vmhba65" bare form
		return rest, true
	}
	return rest[:j], true
}

// ClassifyHBA maps a host bus adapter's concrete type name (or device
// descriptor) to a storage transport protocol. Inputs look like
// "types.HostFibreChannelHba", "types.HostInternetScsiHba", "vmhba2 (nvme)".
func ClassifyHBA(descriptor string) string {
	d := strings.ToLower(descriptor)
	switch {
	case strings.Contains(d, "fibrechannel"):
		return TransportFC
	case strings.Contains(d, "internet"), strings.Contains(d, "iscsi"):
		return TransportiSCSI
	case strings.Contains(d, "nvme"):
		return TransportNVMe
	default:
		return TransportUnknown
	}
}

// ClassifyCanonicalName maps a SCSI/NVMe device canonical name to a
// transport. FC LUNs canonically appear as "naa."/"fc." ids; NVMe namespaces
// as "eui."/"nqn."/"nvme." ids; iSCSI targets as "iqn." names. Local
// SAS/ATA ("t10.", "mpx.vmhbaN") cannot be classified from the name alone
// and return unknown.
func ClassifyCanonicalName(canonical string) string {
	c := strings.ToLower(canonical)
	switch {
	case c == "":
		return TransportUnknown
	case strings.HasPrefix(c, "eui."), strings.HasPrefix(c, "nqn."), strings.HasPrefix(c, "nvme."):
		return TransportNVMe
	case strings.HasPrefix(c, "iqn."), strings.Contains(c, ".iqn."):
		return TransportiSCSI
	case strings.HasPrefix(c, "naa."), strings.HasPrefix(c, "fc."):
		return TransportFC
	default:
		return TransportUnknown
	}
}

// ClassifyScsiLun combines a canonical name with the durable-name namespace
// (NVMe-namespace SCSI translation layers report explicit eui/nqn markers).
func ClassifyScsiLun(canonical, durableNamespace string) string {
	switch strings.ToLower(durableNamespace) {
	case "eui", "nqm", "nqn":
		return TransportNVMe
	}
	return ClassifyCanonicalName(canonical)
}

// ClassifyNVMeController maps HostNvmeController.transportType (values of
// HostNvmeTransportType: pcie, loopback, rdma, tcp, fc, infiniband) to the
// NVMe protocol label. An empty transport type is unknown; anything else is
// NVMe regardless of the fabric it runs over.
func ClassifyNVMeController(transportType string) string {
	switch strings.ToLower(transportType) {
	case "":
		return TransportUnknown
	default:
		return TransportNVMe
	}
}

// PortGroupInfo is one row of the `vswitches` report: a port group joined to
// its switch context.
type PortGroupInfo struct {
	SwitchName string
	SwitchType string // "standard" | "distributed"
	Name       string
	VLAN       string
	Uplinks    []string
	LACP       string // "enabled" | "disabled" | "N/A"
	NumPorts   int32  // switch-level total ports
	UsedPorts  int32  // switch-level ports in use
}

// Switch/port-group classification labels.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"
	LACPEnabled       = "enabled"
	LACPDisabled      = "disabled"
	LACPNA            = "N/A"
)

// ListSwitches returns all standard vSwitch and distributed switch port
// groups, sorted by switch name then port group name.
func ListSwitches(ctx context.Context, c *govmomi.Client) ([]PortGroupInfo, error) {
	var rows []PortGroupInfo

	std, err := listStandardSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("standard switches: %w", err)
	}
	rows = append(rows, std...)

	dist, err := listDistributedSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("distributed switches: %w", err)
	}
	rows = append(rows, dist...)

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SwitchName != rows[j].SwitchName {
			return rows[i].SwitchName < rows[j].SwitchName
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

// listStandardSwitches reads host network systems. A standard vSwitch is
// host-local: the same-named switch on multiple hosts is reported once, with
// uplinks merged and port counts averaged across hosts.
func listStandardSwitches(ctx context.Context, c *govmomi.Client) ([]PortGroupInfo, error) {
	var hosts []mo.HostSystem
	if err := inventory(ctx, c, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts); err != nil {
		return nil, err
	}

	type agg struct {
		uplinks map[string]bool
		pgs     map[string]PortGroupInfo
		numSum  int64
		usedSum int64
		count   int
	}
	sw := map[string]*agg{}

	for i := range hosts {
		if hosts[i].Config == nil {
			continue
		}
		hn := hosts[i].Config.Network
		if hn == nil {
			continue
		}
		pnicDevices := map[string]bool{}
		for _, p := range hn.Pnic {
			pnicDevices[p.Device] = true
		}
		for _, vs := range hn.Vswitch {
			a := sw[vs.Name]
			if a == nil {
				a = &agg{uplinks: map[string]bool{}, pgs: map[string]PortGroupInfo{}}
				sw[vs.Name] = a
			}
			a.count++
			a.numSum += int64(vs.NumPorts)
			used := vs.NumPorts - vs.NumPortsAvailable
			if used < 0 {
				used = 0
			}
			a.usedSum += int64(used)
			for _, p := range vs.Pnic {
				// vSwitch pnic entries reference pnic keys; simulator and
				// older builds use "key-vim.host.PhysicalNic-vmnic0", live
				// hosts use the device name directly.
				name := strings.TrimPrefix(p, "key-vim.host.PhysicalNic-")
				if !pnicDevices[name] {
					name = p
				}
				a.uplinks[name] = true
			}
			for _, pg := range hn.Portgroup {
				if pg.Spec.VswitchName != vs.Name {
					continue
				}
				a.pgs[pg.Spec.Name] = PortGroupInfo{
					SwitchName: vs.Name,
					SwitchType: SwitchStandard,
					Name:       pg.Spec.Name,
					VLAN:       formatStandardVLAN(pg.Spec.VlanId),
					LACP:       LACPNA, // LACP does not exist on standard vSwitches
				}
			}
		}
	}

	var out []PortGroupInfo
	for name, a := range sw {
		uplinks := sortedSet(a.uplinks)
		n := int64(max(a.count, 1))
		num := int32(a.numSum / n)
		used := int32(a.usedSum / n)
		for _, pg := range a.pgs {
			pg.Uplinks = uplinks
			pg.NumPorts = num
			pg.UsedPorts = used
			out = append(out, pg)
		}
		if len(a.pgs) == 0 {
			out = append(out, PortGroupInfo{
				SwitchName: name, SwitchType: SwitchStandard, Name: "(none)",
				VLAN: "unknown", Uplinks: uplinks, LACP: LACPNA, NumPorts: num, UsedPorts: used,
			})
		}
	}
	return out, nil
}

// formatStandardVLAN renders the standard-vSwitch port group VLAN id. 0 means
// no VLAN; 4095 is trunk mode where the guest manages its own tags.
func formatStandardVLAN(vlanID int32) string {
	switch vlanID {
	case 0:
		return "0"
	case 4095:
		return "trunk(0-4094)"
	default:
		return strconv.FormatInt(int64(vlanID), 10)
	}
}

// listDistributedSwitches reads DVS entities and their DVPGs.
func listDistributedSwitches(ctx context.Context, c *govmomi.Client) ([]PortGroupInfo, error) {
	var dvss []mo.DistributedVirtualSwitch
	if err := inventory(ctx, c, []string{"DistributedVirtualSwitch"},
		[]string{"name", "uuid", "capability", "summary", "config", "portgroup"}, &dvss); err != nil {
		return nil, err
	}
	if len(dvss) == 0 {
		return nil, nil
	}

	var dvpgs []mo.DistributedVirtualPortgroup
	if err := inventory(ctx, c, []string{"DistributedVirtualPortgroup"},
		[]string{"name", "key", "config", "portKeys"}, &dvpgs); err != nil {
		return nil, err
	}
	bySwitch := map[string][]mo.DistributedVirtualPortgroup{}
	for _, pg := range dvpgs {
		if pg.Config.DistributedVirtualSwitch == nil {
			continue
		}
		key := pg.Config.DistributedVirtualSwitch.Value
		bySwitch[key] = append(bySwitch[key], pg)
	}

	var out []PortGroupInfo
	for i := range dvss {
		dvs := &dvss[i]
		lacp := dvsLACPState(dvs)
		uplinks := dvsUplinks(dvs)
		numPorts := dvs.Summary.NumPorts
		used := dvsUsedPorts(ctx, c, dvs)

		emitted := 0
		for _, pg := range bySwitch[dvs.Self.Value] {
			if pg.Config.Uplink != nil && *pg.Config.Uplink {
				continue // uplink port groups *are* the uplink entries
			}
			out = append(out, PortGroupInfo{
				SwitchName: dvs.Name,
				SwitchType: SwitchDistributed,
				Name:       pg.Name,
				VLAN:       formatDVLAN(pg.Config.DefaultPortConfig),
				Uplinks:    uplinks,
				LACP:       lacp,
				NumPorts:   numPorts,
				UsedPorts:  used,
			})
			emitted++
		}
		if emitted == 0 {
			out = append(out, PortGroupInfo{
				SwitchName: dvs.Name, SwitchType: SwitchDistributed, Name: "(none)",
				VLAN: "unknown", Uplinks: uplinks, LACP: lacp, NumPorts: numPorts, UsedPorts: used,
			})
		}
	}
	return out, nil
}

// dvsLACPState reports whether a distributed switch has LACP enabled. The
// switch feature capability is authoritative when present; otherwise an
// attached LACP group config is the only positive evidence. Degrades to
// "N/A" when support cannot be determined (e.g. the simulator).
func dvsLACPState(dvs *mo.DistributedVirtualSwitch) string {
	if feat := dvs.Capability.FeaturesSupported; feat != nil {
		if vc, ok := feat.(*types.VMwareDVSFeatureCapability); ok && vc.LacpCapability != nil {
			if vc.LacpCapability.LacpSupported == nil || !*vc.LacpCapability.LacpSupported {
				return LACPNA
			}
			if len(lacpGroups(dvs)) > 0 {
				return LACPEnabled
			}
			return LACPDisabled
		}
	}
	if len(lacpGroups(dvs)) > 0 {
		return LACPEnabled
	}
	return LACPNA
}

func lacpGroups(dvs *mo.DistributedVirtualSwitch) []types.VMwareDvsLacpGroupConfig {
	if cfg, ok := dvs.Config.(*types.VMwareDVSConfigInfo); ok {
		return cfg.LacpGroupConfig
	}
	return nil
}

// dvsUplinks lists the distributed switch's uplink port names, annotated with
// physical NIC bindings from host members when available.
func dvsUplinks(dvs *mo.DistributedVirtualSwitch) []string {
	var names []string
	seen := map[string]bool{}
	if dvs.Config == nil {
		return names
	}
	info := dvs.Config.GetDVSConfigInfo()
	if info != nil {
		if np, ok := info.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok {
			for _, n := range np.UplinkPortName {
				if !seen[n] {
					names = append(names, n)
					seen[n] = true
				}
			}
		}
		for _, member := range info.Host {
			backing, ok := member.Config.Backing.(*types.DistributedVirtualSwitchHostMemberPnicBacking)
			if !ok {
				continue
			}
			for _, spec := range backing.PnicSpec {
				label := spec.PnicDevice
				if spec.UplinkPortKey != "" {
					label = spec.UplinkPortKey + "(" + spec.PnicDevice + ")"
				}
				if !seen[label] {
					names = append(names, label)
					seen[label] = true
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

// dvsUsedPorts counts distributed switch ports currently connected to a
// virtual machine. Degrades to 0 when the fetch is unsupported (the
// simulator does not model port connections) without dropping the switch.
func dvsUsedPorts(ctx context.Context, c *govmomi.Client, dvs *mo.DistributedVirtualSwitch) int32 {
	sw := object.NewDistributedVirtualSwitch(c.Client, dvs.Self)
	connected := true
	ports, err := sw.FetchDVPorts(ctx, &types.DistributedVirtualSwitchPortCriteria{Connected: &connected})
	if err != nil {
		return 0
	}
	var n int32
	for _, p := range ports {
		if p.Connectee != nil && p.Connectee.ConnectedEntity != nil &&
			p.Connectee.ConnectedEntity.Type == "VirtualMachine" {
			n++
		}
	}
	return n
}

// formatDVLAN renders a DV port setting's VLAN as a single id, a trunk range
// "trunk(0-4094)", a private-VLAN id, or "unknown".
func formatDVLAN(setting types.BaseDVPortSetting) string {
	ps, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || ps == nil || ps.Vlan == nil {
		return "unknown"
	}
	switch v := ps.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.FormatInt(int64(v.VlanId), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(v.VlanId) == 0 {
			return "trunk(empty)"
		}
		parts := make([]string, 0, len(v.VlanId))
		for _, r := range v.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		return "trunk(" + strings.Join(parts, ",") + ")"
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan(%d)", v.PvlanId)
	default:
		return "unknown"
	}
}

// VMsOnPortGroup returns the virtual machines connected to the named port
// group, for standard and distributed port groups alike.
func VMsOnPortGroup(ctx context.Context, c *govmomi.Client, name string) ([]VMInfo, error) {
	refs, err := portGroupRefs(ctx, c, name)
	if err != nil {
		return nil, err
	}

	var vms []mo.VirtualMachine
	if err := inventory(ctx, c, []string{"VirtualMachine"},
		[]string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage.committed", "network"}, &vms); err != nil {
		return nil, err
	}

	out := make([]VMInfo, 0)
	for _, vm := range vms {
		connected := false
		for _, net := range vm.Network {
			if refs[net] {
				connected = true
				break
			}
		}
		if !connected {
			continue
		}
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.VCPU = vm.Config.Hardware.NumCPU
			info.RAMMB = int64(vm.Config.Hardware.MemoryMB)
		}
		if vm.Summary.Storage != nil {
			info.CommittedStorage = vm.Summary.Storage.Committed
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// portGroupRefs resolves a port group name to the set of network references
// that represent it: the standard host Network entity and/or the
// DistributedVirtualPortgroup entity.
func portGroupRefs(ctx context.Context, c *govmomi.Client, name string) (map[types.ManagedObjectReference]bool, error) {
	refs := map[types.ManagedObjectReference]bool{}

	var dvpgs []mo.DistributedVirtualPortgroup
	if err := inventory(ctx, c, []string{"DistributedVirtualPortgroup"}, []string{"name"}, &dvpgs); err != nil {
		return nil, err
	}
	for _, pg := range dvpgs {
		if pg.Name == name {
			refs[pg.Self] = true
		}
	}

	// Standard port groups surface as Network entities named after the
	// port group.
	var nets []mo.Network
	if err := inventory(ctx, c, []string{"Network"}, []string{"name"}, &nets); err != nil {
		return nil, err
	}
	for _, n := range nets {
		if n.Name == name {
			refs[n.Self] = true
		}
	}
	if len(refs) == 0 {
		seen := map[string]bool{}
		var names []string
		for _, pg := range dvpgs {
			if !seen[pg.Name] {
				seen[pg.Name] = true
				names = append(names, pg.Name)
			}
		}
		for _, n := range nets {
			if !seen[n.Name] {
				seen[n.Name] = true
				names = append(names, n.Name)
			}
		}
		sort.Strings(names)
		return nil, fmt.Errorf("port group %q not found; known port groups: %s", name, strings.Join(names, ", "))
	}
	return refs, nil
}

// inventory runs a one-shot property collect against every entity of the
// given types under the vCenter root folder.
func inventory(ctx context.Context, c *govmomi.Client, kinds, fields []string, dst any) error {
	m := view.NewManager(c.Client)
	cv, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, kinds, true)
	if err != nil {
		return fmt.Errorf("create %v view: %w", kinds, err)
	}
	defer func() { _ = cv.Destroy(ctx) }()

	refs, err := cv.Find(ctx, kinds, property.Match{})
	if err != nil {
		return fmt.Errorf("find %v: %w", kinds, err)
	}
	if len(refs) == 0 {
		return nil
	}
	if err := c.Retrieve(ctx, refs, fields, dst); err != nil {
		return fmt.Errorf("retrieve %v: %w", kinds, err)
	}
	return nil
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
