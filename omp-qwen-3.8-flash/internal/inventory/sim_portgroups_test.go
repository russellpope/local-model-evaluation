package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

// TestVMsOnDistributedPortgroup configures the model so all VMs attach to
// the first distributed port group, then asserts the lookup returns exactly
// that set and nothing else. vcsim wires every VM's eth0 to "<dc>_DVPG0".
func TestVMsOnDistributedPortgroup(t *testing.T) {
	mustRun(t, vpxModel(3, 1, 2), func(ctx context.Context, c *vim25.Client) error {
		// Discover the real simulator port group name rather than hardcoding.
		pgs, err := Switches(ctx, c)
		if err != nil {
			t.Fatalf("Switches: %v", err)
		}
		want := ""
		for _, p := range pgs {
			if p.SwitchType == SwitchDistributed {
				want = p.Portgroup
				break
			}
		}
		if want == "" {
			t.Fatal("no distributed port group found to test against")
		}

		got, err := VMsOnPortgroup(ctx, c, want)
		if err != nil {
			t.Fatalf("VMsOnPortgroup(%q): %v", want, err)
		}
		if len(got) != 3 {
			t.Fatalf("VMsOnPortgroup(%q) returned %d VMs, want 3: %v", want, len(got), names(got))
		}
		// Every returned VM must report storage and size fields.
		for _, vm := range got {
			if vm.Name == "" || vm.NumCPU <= 0 || vm.MemoryMB <= 0 {
				t.Errorf("incomplete VM row from portgroup lookup: %+v", vm)
			}
		}
		return nil
	})
}

// TestVMsOnStandardPortgroup exercises the standard-path lookup: with no
// distributed port groups configured, VMs land on "VM Network".
func TestVMsOnStandardPortgroup(t *testing.T) {
	mustRun(t, vpxModel(2, 1, 0), func(ctx context.Context, c *vim25.Client) error {
		// Confirm the port group is present in the switch listing first.
		pgs, err := Switches(ctx, c)
		if err != nil {
			t.Fatalf("Switches: %v", err)
		}
		found := false
		for _, p := range pgs {
			if p.Portgroup == "VM Network" {
				found = true
			}
		}
		if !found {
			t.Fatal("VM Network port group missing from simulator inventory")
		}

		got, err := VMsOnPortgroup(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("VMsOnPortgroup(VM Network): %v", err)
		}
		// vcsim attaches default NICs to "VM Network" when no DVPG exists.
		if len(got) == 0 {
			t.Fatalf("VMsOnPortgroup(VM Network) returned no VMs; want the inventory VMs")
		}
		return nil
	})
}

func TestVMsOnPortgroupNotFound(t *testing.T) {
	mustRun(t, vpxModel(1, 1, 1), func(ctx context.Context, c *vim25.Client) error {
		if _, err := VMsOnPortgroup(ctx, c, "definitely-not-a-portgroup"); err == nil {
			t.Fatal("VMsOnPortgroup for missing group: want error, got nil")
		}
		return nil
	})
}

// TestVMsOnPortgroupIsolatesGroups proves the lookup returns only the VMs on
// the named group when a second group also exists (two-DVPG model).
func TestVMsOnPortgroupIsolatesGroups(t *testing.T) {
	mustRun(t, vpxModel(2, 1, 2), func(ctx context.Context, c *vim25.Client) error {
		pgs, err := Switches(ctx, c)
		if err != nil {
			t.Fatalf("Switches: %v", err)
		}
		var dist []string
		seen := map[string]bool{}
		for _, p := range pgs {
			if p.SwitchType == SwitchDistributed && !seen[p.Portgroup] {
				seen[p.Portgroup] = true
				dist = append(dist, p.Portgroup)
			}
		}
		if len(dist) < 2 {
			t.Fatalf("need >=2 distributed port groups, got %v", dist)
		}
		// vcsim attaches every VM to DVPG0; DVPG1 must be empty.
		on0, err := VMsOnPortgroup(ctx, c, dist[0])
		if err != nil {
			t.Fatalf("VMsOnPortgroup(%q): %v", dist[0], err)
		}
		on1, err := VMsOnPortgroup(ctx, c, dist[1])
		if err != nil {
			t.Fatalf("VMsOnPortgroup(%q): %v", dist[1], err)
		}
		if len(on0) != 2 {
			t.Errorf("portgroup %s: %d VMs, want 2", dist[0], len(on0))
		}
		if len(on1) != 0 {
			t.Errorf("portgroup %s: %d VMs, want 0 (no VM attached)", dist[1], len(on1))
		}
		return nil
	})
}

func names(vms []VMInfo) []string {
	out := make([]string, 0, len(vms))
	for _, v := range vms {
		out = append(out, v.Name)
	}
	return out
}
