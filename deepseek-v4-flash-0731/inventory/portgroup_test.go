package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// TestVMsOnPortGroup configures a known model: all VMs hang off the first
// distributed port group (DC0_DVPG0), the second has none.
func TestVMsOnPortGroup(t *testing.T) {
	const machineCount = 4

	model := simulator.VPX()
	model.Host = 0 // no standalone host; VMs land in the single cluster pool
	model.Machine = machineCount
	model.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := VMsOnPortGroup(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("VMsOnPortGroup(DC0_DVPG0): %v", err)
		}
		if len(vms) != machineCount {
			t.Fatalf("VMsOnPortGroup(DC0_DVPG0) returned %d VMs, want %d", len(vms), machineCount)
		}
		for i, vm := range vms {
			if vm.Name == "" {
				t.Errorf("vm[%d] has empty name", i)
			}
			if vm.CPUs <= 0 || vm.RAMMB <= 0 {
				t.Errorf("vm %q missing hardware: vcpu=%d ramMB=%d", vm.Name, vm.CPUs, vm.RAMMB)
			}
			if i > 0 && vms[i].Name < vms[i-1].Name {
				t.Errorf("result not sorted by name: %q before %q", vms[i].Name, vms[i-1].Name)
			}
		}

		none, err := VMsOnPortGroup(ctx, c, "DC0_DVPG1")
		if err != nil {
			t.Fatalf("VMsOnPortGroup(DC0_DVPG1): %v", err)
		}
		if len(none) != 0 {
			t.Fatalf("VMsOnPortGroup(DC0_DVPG1) returned %d VMs, want 0", len(none))
		}

		if _, err := VMsOnPortGroup(ctx, c, "no-such-portgroup"); err == nil {
			t.Fatal("expected error for unknown port group")
		}
	}, model)
}
