package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type Inventory struct {
	client *vim25.Client
}

func NewInventory(client *vim25.Client) *Inventory {
	return &Inventory{client: client}
}

func (i *Inventory) ListVMs(ctx context.Context) ([]VMInfo, error) {
	var vms []mo.VirtualMachine
	if err := i.retrieve(ctx, []string{"VirtualMachine"}, []string{
		"name",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"summary.storage.committed",
	}, &vms); err != nil {
		return nil, fmt.Errorf("retrieve virtual machines: %w", err)
	}

	out := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		info := VMInfo{Name: vm.Name, StorageBytes: vm.Summary.Storage.Committed}
		if vm.Config != nil {
			info.VCPU = vm.Config.Hardware.NumCPU
			info.RAMBytes = int64(vm.Config.Hardware.MemoryMB) * 1024 * 1024
		}
		out = append(out, info)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

func (i *Inventory) ListDatastores(ctx context.Context) ([]DatastoreInfo, error) {
	var datastores []mo.Datastore
	if err := i.retrieve(ctx, []string{"Datastore"}, []string{"name", "summary", "info"}, &datastores); err != nil {
		return nil, fmt.Errorf("retrieve datastores: %w", err)
	}

	var storage []types.HostStorageDeviceInfo
	if containsVMFS(datastores) {
		var err error
		storage, err = i.hostStorageDevices(ctx)
		if err != nil {
			return nil, err
		}
	}

	out := make([]DatastoreInfo, 0, len(datastores))
	for _, ds := range datastores {
		capacity := ds.Summary.Capacity
		available := ds.Summary.FreeSpace
		out = append(out, DatastoreInfo{
			Name:           ds.Name,
			Type:           datastoreTransport(ds, storage),
			UsedBytes:      UsedBytes(capacity, available),
			AvailableBytes: available,
			CapacityBytes:  capacity,
		})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

func (i *Inventory) ListSwitches(ctx context.Context) ([]SwitchInfo, error) {
	var rows []SwitchInfo

	standard, err := i.standardSwitches(ctx)
	if err != nil {
		return nil, err
	}
	rows = append(rows, standard...)

	distributed, err := i.distributedSwitches(ctx)
	if err != nil {
		return nil, err
	}
	rows = append(rows, distributed...)

	sort.Slice(rows, func(a, b int) bool {
		if rows[a].Switch != rows[b].Switch {
			return rows[a].Switch < rows[b].Switch
		}
		return rows[a].PortGroup < rows[b].PortGroup
	})
	return rows, nil
}

func (i *Inventory) ListVMsByPortGroup(ctx context.Context, name string) ([]VMInfo, error) {
	netRefs, dpgKeys, found, err := i.networkRefsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("portgroup %q was not found", name)
	}

	var vms []mo.VirtualMachine
	if err := i.retrieve(ctx, []string{"VirtualMachine"}, []string{
		"name",
		"config",
		"summary.storage.committed",
		"network",
	}, &vms); err != nil {
		return nil, fmt.Errorf("retrieve virtual machines for portgroup %q: %w", name, err)
	}

	out := make([]VMInfo, 0)
	for _, vm := range vms {
		if !vmOnAnyNetwork(vm, name, netRefs, dpgKeys) {
			continue
		}
		info := VMInfo{Name: vm.Name, StorageBytes: vm.Summary.Storage.Committed}
		if vm.Config != nil {
			info.VCPU = vm.Config.Hardware.NumCPU
			info.RAMBytes = int64(vm.Config.Hardware.MemoryMB) * 1024 * 1024
		}
		out = append(out, info)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

func (i *Inventory) retrieve(ctx context.Context, kinds []string, props []string, dst any) error {
	m := view.NewManager(i.client)
	cv, err := m.CreateContainerView(ctx, i.client.ServiceContent.RootFolder, kinds, true)
	if err != nil {
		return fmt.Errorf("create container view for %s: %w", strings.Join(kinds, ","), err)
	}
	defer func() { _ = cv.Destroy(ctx) }()

	if err := cv.Retrieve(ctx, kinds, props, dst); err != nil {
		return fmt.Errorf("retrieve properties %v: %w", props, err)
	}
	return nil
}

func (i *Inventory) hostStorageDevices(ctx context.Context) ([]types.HostStorageDeviceInfo, error) {
	var hosts []mo.HostSystem
	if err := i.retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.storageDevice"}, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve host storage devices: %w", err)
	}

	storage := make([]types.HostStorageDeviceInfo, 0, len(hosts))
	for _, host := range hosts {
		if host.Config == nil || host.Config.StorageDevice == nil {
			continue
		}
		storage = append(storage, *host.Config.StorageDevice)
	}
	return storage, nil
}

func (i *Inventory) standardSwitches(ctx context.Context) ([]SwitchInfo, error) {
	var hosts []mo.HostSystem
	if err := i.retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve host networking: %w", err)
	}

	var rows []SwitchInfo
	seen := map[string]bool{}
	for _, host := range hosts {
		if host.Config == nil || host.Config.Network == nil {
			continue
		}
		pnicNames := map[string]string{}
		for _, pnic := range host.Config.Network.Pnic {
			pnicNames[pnic.Key] = pnic.Device
		}
		for _, sw := range host.Config.Network.Vswitch {
			for _, pg := range host.Config.Network.Portgroup {
				if pg.Spec.VswitchName != sw.Name && pg.Vswitch != sw.Key {
					continue
				}
				key := sw.Name + "\x00" + pg.Spec.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				total := sw.NumPorts
				if total == 0 {
					total = sw.Spec.NumPorts
				}
				used := total - sw.NumPortsAvailable
				if used < 0 {
					used = int32(len(pg.Port))
				}
				rows = append(rows, SwitchInfo{
					Switch:     sw.Name,
					SwitchType: "standard",
					PortGroup:  pg.Spec.Name,
					VLAN:       standardVLAN(pg.Spec.VlanId),
					Uplinks:    uplinkNames(sw, pnicNames),
					LACP:       "N/A",
					TotalPorts: total,
					UsedPorts:  used,
				})
			}
		}
	}
	return rows, nil
}

func (i *Inventory) distributedSwitches(ctx context.Context) ([]SwitchInfo, error) {
	var switches []mo.DistributedVirtualSwitch
	if err := i.retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "summary", "config", "portgroup"}, &switches); err != nil {
		return nil, fmt.Errorf("retrieve distributed switches: %w", err)
	}
	if len(switches) == 0 {
		return nil, nil
	}

	switchByRef := map[string]mo.DistributedVirtualSwitch{}
	for _, sw := range switches {
		switchByRef[sw.Reference().String()] = sw
	}

	var portgroups []mo.DistributedVirtualPortgroup
	if err := i.retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "config", "portKeys"}, &portgroups); err != nil {
		return nil, fmt.Errorf("retrieve distributed portgroups: %w", err)
	}

	var rows []SwitchInfo
	for _, pg := range portgroups {
		if pg.Config.DistributedVirtualSwitch == nil {
			continue
		}
		sw, ok := switchByRef[pg.Config.DistributedVirtualSwitch.String()]
		if !ok {
			continue
		}
		rows = append(rows, SwitchInfo{
			Switch:     sw.Name,
			SwitchType: "distributed",
			PortGroup:  pg.Name,
			VLAN:       distributedVLAN(pg.Config.DefaultPortConfig),
			Uplinks:    distributedUplinks(sw.Config),
			LACP:       distributedLACP(sw.Config),
			TotalPorts: pg.Config.NumPorts,
			UsedPorts:  boundedUsed(int32(len(pg.PortKeys)), pg.Config.NumPorts),
		})
	}
	return rows, nil
}

