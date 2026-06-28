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

		// Results must be sorted by name.
		for i := 1; i < len(matched); i++ {
			if matched[i-1].Name > matched[i].Name {
				t.Errorf("ListVMsByPortGroup: not sorted by name at index %d (%q > %q)", i, matched[i-1].Name, matched[i].Name)
			}
		}

		// Querying a port group name that no VM connects to must return zero results
		// (not leftover state from a previous query).
		nonExistent, err := ListVMsByPortGroup(ctx, c, "THIS_PORT_GROUP_DOES_NOT_EXIST_XXXXX")
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(non-existent): %v", err)
		}
		if len(nonExistent) != 0 {
			t.Errorf("ListVMsByPortGroup(non-existent): expected 0 VMs, got %d", len(nonExistent))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}

// TestListVMsByPortGroup_DistributedPG_ExactSet verifies that ListVMsByPortGroup
// returns the exact set of VMs connected to a distributed port group. The
// expected set is derived at runtime from ListVMs so the test is immune to
// model.Machine drift. Every returned VM must exist in the full inventory,
// names must be unique, and each returned VM's vCPU/RAM must match the
// inventory record (N1 identity-consistency).
func TestListVMsByPortGroup_DistributedPG_ExactSet(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 8

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		// Find a distributed port group.
		var dvPG string
		for _, s := range switches {
			if s.SwitchType == "distributed" && s.PortGroup != "" {
				dvPG = s.PortGroup
				break
			}
		}
		if dvPG == "" {
			t.Fatal("no distributed port group found in simulator")
		}

		// Build expected set from the full inventory.
		allVMs, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		expectedByName := make(map[string]VMInfo, len(allVMs))
		for _, v := range allVMs {
			expectedByName[v.Name] = v
		}

		// Get actual set from ListVMsByPortGroup.
		actual, err := ListVMsByPortGroup(ctx, c, dvPG)
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", dvPG, err)
		}

		if len(actual) == 0 {
			t.Fatalf("ListVMsByPortGroup(%q): expected non-empty set, got zero", dvPG)
		}

		// No extras: every actual VM must exist in the full inventory.
		for _, v := range actual {
			if _, ok := expectedByName[v.Name]; !ok {
				t.Errorf("ListVMsByPortGroup(%q): returned VM %q not found in inventory", dvPG, v.Name)
			}
		}

		// No missing: every VM in the inventory must appear in the result
		// (VPX simulator attaches all VMs to the distributed PG).
		for name := range expectedByName {
			found := false
			for _, v := range actual {
				if v.Name == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ListVMsByPortGroup(%q): VM %q in inventory but not returned", dvPG, name)
			}
		}

		// No duplicates in actual results.
		seen := map[string]bool{}
		for _, v := range actual {
			if v.Name == "" {
				t.Errorf("ListVMsByPortGroup(%q): empty VM name in results", dvPG)
				continue
			}
			if seen[v.Name] {
				t.Errorf("ListVMsByPortGroup(%q): duplicate VM name %q", dvPG, v.Name)
			}
			seen[v.Name] = true
		}

		// N1 identity-consistency: each actual VM's vCPU and RAM must match
		// the corresponding ListVMs record.
		for _, v := range actual {
			orig, ok := expectedByName[v.Name]
			if !ok {
				continue // already reported above
			}
			if v.VCPUs != orig.VCPUs {
				t.Errorf("ListVMsByPortGroup(%q): VM %q vCPUs mismatch: got %d, expected %d", dvPG, v.Name, v.VCPUs, orig.VCPUs)
			}
			if v.MemoryMB != orig.MemoryMB {
				t.Errorf("ListVMsByPortGroup(%q): VM %q MemoryMB mismatch: got %d, expected %d", dvPG, v.Name, v.MemoryMB, orig.MemoryMB)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}

// TestListVMsByPortGroup_StandardPG_Empty verifies that a standard port group
// with no VMs returns zero results. On the VPX simulator, VMs attach only to
// the distributed port group, so standard PGs are always empty — this is
// correct behavior and the test asserts it explicitly.
func TestListVMsByPortGroup_StandardPG_Empty(t *testing.T) {
	model := simulator.VPX()

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		// Find any standard port group.
		var stdPG string
		for _, s := range switches {
			if s.SwitchType == "standard" && s.PortGroup != "" {
				stdPG = s.PortGroup
				break
			}
		}
		if stdPG == "" {
			t.Fatal("no standard port group found in simulator")
		}

		matched, err := ListVMsByPortGroup(ctx, c, stdPG)
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", stdPG, err)
		}

		if len(matched) != 0 {
			t.Errorf("ListVMsByPortGroup(%q): expected 0 VMs, got %d", stdPG, len(matched))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
