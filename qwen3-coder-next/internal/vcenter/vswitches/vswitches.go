package vswitches

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/model"
)

func ListVSwitches(ctx context.Context, client *vim25.Client) ([]model.SwitchInfo, error) {
	finder := find.NewFinder(client, false)

	hostList, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}

	var switches []model.SwitchInfo
	for _, host := range hostList {
		var props struct {
			Name          string
			ConfigManager types.HostConfigManager
		}

		pc := property.DefaultCollector(client)
		if err := pc.RetrieveOne(ctx, host.Reference(), []string{"name", "configManager.networkSystem"}, &props); err != nil {
			return nil, fmt.Errorf("failed to get host properties: %w", err)
		}

		if props.ConfigManager.NetworkSystem == nil {
			continue
		}

		var netConfig types.HostNetworkConfig

		if err := pc.RetrieveOne(ctx, *props.ConfigManager.NetworkSystem, []string{"networkConfig"}, &netConfig); err != nil {
			continue
		}

		for _, vswitch := range netConfig.Vswitch {
			uplinks := []string{}

			switchType := "standard"
			lacp := "N/A"

			if vswitch.Spec != nil {
				if vswitch.Spec.Bridge != nil {
					switch t := vswitch.Spec.Bridge.(type) {
					case *types.HostVirtualSwitchBridge:
						_ = t
					}
				}
			}

			if netConfig.Portgroup != nil {
				for _, pg := range netConfig.Portgroup {
					vlan := fmt.Sprintf("%d", pg.Spec.VlanId)
					if pg.Spec.VlanId == 4095 {
						vlan = "trunk"
					}

					switches = append(switches, model.SwitchInfo{
						SwitchName:    vswitch.Name,
						SwitchType:    switchType,
						PortgroupName: pg.Spec.Name,
						VLAN:          vlan,
						Uplinks:       uplinks,
						LACP:          lacp,
						TotalPorts:    vswitch.Spec.NumPorts,
						UsedPorts:     0,
					})
				}
			}
		}

		for _, dvs := range netConfig.ProxySwitch {
			uplinks := []string{}

			lacp := "disabled"

			switches = append(switches, model.SwitchInfo{
				SwitchName:    dvs.Uuid,
				SwitchType:    "distributed",
				PortgroupName: "",
				VLAN:          "",
				Uplinks:       uplinks,
				LACP:          lacp,
				TotalPorts:    0,
				UsedPorts:     0,
			})
		}
	}

	return switches, nil
}

func ListVMsByPortgroup(ctx context.Context, client *vim25.Client, portgroupName string) ([]model.VMInfo, error) {
	return ListVMsByPortgroup(ctx, client, portgroupName)
}

func init() {
	_ = find.NewFinder
	_ = property.DefaultCollector
}