func (i *Inventory) networkRefsByName(ctx context.Context, name string) (map[string]bool, map[string]bool, bool, error) {
	refs := map[string]bool{}
	dpgKeys := map[string]bool{}
	found := false

	var networks []mo.Network
	if err := i.retrieve(ctx, []string{"Network"}, []string{"name"}, &networks); err != nil {
		return nil, nil, false, fmt.Errorf("retrieve networks: %w", err)
	}
	for _, net := range networks {
		if net.Name == name {
			found = true
			refs[net.Reference().String()] = true
		}
	}

	var dpgs []mo.DistributedVirtualPortgroup
	if err := i.retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "key"}, &dpgs); err != nil {
		return nil, nil, false, fmt.Errorf("retrieve distributed portgroups: %w", err)
	}
	for _, pg := range dpgs {
		if pg.Name == name {
			found = true
			refs[pg.Reference().String()] = true
			if pg.Key != "" {
				dpgKeys[pg.Key] = true
			}
		}
	}

	var hosts []mo.HostSystem
	if err := i.retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts); err != nil {
		return nil, nil, false, fmt.Errorf("retrieve host portgroups: %w", err)
	}
	for _, host := range hosts {
		if host.Config == nil || host.Config.Network == nil {
			continue
		}
		for _, pg := range host.Config.Network.Portgroup {
			if pg.Spec.Name == name {
				found = true
			}
		}
	}

	return refs, dpgKeys, found, nil
}

