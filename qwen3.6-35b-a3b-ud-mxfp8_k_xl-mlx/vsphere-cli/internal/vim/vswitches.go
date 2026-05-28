package vim

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/mo"
)

type SwitchInfo struct {
	SwitchName string
	SwitchType string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	TotalPorts int32
	UsedPorts  int32
}

func (c *Client) GetVSwitches(ctx context.Context) ([]SwitchInfo, error) {
	var switches []SwitchInfo

	// Get standard vSwitches
	v, err := c.View.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating host view: %w", err)
	}
	defer v.Destroy(ctx)

	var hosts []mo.HostSystem

	err = v.Retrieve(ctx, []string{"HostSystem"}, nil, &hosts)
	if err != nil {
		return nil, fmt.Errorf("retrieving hosts: %w", err)
	}

	for _, host := range hosts {
		for _, net := range host.Config.Network.Portgroup {
			si := SwitchInfo{
				SwitchName: net.Spec.VswitchName,
				Portgroup:  net.Spec.Name,
			}

			si.SwitchType = "standard"
			si.LACP = "N/A"

			if net.Spec.VlanId != 0 {
				si.VLAN = fmt.Sprintf("%d", net.Spec.VlanId)
			} else {
				si.VLAN = "trunk"
			}

			switches = append(switches, si)
		}
	}

	return switches, nil
}

func (c *Client) GetVMsByPortgroup(ctx context.Context, portgroupName string) ([]VMInfo, error) {
	v, err := c.View.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating VM view: %w", err)
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine

	err = v.Retrieve(ctx, []string{"VirtualMachine"}, nil, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieving VMs: %w", err)
	}

	var result []VMInfo
	for _, vm := range vms {
		result = append(result, VMInfo{Name: vm.Name})
	}

	return result, nil
}

func formatUplinks(nics []string) string {
	if len(nics) == 0 {
		return "none"
	}
	result := nics[0]
	for i := 1; i < len(nics); i++ {
		result += ", " + nics[i]
	}
	return result
}

func formatSlice(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += ", " + items[i]
	}
	return result
}
