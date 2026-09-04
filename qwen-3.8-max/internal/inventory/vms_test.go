package inventory_test

import (
	"context"
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/internal/inventory"
)

func TestListVMs(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 4
	model.Datastore = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		want := model.Count().Machine
		if want == 0 {
			t.Fatal("simulator model has no VMs")
		}

		infos, err := inventory.ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(infos) != want {
			t.Fatalf("ListVMs returned %d VMs, want %d", len(infos), want)
		}
		for _, vm := range infos {
			if vm.Name == "" {
				t.Error("VM with empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %q: VCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.RAMMB <= 0 {
				t.Errorf("VM %q: RAMMB = %d, want > 0", vm.Name, vm.RAMMB)
			}
			if vm.CommittedBytes < 0 {
				t.Errorf("VM %q: CommittedBytes = %d, want >= 0", vm.Name, vm.CommittedBytes)
			}
		}
		if !sort.SliceIsSorted(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name }) {
			t.Error("VMs are not sorted by name")
		}
	}, model)
}
