package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vim25 "github.com/vmware/govmomi/vim25"
)

func TestListVMs_Simulator(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 4 // VMs per resource pool; with default ClusterHost=3 that is at least 4 total.

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}

		if len(vms) == 0 {
			t.Fatal("ListVMs returned no VMs")
		}

		for _, v := range vms {
			if v.Name == "" {
				t.Error("ListVMs: empty VM name")
			}
			if v.VCPUs <= 0 {
				t.Errorf("ListVMs %q: VCPUs = %d, want > 0", v.Name, v.VCPUs)
			}
			if v.MemoryMB <= 0 {
				t.Errorf("ListVMs %q: MemoryMB = %d, want > 0", v.Name, v.MemoryMB)
			}
			if v.StorageB < 0 {
				t.Errorf("ListVMs %q: StorageB = %d, want >= 0", v.Name, v.StorageB)
			}
		}

		// Verify sort order.
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Errorf("ListVMs: not sorted by name at index %d (%q > %q)", i, vms[i-1].Name, vms[i].Name)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
