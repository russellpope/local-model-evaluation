package vms

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetVMs(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 5

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		results, err := GetVMs(ctx, c)
		if err != nil {
			t.Fatalf("GetVMs: %v", err)
		}

		expectedCount := model.Machine * (model.Host + model.Cluster)
		if len(results) != expectedCount {
			t.Errorf("expected %d VMs, got %d", expectedCount, len(results))
		}

		for _, vm := range results {
			if vm.Name == "" {
				t.Error("VM has empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %q has VCPU <= 0: %d", vm.Name, vm.VCPU)
			}
			if vm.RAM <= 0 {
				t.Errorf("VM %q has RAM <= 0: %d", vm.Name, vm.RAM)
			}
			if vm.Storage < 0 {
				t.Errorf("VM %q has storage < 0: %d", vm.Name, vm.Storage)
			}
		}

		for i := 1; i < len(results); i++ {
			if results[i-1].Name > results[i].Name {
				t.Error("VMs are not sorted by name")
			}
		}
	}, model)
}

func TestGetVMsDefault(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		results, err := GetVMs(ctx, c)
		if err != nil {
			t.Fatalf("GetVMs: %v", err)
		}

		if len(results) < 1 {
			t.Error("expected at least 1 VM, got 0")
		}

		for _, vm := range results {
			if vm.Name == "" {
				t.Error("VM has empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %q has VCPU <= 0: %d", vm.Name, vm.VCPU)
			}
			if vm.RAM <= 0 {
				t.Errorf("VM %q has RAM <= 0: %d", vm.Name, vm.RAM)
			}
		}
	})
}
