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
)

func TestGetSwitches(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 2
	model.Portgroup = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches() error = %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("GetSwitches() should return at least one switch")
		}

		hasStandard := false
		for _, sw := range switches {
			if sw.Portgroup == "" {
				t.Error("Portgroup should not be empty")
			}

			if sw.UsedPorts > sw.TotalPorts {
				t.Errorf("Switch %s/%s: used ports (%d) > total ports (%d)",
					sw.Name, sw.Portgroup, sw.UsedPorts, sw.TotalPorts)
			}

			validLACP := map[string]bool{
				"enabled":  true,
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
			}
		}

		if !hasStandard {
			t.Error("GetSwitches() should return at least one standard switch")
		}
	}, model)
}

func TestGetVMsByPortgroup(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 1
	model.Machine = 3
	model.Portgroup = 2

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

		if len(vmsList) == 0 {
			t.Fatalf("GetVMsByPortgroup(%q) should return at least one VM", portgroupName)
		}

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		for _, vm := range vmsList {
			if vm.Name == "" {
				t.Error("VM name should not be empty")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %s: VCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.RAMMB <= 0 {
				t.Errorf("VM %s: RAMMB = %d, want > 0", vm.Name, vm.RAMMB)
			}
			if vm.StorageBytes < 0 {
				t.Errorf("VM %s: StorageBytes = %d, want >= 0", vm.Name, vm.StorageBytes)
			}
		}
	}, model)
}
