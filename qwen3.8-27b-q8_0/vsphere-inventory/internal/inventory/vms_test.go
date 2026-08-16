package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func smallModel() *simulator.Model {
	m := simulator.VPX()
	m.Host = 0
	m.ClusterHost = 1
	m.Machine = 3
	m.Datastore = 1
	m.Portgroup = 1
	return m
}

func TestListVMs(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}

		if len(vms) != 3 {
			t.Fatalf("ListVMs returned %d VMs, want 3", len(vms))
		}

		for _, vm := range vms {
			if vm.Name == "" {
				t.Errorf("VM with empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %s: vCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.MemoryMB <= 0 {
				t.Errorf("VM %s: memory = %d MB, want > 0", vm.Name, vm.MemoryMB)
			}
			if vm.Committed < 0 {
				t.Errorf("VM %s: committed = %d, want >= 0", vm.Name, vm.Committed)
			}
		}

		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name >= vms[i].Name {
				t.Errorf("VMs not sorted by name: %q before %q", vms[i-1].Name, vms[i].Name)
			}
		}
	}, smallModel())
}
