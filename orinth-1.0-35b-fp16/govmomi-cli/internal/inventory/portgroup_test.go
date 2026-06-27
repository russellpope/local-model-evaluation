package inventory

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vim25 "github.com/vmware/govmomi/vim25"
)

func TestListVMsByPortGroup_Simulator(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 2 // at least a couple of VMs per RP.

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		// First, list the switches to discover real port group names in this
		// simulator instance (names vary by version).
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		if len(switches) == 0 {
			t.Skip("no port groups in simulator — skipping --portgroup test")
		}

		// Pick the first unique port group name we find.
		seen := map[string]bool{}
		var pgNames []string
		for _, s := range switches {
			if !seen[s.PortGroup] {
				seen[s.PortGroup] = true
				pgNames = append(pgNames, s.PortGroup)
			}
		}

		if len(pgNames) == 0 {
			t.Fatal("no unique port group names found")
		}

		targetPG := pgNames[0]
		t.Logf("looking for VMs on port group %q (switch=%q)", targetPG, switches[0].Switch)

		matched, err := ListVMsByPortGroup(ctx, c, targetPG)
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", targetPG, err)
		}

		// At minimum the lookup should succeed and return a result that makes sense.
		for _, v := range matched {
			if !strings.HasPrefix(v.Name, "VPX") && !strings.Contains(v.Name, "_VM") {
				// vcsim names VMs starting with "VPX" or "<prefix>_VM<index>". A
				// result whose name doesn't match either pattern is suspicious.
				t.Logf("ListVMsByPortGroup: unexpected VM name %q", v.Name)
			}
		}

		// If the simulator attaches any NIC to this port group, we expect at
		// least one result; otherwise zero results is also valid. Either way no
		// error should be raised and all names must be non-empty.
		for _, v := range matched {
			if v.Name == "" {
				t.Error("ListVMsByPortGroup: empty VM name in results")
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
