package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// TestListVMsAgainstSimulator configures a known VM count and asserts the
// exact number comes back, with every field populated sensibly.
func TestListVMsAgainstSimulator(t *testing.T) {
	const wantCount = 8

	model := simulator.VPX()
	model.Host = 0 // no standalone host; VMs land in the single cluster pool
	model.Machine = wantCount

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) != wantCount {
			t.Fatalf("ListVMs returned %d VMs, want %d", len(vms), wantCount)
		}
		prev := ""
		for i, vm := range vms {
			if vm.Name == "" {
				t.Errorf("vm[%d] has empty name", i)
			}
			if vm.CPUs <= 0 {
				t.Errorf("vm %q has vCPU %d, want > 0", vm.Name, vm.CPUs)
			}
			if vm.RAMMB <= 0 {
				t.Errorf("vm %q has RAM %d MB, want > 0", vm.Name, vm.RAMMB)
			}
			if vm.CommittedBytes < 0 {
				t.Errorf("vm %q has negative committed storage %d", vm.Name, vm.CommittedBytes)
			}
			if i > 0 && vm.Name < prev {
				t.Errorf("vms not sorted by name: %q after %q", vm.Name, prev)
			}
			prev = vm.Name
		}
	}, model)
}
