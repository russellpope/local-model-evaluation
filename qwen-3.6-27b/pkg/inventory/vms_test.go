package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestListVMs(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 1
	model.ClusterHost = 2
	model.Pool = 1
	model.Machine = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		client := &govmomi.Client{
			Client:         c,
			SessionManager: session.NewManager(c),
		}

		vms, err := ListVMs(ctx, client)
		if err != nil {
			t.Fatalf("ListVMs failed: %v", err)
		}

		expectedCount := model.Datacenter * model.Cluster * (model.Pool + 1) * model.Machine
		if len(vms) != expectedCount {
			t.Errorf("expected %d VMs, got %d", expectedCount, len(vms))
		}

		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("VM has empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %s has vCPU <= 0: %d", vm.Name, vm.VCPU)
			}
			if vm.RAMMB <= 0 {
				t.Errorf("VM %s has RAMMB <= 0: %d", vm.Name, vm.RAMMB)
			}
			if vm.Storage < 0 {
				t.Errorf("VM %s has negative storage: %d", vm.Name, vm.Storage)
			}
		}
	}, model)
}
