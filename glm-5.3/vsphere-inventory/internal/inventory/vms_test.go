package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
)

// TestListVMs configures the model with a known machine count and asserts
// the inventory returns exactly the expected VMs with sane values.
func TestListVMs(t *testing.T) {
	const machinesPerPool = 3
	// The default VPX model deploys machines into two resource pools:
	// the standalone host's pool and the cluster's root pool.
	const wantVMs = 2 * machinesPerPool

	runSimulator(t, func(m *simulator.Model) {
		m.Machine = machinesPerPool
	}, func(ctx context.Context, c *govmomi.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) != wantVMs {
			t.Fatalf("got %d VMs, want %d: %+v", len(vms), wantVMs, vms)
		}

		seen := make(map[string]bool, len(vms))
		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("VM with empty name")
			}
			if vm.NumCPU <= 0 {
				t.Errorf("%s: NumCPU = %d, want > 0", vm.Name, vm.NumCPU)
			}
			if vm.MemoryMB <= 0 {
				t.Errorf("%s: MemoryMB = %d, want > 0", vm.Name, vm.MemoryMB)
			}
			if vm.StorageCommittedBytes < 0 {
				t.Errorf("%s: StorageCommittedBytes = %d, want >= 0", vm.Name, vm.StorageCommittedBytes)
			}
			seen[vm.Name] = true
		}

		// The simulator names VMs deterministically; verify a couple of
		// concrete names to make sure the mapping is not bogus.
		for _, name := range []string{"DC0_H0_VM0", "DC0_C0_RP0_VM0"} {
			if !seen[name] {
				t.Errorf("expected VM %q in results, got %v", name, keysOf(seen))
			}
		}

		// Sorted by name.
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Fatalf("VMs not sorted by name: %q before %q", vms[i-1].Name, vms[i].Name)
			}
		}
	})
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
