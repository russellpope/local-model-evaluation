package inventory

import (
	"context"
	"sort"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

func TestVMsInPortgroup(t *testing.T) {
	withClient(t, func(ctx context.Context, c *vim25.Client) {
		// The deterministic model names its single distributed port group
		// DC0_DVPG0; verify it really appears as a distributed port group.
		const dvpg = "DC0_DVPG0"
		ss, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		found := false
		for _, s := range ss {
			if s.SwitchType == "distributed" && s.Portgroup == dvpg {
				found = true
			}
		}
		if !found {
			t.Fatalf("distributed port group %s not found in switch listing: %+v", dvpg, ss)
		}

		// All three model VMs have their NIC backed by DC0_DVPG0.
		vms, err := VMsInPortgroup(ctx, c, dvpg)
		if err != nil {
			t.Fatalf("VMsInPortgroup(%q): %v", dvpg, err)
		}
		var got []string
		for _, vm := range vms {
			got = append(got, vm.Name)
		}
		sort.Strings(got)
		want := []string{"DC0_H0_VM0", "DC0_H0_VM1", "DC0_H0_VM2"}
		if len(got) != len(want) {
			t.Fatalf("VMsInPortgroup(%q) = %v, want exactly %v", dvpg, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("VMsInPortgroup(%q) = %v, want exactly %v", dvpg, got, want)
			}
		}

		// The standard "VM Network" port group exists but has no VMs attached
		// in this model, so the lookup must return an empty set, not an error.
		empty, err := VMsInPortgroup(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("VMsInPortgroup(\"VM Network\"): %v", err)
		}
		if len(empty) != 0 {
			t.Fatalf("VMsInPortgroup(\"VM Network\") = %d VMs, want 0", len(empty))
		}

		// A nonexistent port group must produce an error.
		if _, err := VMsInPortgroup(ctx, c, "no-such-portgroup"); err == nil {
			t.Fatalf("VMsInPortgroup(\"no-such-portgroup\"): expected error, got nil")
		}
	})
}
