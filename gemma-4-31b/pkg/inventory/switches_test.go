package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestProcessSwitches(t *testing.T) {
	dvs := []mo.DistributedVirtualSwitch{
		{
			Reference: mo.NewReference("dvs-123"),
			Name:       "DVS-1",
			Config: &types.DistributedVirtualSwitchConfig{
				TeamingPolicy: &types.DvSwitchTeamingPolicy{
					LoadBalancing: "loadBalanceLACP",
				},
			},
		},
	}
	dvpgs := []mo.DistributedVirtualPortgroup{
		{
			Name: "DVPG-1",
			Config: &types.DistributedVirtualPortgroupConfig{
				DistributedVirtualSwitch: mo.NewReference("dvs-123"),
				DefaultPortgroupVlan:     &types.DvPortgroupVlan{VlanId: 10},
			},
			Summary: &types.DistributedVirtualPortgroupSummary{
				NumPorts:          100,
				EffectiveNumPorts: 80,
			},
		},
	}
	hosts := []mo.HostSystem{
		{
			Name: "Host-1",
			Config: &types.HostConfig{
				Network: &types.HostNetworkConfig{
					vSwitch: []types.HostVirtualSwitch{
						{
							Name: "vSwitch0",
							Portgroup: []types.HostPortgroup{
								{
									Name:   "PG-Standard",
									VlanId: int32Ptr(20),
								},
							},
						},
					},
				},
			},
		},
	}

	result := processSwitches(dvs, dvpgs, hosts)

	if len(result) != 2 {
		t.Fatalf("expected 2 switches, got %d", len(result))
	}

	// Check DVPG
	dvpgRow := result[0]
	if dvpgRow.Switch != "DVS-1" || dvpgRow.Portgroup != "DVPG-1" || dvpgRow.LACP != "Enabled" || dvpgRow.Used != 20 || dvpgRow.Ports != 100 {
		t.Errorf("unexpected DVPG row: %+v", dvpgRow)
	}

	// Check Standard PG
	stdRow := result[1]
	if stdRow.Switch != "vSwitch0" || stdRow.Portgroup != "PG-Standard" || stdRow.VLAN != "20" {
		t.Errorf("unexpected Standard row: %+v", stdRow)
	}
}

func TestProcessVMsInPortgroup(t *testing.T) {
	vms := []mo.VirtualMachine{
		{
			Name: "VM-1",
			Config: &types.VirtualMachineConfig{
				Hardware: &types.VirtualMachineHardware{
					Device: []types.BaseVirtualDevice{
						&types.VirtualEthernetCard{
							Backing: &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
								Port: &types.DistributedVirtualPort{
									PortgroupKey: "pg-key-123",
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "VM-2",
			Config: &types.VirtualMachineConfig{
				Hardware: &types.VirtualMachineHardware{
					Device: []types.BaseVirtualDevice{
						&types.VirtualEthernetCard{
							Backing: &types.VirtualEthernetCardNetworkBackingInfo{
								DeviceName: "PG-Standard",
							},
						},
					},
				},
			},
		},
		{
			Name: "VM-3",
			Config: &types.VirtualMachineConfig{
				Hardware: &types.VirtualMachineHardware{
					Device: []types.BaseVirtualDevice{
						&types.VirtualEthernetCard{
							Backing: &types.VirtualEthernetCardNetworkBackingInfo{
								DeviceName: "Wrong-PG",
							},
						},
					},
				},
			},
		},
	}

	t.Run("DistributedPortgroup", func(t *testing.T) {
		res := processVMsInPortgroup(vms, "pg-key-123", "DVPG-1")
		if len(res) != 1 || res[0] != "VM-1" {
			t.Errorf("expected [VM-1], got %v", res)
		}
	})

	t.Run("StandardPortgroup", func(t *testing.T) {
		res := processVMsInPortgroup(vms, "", "PG-Standard")
		if len(res) != 1 || res[0] != "VM-2" {
			t.Errorf("expected [VM-2], got %v", res)
		}
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}
