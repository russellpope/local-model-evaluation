package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// runModel boots an in-process vCenter with a deterministic model and runs fn
// against a connected client.
func runModel(t *testing.T, m *simulator.Model, fn func(ctx context.Context, c *vim25.Client) error) {
	t.Helper()
	if err := m.Run(fn); err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}

func TestFetchVMs(t *testing.T) {
	m := simulator.VPX()
	m.Cluster = 0
	m.Host = 1
	m.Machine = 3
	m.Portgroup = 1

	runModel(t, m, func(ctx context.Context, c *vim25.Client) error {
		vms, err := FetchVMs(ctx, c)
		if err != nil {
			return fmt.Errorf("FetchVMs: %w", err)
		}
		if len(vms) != 3 {
			return fmt.Errorf("FetchVMs returned %d VMs, want 3", len(vms))
		}
		for i, vm := range vms {
			if vm.Name == "" {
				return fmt.Errorf("VM %d has empty name", i)
			}
			if vm.NumCPU <= 0 {
				return fmt.Errorf("VM %q: NumCPU = %d, want > 0", vm.Name, vm.NumCPU)
			}
			if vm.MemoryMB <= 0 {
				return fmt.Errorf("VM %q: MemoryMB = %d, want > 0", vm.Name, vm.MemoryMB)
			}
			if vm.Committed < 0 {
				return fmt.Errorf("VM %q: Committed = %d, want >= 0", vm.Name, vm.Committed)
			}
		}
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				return fmt.Errorf("VMs not sorted: %q after %q", vms[i].Name, vms[i-1].Name)
			}
		}
		return nil
	})
}
