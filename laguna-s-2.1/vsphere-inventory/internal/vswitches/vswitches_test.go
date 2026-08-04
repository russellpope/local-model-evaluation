package vswitches

import (
	"context"
	"sort"
	"testing"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestGetSwitches(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 2
	simModel.Portgroup = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches() error = %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("GetSwitches() should return at least one switch")
		}

		hasStandard := false
		hasDistributed := false
		for _, sw := range switches {
			if sw.Portgroup == "" {
				t.Error("Portgroup should not be empty")
			}

			if sw.UsedPorts > sw.TotalPorts {
				t.Errorf("Switch %s/%s: used ports (%d) > total ports (%d)",
					sw.Name, sw.Portgroup, sw.UsedPorts, sw.TotalPorts)
			}

			validLACP := map[string]bool{
				"disabled": true,
				"N/A":      true,
			}
			if !validLACP[sw.LACP] {
				t.Errorf("Switch %s/%s: LACP %q is not valid", sw.Name, sw.Portgroup, sw.LACP)
			}

			validTypes := map[string]bool{
				"standard":    true,
				"distributed": true,
			}
			if !validTypes[sw.Type] {
				t.Errorf("Switch %s/%s: type %q is not valid", sw.Name, sw.Portgroup, sw.Type)
			}

			if sw.Type == "standard" {
				hasStandard = true
				if sw.TotalPorts != 1536 {
					t.Errorf("Standard switch %s: TotalPorts = %d, want 1536", sw.Name, sw.TotalPorts)
				}
				if sw.UsedPorts != 6 {
					t.Errorf("Standard switch %s: UsedPorts = %d, want 6", sw.Name, sw.UsedPorts)
				}
				if sw.Uplinks != "vmnic0" {
					t.Errorf("Standard switch %s: Uplinks = %q, want %q", sw.Name, sw.Uplinks, "vmnic0")
				}
			}

			if sw.Type == "distributed" {
				hasDistributed = true
			}
		}

		if !hasStandard {
			t.Error("GetSwitches() should return at least one standard switch")
		}
		if !hasDistributed {
			t.Error("GetSwitches() should return at least one distributed switch")
		}
	}, simModel)
}

func TestGetVMsByPortgroup(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 1
	simModel.Machine = 3
	simModel.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		dc, err := finder.DefaultDatacenter(ctx)
		if err != nil {
			t.Fatalf("finding default datacenter: %v", err)
		}
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			t.Fatalf("finding VMs: %v", err)
		}

		if len(vms) == 0 {
			t.Fatal("no VMs found in simulator")
		}

		var portgroupName string
		for _, vm := range vms {
			var vmMo mo.VirtualMachine
			err := vm.Properties(ctx, vm.Reference(), []string{
				"name",
				"network",
			}, &vmMo)
			if err != nil {
				continue
			}

			if len(vmMo.Network) > 0 {
				netRef := vmMo.Network[0]
				common := object.NewCommon(c, netRef)

				var netMo mo.Network
				err := common.Properties(ctx, netRef, []string{"name"}, &netMo)
				if err != nil {
					continue
				}

				portgroupName = netMo.Name
				break
			}
		}

		if portgroupName == "" {
			t.Fatal("no port group found for any VM")
		}

		vmsList, err := GetVMsByPortgroup(ctx, c, portgroupName)
		if err != nil {
			t.Fatalf("GetVMsByPortgroup() error = %v", err)
		}

		if len(vmsList) != 3 {
			t.Fatalf("GetVMsByPortgroup(%q) returned %d VMs, want 3", portgroupName, len(vmsList))
		}

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		expectedNames := []string{
			"DC0_C0_RP0_VM0",
			"DC0_C0_RP0_VM1",
			"DC0_C0_RP0_VM2",
		}
		for i, vm := range vmsList {
			if vm.Name != expectedNames[i] {
				t.Errorf("VM at index %d: Name = %q, want %q", i, vm.Name, expectedNames[i])
			}
			if vm.VCPU != 1 {
				t.Errorf("VM %s: VCPU = %d, want 1", vm.Name, vm.VCPU)
			}
			if vm.RAMMB != 32 {
				t.Errorf("VM %s: RAMMB = %d, want 32", vm.Name, vm.RAMMB)
			}
			if vm.StorageBytes != 234 {
				t.Errorf("VM %s: StorageBytes = %d, want 234", vm.Name, vm.StorageBytes)
			}
		}
	}, simModel)
}

