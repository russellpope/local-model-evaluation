package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestProcessSwitches(t *testing.T) {
	dvs := []mo.DistributedVirtualSwitch{
		{
			ManagedEntity: mo.ManagedEntity{
				ExtensibleManagedObject: mo.ExtensibleManagedObject{
					Self: types.ManagedObjectReference{Type: "VmwareDistributedVirtualSwitch", Value: "dvs-1"},
				},
				Name: "DVS-1",
			},
			Config: &types.VMwareDVSConfigInfo{
				LacpGroupConfig: []types.VMwareDvsLacpGroupConfig{{Mode: "active"}},
			},
		},
	}
	dvpgs := []mo.DistributedVirtualPortgroup{
		{
			Network: mo.Network{
				ManagedEntity: mo.ManagedEntity{
					ExtensibleManagedObject: mo.ExtensibleManagedObject{
						Self: types.ManagedObjectReference{Type: "DistributedVirtualPortgroup", Value: "dvpg-1"},
					},
				},
				Name: "DVPG-1",
			},
			Config: types.DVPortgroupConfigInfo{
				DistributedVirtualSwitch: &types.ManagedObjectReference{Value: "dvs-1"},
				DefaultPortConfig: &types.VMwareDVSPortSetting{
					Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{VlanId: 10},
				},
			},
		},
	}
	hosts := []mo.HostSystem{
		{
			Config: &types.HostConfigInfo{
				Network: &types.HostNetworkInfo{
					Vswitch: []types.HostVirtualSwitch{
						{Name: "vSwitch0", NumPorts: 128, NumPortsAvailable: 100},
					},
					Portgroup: []types.HostPortGroup{
						{
							Spec: types.HostPortGroupSpec{Name: "VM Network", VlanId: 0, VswitchName: "vSwitch0"},
						},
					},
				},
			},
		},
	}
	portCounts := map[string]PortCount{
		"dvpg-1": {Total: 100, Used: 50},
	}

	result := processSwitches(dvs, dvpgs, hosts, portCounts)
	for _, r := range result {
		t.Logf("Row: Switch=%s, PG=%s, VLAN=%s, LACP=%s", r.Switch, r.Portgroup, r.VLAN, r.LACP)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result))
	}

	var foundDVS, foundVSS bool
	for _, row := range result {
		if row.Switch == "DVS-1" && row.Portgroup == "DVPG-1" {
			foundDVS = true
			if row.LACP != "Enabled" || row.VLAN != "10" {
				t.Errorf("DVS row incorrect: LACP=%s, VLAN=%s", row.LACP, row.VLAN)
			}
		}
		if row.Switch == "vSwitch0" && row.Portgroup == "VM Network" {
			foundVSS = true
		}
	}

	if !foundDVS {
		t.Error("DVS row not found")
	}
	if !foundVSS {
		t.Error("VSS row not found")
	}
}

func TestResolveVMsInPortgroup(t *testing.T) {
	pgRef := "net-1"
	vms := []mo.VirtualMachine{
		{
			ManagedEntity: mo.ManagedEntity{
				ExtensibleManagedObject: mo.ExtensibleManagedObject{
					Self: types.ManagedObjectReference{Value: "vm-1"},
				},
				Name: "VM-1",
			},
			Network: []types.ManagedObjectReference{
				{Value: "net-1"},
			},
		},
		{
			ManagedEntity: mo.ManagedEntity{
				ExtensibleManagedObject: mo.ExtensibleManagedObject{
					Self: types.ManagedObjectReference{Value: "vm-2"},
				},
				Name: "VM-2",
			},
			Network: []types.ManagedObjectReference{
				{Value: "net-2"},
			},
		},
		{
			ManagedEntity: mo.ManagedEntity{
				ExtensibleManagedObject: mo.ExtensibleManagedObject{
					Self: types.ManagedObjectReference{Value: "vm-3"},
				},
				Name: "VM-3",
			},
			Network: []types.ManagedObjectReference{
				{Value: "net-1"},
				{Value: "net-2"},
			},
		},
	}

	var result []string
	for _, vm := range vms {
		for _, net := range vm.Network {
			if net.Value == pgRef {
				result = append(result, vm.Name)
				break
			}
		}
	}

	if len(result) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(result))
	}

	vmMap := make(map[string]bool)
	for _, name := range result {
		vmMap[name] = true
	}

	if !vmMap["VM-1"] || !vmMap["VM-3"] {
		t.Errorf("incorrect VMs resolved: %v", result)
	}
}