func vmOnAnyNetwork(vm mo.VirtualMachine, name string, refs map[string]bool, dpgKeys map[string]bool) bool {
	for _, ref := range vm.Network {
		if refs[ref.String()] {
			return true
		}
	}
	if vm.Config == nil {
		return false
	}
	for _, device := range vm.Config.Hardware.Device {
		card, ok := ethernetCard(device)
		if !ok || card.Backing == nil {
			continue
		}
		switch backing := card.Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if backing.DeviceName == name {
				return true
			}
			if backing.Network != nil && refs[backing.Network.String()] {
				return true
			}
		case *types.VirtualEthernetCardLegacyNetworkBackingInfo:
			if backing.DeviceName == name {
				return true
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			if backing.Port.PortgroupKey != "" && dpgKeys[backing.Port.PortgroupKey] {
				return true
			}
		}
	}
	return false
}

func ethernetCard(device types.BaseVirtualDevice) (*types.VirtualEthernetCard, bool) {
	card, ok := device.(types.BaseVirtualEthernetCard)
	if !ok {
		return nil, false
	}
	return card.GetVirtualEthernetCard(), true
}

func containsVMFS(datastores []mo.Datastore) bool {
	for _, ds := range datastores {
		if strings.EqualFold(ds.Summary.Type, "VMFS") {
			return true
		}
		if _, ok := ds.Info.(*types.VmfsDatastoreInfo); ok {
			return true
		}
	}
	return false
}

func datastoreTransport(ds mo.Datastore, storage []types.HostStorageDeviceInfo) StorageTransport {
	if strings.EqualFold(ds.Summary.Type, "NFS") || strings.EqualFold(ds.Summary.Type, "NFS41") {
		return TransportNFS
	}
	info, ok := ds.Info.(*types.VmfsDatastoreInfo)
	if !ok {
		return TransportUnknown
	}
	return vmfsTransport(info, storage)
}

func vmfsTransport(info *types.VmfsDatastoreInfo, storage []types.HostStorageDeviceInfo) StorageTransport {
	if info == nil || info.Vmfs == nil {
		return TransportUnknown
	}
	for _, extent := range info.Vmfs.Extent {
		if extent.DiskName == "" {
			continue
		}
		for _, hostStorage := range storage {
			if transport := vmfsExtentTransport(extent.DiskName, hostStorage); transport != TransportUnknown {
				return transport
			}
		}
	}
	return TransportUnknown
}

func vmfsExtentTransport(diskName string, storage types.HostStorageDeviceInfo) StorageTransport {
	lunKey := lunKeyByCanonicalName(diskName, storage.ScsiLun)
	if lunKey == "" {
		return TransportUnknown
	}
	adapterKey, targetDesc := adapterKeyByLUNKey(lunKey, storage.ScsiTopology)
	if adapterKey != "" {
		for _, hba := range storage.HostBusAdapter {
			if hba == nil {
				continue
			}
			base := hba.GetHostHostBusAdapter()
			if base != nil && base.Key == adapterKey {
				return ClassifyTransportDescriptor(hbaDescriptor(hba))
			}
		}
	}
	if targetDesc != "" {
		return ClassifyTransportDescriptor(targetDesc)
	}
	return TransportUnknown
}

