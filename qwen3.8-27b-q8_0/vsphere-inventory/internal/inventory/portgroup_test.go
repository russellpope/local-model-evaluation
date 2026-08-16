package inventory

import (
	"context"
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// The model attaches every VM NIC to the first distributed port group
// (DC0_DVPG0), so the lookup must return exactly the full VM set for that
// port group and fail for a port group that does not exist.
func TestVMsInPortgroup(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		all, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(all) != 3 {
			t.Fatalf("model sanity: got %d VMs, want 3", len(all))
		}

		got, err := VMsInPortgroup(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("VMsInPortgroup(DC0_DVPG0): %v", err)
		}

		if len(got) != len(all) {
			t.Fatalf("VMsInPortgroup returned %d VMs, want %d", len(got), len(all))
		}

		wantNames := make([]string, 0, len(all))
		for _, vm := range all {
			wantNames = append(wantNames, vm.Name)
		}
		sort.Strings(wantNames)

		gotNames := make([]string, 0, len(got))
		for _, vm := range got {
			gotNames = append(gotNames, vm.Name)
		}
		sort.Strings(gotNames)

		for i := range wantNames {
			if gotNames[i] != wantNames[i] {
				t.Errorf("VM %d in port group = %q, want %q", i, gotNames[i], wantNames[i])
			}
		}

		if _, err := VMsInPortgroup(ctx, c, "DC0_DVPG1"); err == nil {
			t.Error("VMsInPortgroup(DC0_DVPG1): expected an error for a missing port group, got nil")
		}
	}, smallModel())
}
