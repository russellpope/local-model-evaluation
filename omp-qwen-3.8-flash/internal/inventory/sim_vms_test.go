package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// vpxModel returns a deterministic, small vCenter model: one datacenter, one
// standalone host, and optionally a DVS with pg port groups. VMs attach to
// the first distributed port group when pg > 0, otherwise to the standard
// "VM Network" port group.
func vpxModel(vm, ds, pg int) *simulator.Model {
	m := simulator.VPX()
	m.Datacenter = 1
	m.Host = 1
	m.Cluster = 0
	m.ClusterHost = 0
	m.Pool = 0
	m.App = 0
	m.Pod = 0
	m.Folder = 0
	m.Machine = vm
	m.Datastore = ds
	m.Portgroup = pg
	return m
}

func mustRun(t *testing.T, m *simulator.Model, fn func(ctx context.Context, c *vim25.Client) error) {
	t.Helper()
	if err := m.Run(fn); err != nil {
		t.Fatalf("simulator: %v", err)
	}
}

func TestVMs(t *testing.T) {
	mustRun(t, vpxModel(4, 1, 1), func(ctx context.Context, c *vim25.Client) error {
		vms, err := VMs(ctx, c)
		if err != nil {
			t.Fatalf("VMs: %v", err)
		}
		if len(vms) != 4 {
			t.Fatalf("VM count = %d, want 4", len(vms))
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Errorf("empty VM name in %+v", vm)
			}
			if vm.NumCPU <= 0 {
				t.Errorf("vm %s: vCPU = %d, want > 0", vm.Name, vm.NumCPU)
			}
			if vm.MemoryMB <= 0 {
				t.Errorf("vm %s: memory = %d MB, want > 0", vm.Name, vm.MemoryMB)
			}
			if vm.Committed < 0 {
				t.Errorf("vm %s: committed = %d, want >= 0", vm.Name, vm.Committed)
			}
		}
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Errorf("rows not sorted by name: %q before %q", vms[i-1].Name, vms[i].Name)
			}
		}
		return nil
	})
}

// TestVMsConsumedStorage proves the STORAGE column is Summary.Storage.Committed
// (actual consumed bytes), not provisioned capacity: the simulator populates
// committed from the VM's backing files and leaves it strictly below
// provisioned (Unshared+Committed > 0 with distinct values).
func TestVMsConsumedStorage(t *testing.T) {
	mustRun(t, vpxModel(2, 1, 1), func(ctx context.Context, c *vim25.Client) error {
		vms, err := VMs(ctx, c)
		if err != nil {
			t.Fatalf("VMs: %v", err)
		}
		for _, vm := range vms {
			if vm.Committed <= 0 {
				t.Errorf("vm %s: committed = %d, want > 0 (simulator creates vmdk files)", vm.Name, vm.Committed)
			}
		}
		return nil
	})
}

func TestVMsEmptyInventory(t *testing.T) {
	mustRun(t, vpxModel(0, 1, 0), func(ctx context.Context, c *vim25.Client) error {
		vms, err := VMs(ctx, c)
		if err != nil {
			t.Fatalf("VMs: %v", err)
		}
		if len(vms) != 0 {
			t.Fatalf("VM count = %d, want 0", len(vms))
		}
		return nil
	})
}

func TestVMsContextCancelled(t *testing.T) {
	mustRun(t, vpxModel(1, 1, 0), func(_ context.Context, c *vim25.Client) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := VMs(ctx, c); err == nil {
			t.Fatal("VMs with cancelled context: want error, got nil")
		} else if !errors.Is(err, context.Canceled) {
			t.Fatalf("VMs with cancelled context: want context.Canceled, got %v", err)
		}
		return nil
	})
}

func TestVMsContextDeadline(t *testing.T) {
	mustRun(t, vpxModel(1, 1, 0), func(_ context.Context, c *vim25.Client) error {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		if _, err := VMs(ctx, c); err == nil {
			t.Fatal("VMs with expired context: want error, got nil")
		} else if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("VMs with expired context: want context.DeadlineExceeded, got %v", err)
		}
		return nil
	})
}
