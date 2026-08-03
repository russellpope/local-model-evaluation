package vswitches

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type SwitchInfo struct {
	Name       string
	Type       string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	TotalPorts int
	UsedPorts  int
}

type VMInfo struct {
	Name         string
	VCPU         int
	RAMMB        int
	StorageBytes int64
}

func GetSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(client)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding default datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	var result []SwitchInfo

	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing host systems: %w", err)
	}

	for _, host := range hosts {
		var hostMo mo.HostSystem
		err := host.Properties(ctx, host.Reference(), []string{
			"name",
			"config.network",
		}, &hostMo)
		if err != nil {
			continue
		}

		if hostMo.Config == nil || hostMo.Config.Network == nil {
			continue
		}

		networkInfo := hostMo.Config.Network

		portGroupMap := make(map[string]types.HostPortGroup)
		for _, pg := range networkInfo.Portgroup {
			portGroupMap[pg.Spec.Name] = pg
		}

		for _, vsw := range networkInfo.Vswitch {
			for _, pgName := range vsw.Portgroup {
				pg, ok := portGroupMap[pgName]
				if !ok {
					continue
				}

				vlanID := "N/A"
				if pg.Spec.VlanId != 0 {
					vlanID = fmt.Sprintf("%d", pg.Spec.VlanId)
				}

				uplinks := "N/A"
				if len(vsw.Pnic) > 0 {
					uplinks = vsw.Pnic[0]
				}

				totalPorts := int(vsw.NumPorts)
				usedPorts := totalPorts - int(vsw.NumPortsAvailable)
				if usedPorts < 0 {
					usedPorts = 0
				}

				result = append(result, SwitchInfo{
					Name:       vsw.Name,
					Type:       "standard",
					Portgroup:  pg.Spec.Name,
					VLAN:       vlanID,
					Uplinks:    uplinks,
					LACP:       "N/A",
					TotalPorts: totalPorts,
					UsedPorts:  usedPorts,
				})
			}
		}
	}

	networks, err := finder.NetworkList(ctx, "*")
	if err == nil {
		for _, net := range networks {
			ref := net.Reference()
			common := object.NewCommon(client, ref)

			var netMo mo.Network
			err := common.Properties(ctx, ref, []string{
				"name",
				"vm",
			}, &netMo)
			if err != nil {
				continue
			}

			switch net.(type) {
			case *object.DistributedVirtualPortgroup:
				dvpCommon := object.NewCommon(client, ref)
				var dvpMo mo.DistributedVirtualPortgroup
				err := dvpCommon.Properties(ctx, ref, []string{
					"name",
					"config",
				}, &dvpMo)
				if err != nil {
					continue
				}

				vlanID := "N/A"
				if dvpMo.Config.DefaultPortConfig != nil {
					vlanID = "N/A"
				}

				totalPorts := int(dvpMo.Config.NumPorts)
				usedPorts := 0

				result = append(result, SwitchInfo{
					Name:       "N/A",
					Type:       "distributed",
					Portgroup:  dvpMo.Name,
					VLAN:       vlanID,
					Uplinks:    "N/A",
					LACP:       "N/A",
					TotalPorts: totalPorts,
					UsedPorts:  usedPorts,
				})
			}
		}
	}

	return result, nil
}

func GetVMsByPortgroup(ctx context.Context, client *vim25.Client, portgroupName string) ([]VMInfo, error) {
	finder := find.NewFinder(client)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding default datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	pg, err := finder.Network(ctx, portgroupName)
	if err != nil {
		return nil, fmt.Errorf("finding port group %q: %w", portgroupName, err)
	}

	pgRef := pg.Reference()

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing virtual machines: %w", err)
	}

	var result []VMInfo
	for _, vm := range vms {
		var vmMo mo.VirtualMachine
		err := vm.Properties(ctx, vm.Reference(), []string{
			"name",
			"network",
			"config.hardware.numCPU",
			"config.hardware.memoryMB",
			"summary.storage.committed",
		}, &vmMo)
		if err != nil {
			continue
		}

		connected := false
		for _, netRef := range vmMo.Network {
			if netRef == pgRef {
				connected = true
				break
			}
		}

		if !connected {
			continue
		}

		var committed int64
		if vmMo.Summary.Storage != nil {
			committed = vmMo.Summary.Storage.Committed
		}

		result = append(result, VMInfo{
			Name:         vmMo.Name,
			VCPU:         int(vmMo.Config.Hardware.NumCPU),
			RAMMB:        int(vmMo.Config.Hardware.MemoryMB),
			StorageBytes: committed,
		})
	}

	return result, nil
}