func TestGetVMsByPortgroupSubset(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 1
	simModel.Machine = 5
	simModel.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		dc, err := finder.DefaultDatacenter(ctx)
		if err != nil {
			t.Fatalf("finding default datacenter: %v", err)
		}
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			t.Fatalf("finding VMs: %v", err)
		}

		if len(vms) != 5 {
			t.Fatalf("expected 5 VMs, got %d", len(vms))
		}

		stdNetwork, err := finder.Network(ctx, "VM Network")
		if err != nil {
			t.Fatalf("finding standard port group: %v", err)
		}
		stdRef := stdNetwork.Reference()

		for i := 2; i < 5; i++ {
			var vmMo mo.VirtualMachine
			err := vms[i].Properties(ctx, vms[i].Reference(), []string{"config.hardware"}, &vmMo)
			if err != nil {
				t.Fatalf("retrieving properties for VM %s: %v", vms[i].Name(), err)
			}

			var deviceChange []types.BaseVirtualDeviceConfigSpec
			for _, device := range vmMo.Config.Hardware.Device {
				if ethCard, ok := device.(*types.VirtualE1000); ok {
					ethCard.Backing = &types.VirtualEthernetCardNetworkBackingInfo{
						VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
							DeviceName: "VM Network",
						},
					}
					deviceChange = append(deviceChange, &types.VirtualDeviceConfigSpec{
						Operation: types.VirtualDeviceConfigSpecOperationEdit,
						Device:    ethCard,
					})
				}
			}

			if len(deviceChange) == 0 {
				continue
			}

			spec := types.VirtualMachineConfigSpec{
				DeviceChange: deviceChange,
			}
			task, err := vms[i].Reconfigure(ctx, spec)
			if err != nil {
				t.Fatalf("reconfiguring VM %s: %v", vms[i].Name(), err)
			}
			if err := task.Wait(ctx); err != nil {
				t.Fatalf("waiting for VM %s reconfigure: %v", vms[i].Name(), err)
			}
		}

		vmsList, err := GetVMsByPortgroup(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("GetVMsByPortgroup() error = %v", err)
		}

		if len(vmsList) != 2 {
			t.Fatalf("GetVMsByPortgroup(%q) returned %d VMs, want 2", "DC0_DVPG0", len(vmsList))
		}

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		expectedNames := []string{
			"DC0_C0_RP0_VM0",
			"DC0_C0_RP0_VM1",
		}
		for i, vm := range vmsList {
			if vm.Name != expectedNames[i] {
				t.Errorf("VM at index %d: Name = %q, want %q", i, vm.Name, expectedNames[i])
			}
		}

		_ = stdRef

		// Also test standard port group (VM Network) - the 3 reconfigured VMs
		stdVms, err := GetVMsByPortgroup(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("GetVMsByPortgroup(\"VM Network\") error = %v", err)
		}

		if len(stdVms) != 3 {
			t.Fatalf("GetVMsByPortgroup(\"VM Network\") returned %d VMs, want 3", len(stdVms))
		}

		sort.Slice(stdVms, func(i, j int) bool {
			return stdVms[i].Name < stdVms[j].Name
		})

		expectedStdNames := []string{
			"DC0_C0_RP0_VM2",
			"DC0_C0_RP0_VM3",
			"DC0_C0_RP0_VM4",
		}
		for i, vm := range stdVms {
			if vm.Name != expectedStdNames[i] {
				t.Errorf("VM at index %d: Name = %q, want %q", i, vm.Name, expectedStdNames[i])
			}
		}
	}, simModel)
}

func TestClassifyLACP(t *testing.T) {
	tests := []struct {
		name string
		info *types.VMwareDVSConfigInfo
		want string
	}{
		{
			name: "nil config -> N/A",
			info: nil,
			want: "N/A",
		},
		{
			name: "LACP API version with group config -> enabled",
			info: &types.VMwareDVSConfigInfo{
				LacpApiVersion: "1.0.0",
				LacpGroupConfig: []types.VMwareDvsLacpGroupConfig{
					{Name: "test"},
				},
			},
			want: "enabled",
		},
		{
			name: "LACP API version without group config -> disabled",
			info: &types.VMwareDVSConfigInfo{
				LacpApiVersion: "1.0.0",
			},
			want: "disabled",
		},
		{
			name: "no LACP API version -> N/A",
			info: &types.VMwareDVSConfigInfo{},
			want: "N/A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyLACP(tt.info)
			if got != tt.want {
				t.Errorf("classifyLACP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveVlanID(t *testing.T) {
	tests := []struct {
		name       string
		portConfig types.BaseDVPortSetting
		want       string
	}{
		{
			name:       "nil port setting",
			portConfig: nil,
			want:       "0",
		},
		{
			name: "VlanId == 0 -> 0",
			portConfig: &types.VMwareDVSPortSetting{
				DVPortSetting: types.DVPortSetting{},
				Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{
					VlanId: 0,
				},
			},
			want: "0",
		},
		{
			name: "VlanId == 4095 -> trunk",
			portConfig: &types.VMwareDVSPortSetting{
				DVPortSetting: types.DVPortSetting{},
				Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{
					VlanId: 4095,
				},
			},
			want: "trunk",
		},
		{
			name: "VlanId == 100 -> 100",
			portConfig: &types.VMwareDVSPortSetting{
				DVPortSetting: types.DVPortSetting{},
				Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{
					VlanId: 100,
				},
			},
			want: "100",
		},
		{
			name: "TrunkVlanSpec",
			portConfig: &types.VMwareDVSPortSetting{
				DVPortSetting: types.DVPortSetting{},
				Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
					VlanId: []types.NumericRange{
						{Start: 100, End: 200},
					},
				},
			},
			want: "100-200",
		},
		{
			name: "PvlanSpec",
			portConfig: &types.VMwareDVSPortSetting{
				DVPortSetting: types.DVPortSetting{},
				Vlan: &types.VmwareDistributedVirtualSwitchPvlanSpec{
					PvlanId: 1000,
				},
			},
			want: "pvlan:1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveVlanID(tt.portConfig)
			if got != tt.want {
				t.Errorf("resolveVlanID() = %q, want %q", got, tt.want)
			}
		})
	}
}
