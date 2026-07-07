package inventory

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
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

// GetVMs retrieves all virtual machines with their inventory info using
// a single ContainerView + PropertyCollector retrieve (no per-object N+1).
func GetVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create VM container view: %w", err)
	}
	defer v.Destroy(ctx)

	refs, err := v.Find(ctx, []string{"VirtualMachine"}, nil)
	if err != nil {
		return nil, fmt.Errorf("find VMs: %w", err)
	}

	pc := property.DefaultCollector(c)
	var vmProps []mo.VirtualMachine
	if err := pc.Retrieve(ctx, refs, []string{
		"name", "config.hardware.numCPU", "config.hardware.memoryMB",
		"summary.storage.committed",
	}, &vmProps); err != nil {
		return nil, fmt.Errorf("retrieve VM properties: %w", err)
	}

	results := make([]VMInfo, 0, len(vmProps))
	for _, vm := range vmProps {
		results = append(results, VMInfo{
			Name:    vm.Name,
			VCPU:    vm.Config.Hardware.NumCPU,
			RAMMB:   vm.Config.Hardware.MemoryMB,
			Storage: vm.Summary.Storage.Committed,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// GetDatastores retrieves all datastores with capacity and transport info
// using a single ContainerView + PropertyCollector retrieve.
func GetDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("create datastore container view: %w", err)
	}
	defer v.Destroy(ctx)

	refs, err := v.Find(ctx, []string{"Datastore"}, nil)
	if err != nil {
		return nil, fmt.Errorf("find datastores: %w", err)
	}

	pc := property.DefaultCollector(c)
	var dsProps []mo.Datastore
	if err := pc.Retrieve(ctx, refs, []string{
		"name", "summary.capacity", "summary.freeSpace",
		"summary.type", "info", "host",
	}, &dsProps); err != nil {
		return nil, fmt.Errorf("retrieve datastore properties: %w", err)
	}

	results := make([]DatastoreInfo, 0, len(dsProps))
	for _, ds := range dsProps {
		info := DatastoreInfo{
			Name:      ds.Name,
			Available: ds.Summary.FreeSpace,
		}
		if ds.Summary.Capacity > 0 {
			info.Used = formatter.UsedCapacity(ds.Summary.Capacity, ds.Summary.FreeSpace)
		}

		isNFS := ds.Summary.Type == "NFS" || ds.Summary.Type == "NFS41"
		if isNFS {
			info.Type = "NFS"
		} else {
			hbaTypes, deviceNames := extractBackingInfo(ds)
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
				hbaTypes = append(hbaTypes, inferHBAType(extent.DiskName))
			}
		}
	case *types.NasDatastoreInfo:
		// NFS — handled at caller level
	}
	return hbaTypes, deviceNames
}

// inferHBAType infers the host bus adapter type from a device name pattern.
func inferHBAType(deviceName string) string {
	lower := strings.ToLower(deviceName)
	if strings.Contains(lower, "nvme") {
		return "nvme"
	}
	if strings.Contains(lower, "iscsi") {
		return "iscsi"
	}
	if strings.HasPrefix(lower, "naa.") || strings.HasPrefix(lower, "t10.") {
		return "fc"
	}
	return ""
}

// GetVSwitches retrieves all virtual switches (standard and distributed).
func GetVSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	results, err := getStandardVSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("get standard vswitches: %w", err)
	}

	dvResults, err := getDistributedVSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("get distributed vswitches: %w", err)
	}
	results = append(results, dvResults...)

	return results, nil
}

// getStandardVSwitches retrieves standard vswitches using ContainerView +
// PropertyCollector (single batched retrieve, no per-host Properties loop).
func getStandardVSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create host container view: %w", err)
	}
	defer v.Destroy(ctx)

	refs, err := v.Find(ctx, []string{"HostSystem"}, nil)
	if err != nil {
		return nil, fmt.Errorf("find hosts: %w", err)
	}

	pc := property.DefaultCollector(c)
	var hostProps []mo.HostSystem
	if err := pc.Retrieve(ctx, refs, []string{
		"name", "config.network.vswitch", "config.network.portgroup",
	}, &hostProps); err != nil {
		return nil, fmt.Errorf("retrieve host network properties: %w", err)
	}

	seen := make(map[string]bool)
	var results []SwitchInfo

	for _, host := range hostProps {
		for _, vs := range host.Config.Network.Vswitch {
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

			for _, pg := range host.Config.Network.Portgroup {
				if pg.Spec.VswitchName == name {
					si.PortGroups = append(si.PortGroups, PortGroupInfo{
						Name: pg.Spec.Name,
						VLAN: formatVLAN(pg.Spec.VlanId),
					})
				}
			}

			results = append(results, si)
		}
	}

	return results, nil
}