func lunKeyByCanonicalName(diskName string, luns []types.BaseScsiLun) string {
	for _, baseLUN := range luns {
		if baseLUN == nil {
			continue
		}
		lun := baseLUN.GetScsiLun()
		if lun != nil && strings.EqualFold(lun.CanonicalName, diskName) {
			return lun.Key
		}
	}
	return ""
}

func adapterKeyByLUNKey(lunKey string, topology *types.HostScsiTopology) (string, string) {
	if topology == nil {
		return "", ""
	}
	for _, adapter := range topology.Adapter {
		for _, target := range adapter.Target {
			for _, lun := range target.Lun {
				if lun.ScsiLun == lunKey {
					return adapter.Adapter, fmt.Sprintf("%T", target.Transport)
				}
			}
		}
	}
	return "", ""
}

func hbaDescriptor(hba types.BaseHostHostBusAdapter) string {
	if hba == nil {
		return ""
	}
	base := hba.GetHostHostBusAdapter()
	if base == nil {
		return fmt.Sprintf("%T", hba)
	}
	return strings.Join([]string{
		fmt.Sprintf("%T", hba),
		base.Key,
		base.Device,
		base.Model,
		base.Driver,
		base.Pci,
		base.StorageProtocol,
	}, " ")
}

func ClassifyTransportDescriptor(desc string) StorageTransport {
	d := strings.ToLower(desc)
	switch {
	case strings.Contains(d, "nfs"):
		return TransportNFS
	case strings.Contains(d, "nvme"):
		return TransportNVMe
	case strings.Contains(d, "iscsi") || strings.Contains(d, "internetscsi") || strings.Contains(d, "iqn."):
		return TransportISCSI
	case strings.Contains(d, "fibre") || strings.Contains(d, "fiber") || strings.Contains(d, "fibrechannel") || strings.Contains(d, "fc."):
		return TransportFC
	default:
		return TransportUnknown
	}
}

func standardVLAN(id int32) string {
	if id == 4095 {
		return "trunk 0-4095"
	}
	return strconv.FormatInt(int64(id), 10)
}

func distributedVLAN(setting types.BaseDVPortSetting) string {
	vmw, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || vmw.Vlan == nil {
		return "N/A"
	}
	switch vlan := vmw.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.FormatInt(int64(vlan.VlanId), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		parts := make([]string, 0, len(vlan.VlanId))
		for _, r := range vlan.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		if len(parts) == 0 {
			return "trunk"
		}
		return "trunk " + strings.Join(parts, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("private %d", vlan.PvlanId)
	default:
		return "N/A"
	}
}

func uplinkNames(sw types.HostVirtualSwitch, pnics map[string]string) string {
	keys := append([]string{}, sw.Pnic...)
	if len(keys) == 0 && sw.Spec.Bridge != nil {
		switch bridge := sw.Spec.Bridge.(type) {
		case *types.HostVirtualSwitchBondBridge:
			keys = append(keys, bridge.NicDevice...)
		case *types.HostVirtualSwitchSimpleBridge:
			keys = append(keys, bridge.NicDevice)
		}
	}
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		if name := pnics[key]; name != "" {
			names = append(names, name)
		} else {
			names = append(names, key)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "N/A"
	}
	return strings.Join(names, ",")
}

func distributedUplinks(config types.BaseDVSConfigInfo) string {
	if config == nil {
		return "N/A"
	}
	base := config.GetDVSConfigInfo()
	if base == nil || base.UplinkPortPolicy == nil {
		return "N/A"
	}
	if names, ok := base.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok && len(names.UplinkPortName) > 0 {
		out := append([]string{}, names.UplinkPortName...)
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	return "N/A"
}

func distributedLACP(config types.BaseDVSConfigInfo) string {
	vmw, ok := config.(*types.VMwareDVSConfigInfo)
	if !ok {
		return "disabled"
	}
	if len(vmw.LacpGroupConfig) > 0 || vmw.LacpApiVersion != "" {
		return "enabled"
	}
	return "disabled"
}

func boundedUsed(used, total int32) int32 {
	if total > 0 && used > total {
		return total
	}
	return used
}
