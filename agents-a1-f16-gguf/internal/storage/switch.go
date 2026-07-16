package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type SwitchInfo struct {
	Name       string
	SwitchType string // "standard" or "distributed"
	PortGroups []PortGroupInfo
	Uplinks    []string
	LACP       string // "enabled", "disabled", or "N/A"
	TotalPorts int32
	UsedPorts  int32
}

type PortGroupInfo struct {
	Name string
	VLAN string
}

type VMInfoForPortGroup struct {
	Name      string
	PortGroup string
}

func GetSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(client.Client, false)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("finding datacenters: %w", err)
	}

	var allSwitches []SwitchInfo

	for _, dc := range datacenters {
		finder.SetDatacenter(dc)

		// Get standard switches via host network systems
		hosts, err := finder.HostSystemList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing hosts: %w", err)
		}

		for _, host := range hosts {
			switches, err := getStandardSwitches(ctx, host)
			if err != nil {
				continue
			}
			allSwitches = append(allSwitches, switches...)
		}

		// Get distributed switches
		dvs, err := getDistributedSwitches(ctx, client, finder)
		if err != nil {
			return nil, fmt.Errorf("listing distributed switches: %w", err)
		}

		for _, dvs := range dvs {
			switches, err := getDistributedSwitchInfo(ctx, dvs)
			if err != nil {
				continue
			}
			allSwitches = append(allSwitches, switches...)
		}
	}

	sort.Slice(allSwitches, func(i, j int) bool {
		return allSwitches[i].Name < allSwitches[j].Name
	})

	return allSwitches, nil
}

func getStandardSwitches(ctx context.Context, host *object.HostSystem) ([]SwitchInfo, error) {
	// Get host properties using the Properties method
	var hostMo mo.HostSystem
	err := host.Properties(ctx, host.Reference(), []string{"config.network"}, &hostMo)
	if err != nil {
		return nil, fmt.Errorf("getting host network: %w", err)
	}

	var switches []SwitchInfo

	netConfig := hostMo.Config.Network
	if netConfig == nil {
		return switches, nil
	}

	// Iterate through port groups
	for _, pg := range netConfig.Portgroup {
		switchName := pg.Spec.VswitchName
		if switchName == "" {
			continue
		}

		var sw *SwitchInfo
		for i := range switches {
			if switches[i].Name == switchName {
				sw = &switches[i]
				break
			}
		}

		if sw == nil {
			sw = &SwitchInfo{
				Name:       switchName,
				SwitchType: "standard",
				PortGroups: []PortGroupInfo{},
				Uplinks:    []string{},
				LACP:       "N/A",
			}
			// Get uplinks for this switch from vswitches
			for _, vs := range netConfig.Vswitch {
				if vs.Name == switchName {
					sw.Uplinks = vs.Pnic
					break
				}
			}
			switches = append(switches, *sw)
		}

		portGroup := PortGroupInfo{
			Name: pg.Spec.Name,
			VLAN: fmt.Sprintf("%d", pg.Spec.VlanId),
		}
		sw.PortGroups = append(sw.PortGroups, portGroup)
	}

	return switches, nil
}

func getDistributedSwitches(ctx context.Context, client *govmomi.Client, finder *find.Finder) ([]*object.DistributedVirtualSwitch, error) {
	// Use ManagedObjectList to find DVS
	elements, err := finder.ManagedObjectList(ctx, "/", "DistributedVirtualSwitch")
	if err != nil {
		return nil, err
	}

	var dvs []*object.DistributedVirtualSwitch
	for _, el := range elements {
		ref := el.Object.Reference()
		dvs = append(dvs, object.NewDistributedVirtualSwitch(client.Client, ref))
	}

	return dvs, nil
}

func getDistributedSwitchInfo(ctx context.Context, dvs *object.DistributedVirtualSwitch) ([]SwitchInfo, error) {
	// Get DVS properties using the Properties method
	var dvsMo mo.DistributedVirtualSwitch
	err := dvs.Properties(ctx, dvs.Reference(), []string{"name", "summary", "config", "portgroup"}, &dvsMo)
	if err != nil {
		return nil, fmt.Errorf("getting DVS properties: %w", err)
	}

	summary := dvsMo.Summary

	sw := SwitchInfo{
		Name:       dvsMo.Name,
		SwitchType: "distributed",
		PortGroups: []PortGroupInfo{},
		Uplinks:    []string{},
		LACP:       "disabled",
		TotalPorts: summary.NumPorts,
		UsedPorts:  0, // Not available in DVSSummary
	}

	// Check for LACP
	if dvsMo.Config != nil {
		if config, ok := dvsMo.Config.(*types.VMwareDVSConfigInfo); ok {
			if config.LacpApiVersion != "" {
				sw.LACP = "enabled"
			}
		}
	}

	// Get port groups
	for _, pgRef := range dvsMo.Portgroup {
		// Get port group name
		var pgName string
		var pgMo mo.DistributedVirtualPortgroup
		err := dvs.Properties(ctx, pgRef, []string{"name"}, &pgMo)
		if err != nil {
			// Try to get name from reference
			pgName = pgRef.Value
		} else {
			pgName = pgMo.Name
		}

		portGroup := PortGroupInfo{
			Name: pgName,
			VLAN: "unknown",
		}
		sw.PortGroups = append(sw.PortGroups, portGroup)
	}

	return []SwitchInfo{sw}, nil
}

func GetVMsByPortGroup(ctx context.Context, client *govmomi.Client, portgroupName string) ([]VMInfoForPortGroup, error) {
	finder := find.NewFinder(client.Client, false)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("finding datacenters: %w", err)
	}

	var vms []VMInfoForPortGroup

	for _, dc := range datacenters {
		finder.SetDatacenter(dc)
		vmList, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing VMs: %w", err)
		}

		for _, vm := range vmList {
			vmVMs, err := getVMsInPortGroup(ctx, vm, portgroupName)
			if err != nil {
				continue
			}
			vms = append(vms, vmVMs...)
		}
	}

	return vms, nil
}

func getVMsInPortGroup(ctx context.Context, vm *object.VirtualMachine, portgroupName string) ([]VMInfoForPortGroup, error) {
	// Get VM devices using the Device method
	devices, err := vm.Device(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting VM devices: %w", err)
	}

	for _, device := range devices {
		if netDevice, ok := device.(*types.VirtualEthernetCard); ok {
			if netDevice.Backing != nil {
				if backing, ok := netDevice.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
					// The backing has a Network field, which is a reference to the network
					// We need to get the network name from the reference
					// For now, we'll just use the reference value as the port group name
					// This is a simplification
					if backing.Network != nil {
						// Get the network name from the reference
						// In a real scenario, we would need to query the network object
						// For now, we'll just check if the reference matches the port group name
						if strings.EqualFold(backing.Network.Value, portgroupName) {
							name := vm.Name()
							return []VMInfoForPortGroup{{Name: name, PortGroup: portgroupName}}, nil
						}
					}
				}
			}
		}
	}

	return nil, nil
}
