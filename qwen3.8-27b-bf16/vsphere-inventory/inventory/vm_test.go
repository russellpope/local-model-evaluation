package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

func TestListVMs(t *testing.T) {
	withClient(t, func(ctx context.Context, c *vim25.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) != 3 {
			t.Fatalf("expected 3 VMs, got %d: %+v", len(vms), vms)
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Fatalf("VM with empty name: %+v", vm)
			}
			if vm.VCPU <= 0 {
				t.Fatalf("VM %s: vCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.RAMBytes <= 0 {
				t.Fatalf("VM %s: RAM = %d bytes, want > 0", vm.Name, vm.RAMBytes)
			}
			if vm.StorageBytes < 0 {
				t.Fatalf("VM %s: storage = %d, want >= 0", vm.Name, vm.StorageBytes)
			}
		}
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name >= vms[i].Name {
				t.Fatalf("VMs not sorted by name: %q before %q", vms[i-1].Name, vms[i].Name)
			}
		}
	})
}