// getDistributedVSwitches retrieves distributed vswitches using ContainerView +
// PropertyCollector. Port group names are resolved in a single batched retrieve.
func getDistributedVSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, fmt.Errorf("create DVS container view: %w", err)
	}
	defer v.Destroy(ctx)

	refs, err := v.Find(ctx, []string{"DistributedVirtualSwitch"}, nil)
	if err != nil {
		return nil, fmt.Errorf("find distributed vswitches: %w", err)
	}

	if len(refs) == 0 {
		return nil, nil
	}

	pc := property.DefaultCollector(c)
	var dvsProps []mo.DistributedVirtualSwitch
	if err := pc.Retrieve(ctx, refs, []string{"name", "config", "portgroup"}, &dvsProps); err != nil {
		return nil, fmt.Errorf("retrieve DVS properties: %w", err)
	}

	// Collect all port group refs for a single batched retrieve
	var allPGRefs []types.ManagedObjectReference
	type dvsEntry struct {
		dvs    mo.DistributedVirtualSwitch
		pgRefs []types.ManagedObjectReference
	}
	seen := make(map[string]bool)
	var dvsList []dvsEntry

	for _, dvs := range dvsProps {
		if seen[dvs.Name] {
			continue
		}
		seen[dvs.Name] = true

		entry := dvsEntry{dvs: dvs}
		for _, pgRef := range dvs.Portgroup {
			entry.pgRefs = append(entry.pgRefs, pgRef)
			allPGRefs = append(allPGRefs, pgRef)
		}
		dvsList = append(dvsList, entry)
	}

	// Single batched retrieve for all port group names
	pgNames := make(map[types.ManagedObjectReference]string)
	if len(allPGRefs) > 0 {
		var pgProps []mo.DistributedVirtualPortgroup
		if err := pc.Retrieve(ctx, allPGRefs, []string{"name"}, &pgProps); err != nil {
			return nil, fmt.Errorf("retrieve port group properties: %w", err)
		}
		for _, pg := range pgProps {
			pgNames[pg.Reference()] = pg.Name
		}
	}

	var results []SwitchInfo
	for _, entry := range dvsList {
		dvs := entry.dvs
		si := SwitchInfo{
			SwitchName: dvs.Name,
			SwitchType: "distributed",
		}

		if vmwareCfg, ok := dvs.Config.(*types.VMwareDVSConfigInfo); ok {
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

		for _, pgRef := range entry.pgRefs {
			if name, ok := pgNames[pgRef]; ok {
				si.PortGroups = append(si.PortGroups, PortGroupInfo{Name: name})
			}
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
// Uses a single batched retrieve to build a VM->networks map, resolves the
// target PG's moref once, then matches — no O(VMs x NICs) inner loop.
func GetVMsByPortGroup(ctx context.Context, c *vim25.Client, portGroupName string) ([]VMInfo, error) {
	pc := property.DefaultCollector(c)
	m := view.NewManager(c)

	// Resolve target port group MOR
	netView, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Network"}, true)
	if err != nil {
		return nil, fmt.Errorf("create network container view: %w", err)
	}
	defer netView.Destroy(ctx)

	netRefs, err := netView.Find(ctx, []string{"Network"}, nil)
	if err != nil {
		return nil, fmt.Errorf("find networks: %w", err)
	}

	var netProps []mo.Network
	if err := pc.Retrieve(ctx, netRefs, []string{"name"}, &netProps); err != nil {
		return nil, fmt.Errorf("retrieve network names: %w", err)
	}

	// Collect all MORs matching the port group name (multiple hosts may
	// each have a port group with the same name, e.g. "VM Network").
	targetRefs := make(map[types.ManagedObjectReference]bool)
	for _, n := range netProps {
		if n.Name == portGroupName {
			targetRefs[n.Reference()] = true
		}
	}
	if len(targetRefs) == 0 {
		return nil, nil
	}

	// Single batched retrieve: all VMs with their network refs
	vmView, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create VM container view: %w", err)
	}
	defer vmView.Destroy(ctx)

	vmRefs, err := vmView.Find(ctx, []string{"VirtualMachine"}, nil)
	if err != nil {
		return nil, fmt.Errorf("find VMs: %w", err)
	}

	var vms []mo.VirtualMachine
	if err := pc.Retrieve(ctx, vmRefs, []string{"name", "network"}, &vms); err != nil {
		return nil, fmt.Errorf("retrieve VM network refs: %w", err)
	}

	// Match VMs to target port group — single scan over in-memory data
	var matchingRefs []types.ManagedObjectReference
	for _, vm := range vms {
		for _, net := range vm.Network {
			if targetRefs[net] {
				matchingRefs = append(matchingRefs, vm.Reference())
				break
			}
		}
	}

	if len(matchingRefs) == 0 {
		return nil, nil
	}

	// Batch retrieve full VM info for matches only
	var vmProps []mo.VirtualMachine
	if err := pc.Retrieve(ctx, matchingRefs, []string{
		"name", "config.hardware.numCPU", "config.hardware.memoryMB",
		"summary.storage.committed",
	}, &vmProps); err != nil {
		return nil, fmt.Errorf("retrieve VM properties: %w", err)
	}

	results := make([]VMInfo, 0, len(vmProps))
	for _, vm := range vmProps {
		results = append(results, VMInfo{
			Name:    vm.Name,
			VCPU:    vm.Config.Hardware.NumCPU,
			RAMMB:   vm.Config.Hardware.MemoryMB,
			Storage: vm.Summary.Storage.Committed,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// WriteVSwitches formats and writes switch info to the given writer.
// Extracted from inline RunE formatting for readability (L5).
func WriteVSwitches(w io.Writer, switches []SwitchInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, sw := range switches {
		if len(sw.PortGroups) == 0 {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				sw.SwitchName, sw.SwitchType,
				"-", "-", sw.Uplinks, sw.LACP, sw.Ports, sw.UsedPorts,
			)
		} else {
			for i, pg := range sw.PortGroups {
				if i == 0 {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
						sw.SwitchName, sw.SwitchType,
						pg.Name, pg.VLAN, sw.Uplinks, sw.LACP, sw.Ports, sw.UsedPorts,
					)
				} else {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
						"", "", pg.Name, pg.VLAN, "", "", 0, 0,
					)
				}
			}
		}
	}
	return tw.Flush()
}
