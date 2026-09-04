package inventory_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/inventory"
)

func collectVMs(t *testing.T, ctx context.Context, c *vim25.Client) []mo.VirtualMachine {
	t.Helper()
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		t.Fatalf("create VM view: %v", err)
	}
	defer v.Destroy(ctx)
	var vms []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "network"}, &vms); err != nil {
		t.Fatalf("retrieve VMs: %v", err)
	}
	return vms
}

func sortedNames(vms []mo.VirtualMachine) []string {
	names := make([]string, 0, len(vms))
	for _, vm := range vms {
		names = append(names, vm.Name)
	}
	sort.Strings(names)
	return names
}

func TestVMsOnDistributedPortgroup(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 3
	model.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		m := view.NewManager(c)
		v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"DistributedVirtualPortgroup"}, true)
		if err != nil {
			t.Fatalf("create portgroup view: %v", err)
		}
		defer v.Destroy(ctx)
		var pgs []mo.DistributedVirtualPortgroup
		if err := v.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"config.name"}, &pgs); err != nil {
			t.Fatalf("retrieve portgroups: %v", err)
		}
		nameOf := make(map[types.ManagedObjectReference]string, len(pgs))
		for _, pg := range pgs {
			nameOf[pg.Self] = pg.Config.Name
		}

		vms := collectVMs(t, ctx, c)
		attached := map[string][]string{}
		for _, vm := range vms {
			for _, ref := range vm.Network {
				if name, ok := nameOf[ref]; ok {
					attached[name] = append(attached[name], vm.Name)
				}
			}
		}
		if len(attached) == 0 {
			t.Fatal("no VM attached to any distributed port group")
		}

		for name, want := range attached {
			sort.Strings(want)
			got, err := inventory.VMsOnPortgroup(ctx, c, name)
			if err != nil {
				t.Fatalf("VMsOnPortgroup(%q): %v", name, err)
			}
			gotNames := make([]string, 0, len(got))
			for _, vm := range got {
				gotNames = append(gotNames, vm.Name)
			}
			if !reflect.DeepEqual(gotNames, want) {
				t.Errorf("VMsOnPortgroup(%q) = %v, want %v", name, gotNames, want)
			}
		}

		inUse := ""
		for name := range attached {
			if len(attached[name]) == model.Count().Machine {
				inUse = name
			}
		}
		if inUse == "" {
			t.Fatal("expected one distributed port group to carry every VM")
		}
	}, model)
}

func TestVMsOnStandardPortgroup(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 3
	model.Portgroup = 0

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms := collectVMs(t, ctx, c)
		want := sortedNames(vms)
		if len(want) == 0 {
			t.Fatal("simulator model has no VMs")
		}

		got, err := inventory.VMsOnPortgroup(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("VMsOnPortgroup(VM Network): %v", err)
		}
		gotNames := make([]string, 0, len(got))
		for _, vm := range got {
			gotNames = append(gotNames, vm.Name)
		}
		if !reflect.DeepEqual(gotNames, want) {
			t.Errorf("VMsOnPortgroup(VM Network) = %v, want %v", gotNames, want)
		}

		empty, err := inventory.VMsOnPortgroup(ctx, c, "Management Network")
		if err != nil {
			t.Fatalf("VMsOnPortgroup(Management Network): %v", err)
		}
		if len(empty) != 0 {
			t.Errorf("VMsOnPortgroup(Management Network) = %v, want no VMs", empty)
		}
	}, model)
}

func TestVMsOnUnknownPortgroup(t *testing.T) {
	model := simulator.VPX()
	model.Portgroup = 1

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		_, err := inventory.VMsOnPortgroup(ctx, c, "no-such-portgroup")
		if err == nil {
			t.Fatal("VMsOnPortgroup with unknown port group: expected error, got nil")
		}
	}, model)
}
