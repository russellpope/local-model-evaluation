package vms

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetVMs(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 1
	simModel.Machine = 5
	simModel.Pool = 0

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vmsList, err := GetVMs(ctx, c)
		if err != nil {
			t.Fatalf("GetVMs() error = %v", err)
		}

		if len(vmsList) != 5 {
			t.Fatalf("GetVMs() returned %d VMs, want 5", len(vmsList))
		}

		for _, vm := range vmsList {
			if vm.Name == "" {
				t.Error("VM name should not be empty")
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
