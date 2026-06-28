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
	model.Machine = 4 // at least a couple of VMs per RP.

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("no port groups in simulator")
		}

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

		matched, err := ListVMsByPortGroup(ctx, c, targetPG)
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", targetPG, err)
		}

		if len(matched) == 0 {
			t.Fatalf("ListVMsByPortGroup(%q): expected at least one VM connected to a port group in the VPX simulator, got zero results", targetPG)
		}

		for _, v := range matched {
			if !strings.HasPrefix(v.Name, "VPX") && !strings.Contains(v.Name, "_VM") {
				t.Errorf("ListVMsByPortGroup: unexpected VM name %q (want VPX* or *_VM*)", v.Name)
			}
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
