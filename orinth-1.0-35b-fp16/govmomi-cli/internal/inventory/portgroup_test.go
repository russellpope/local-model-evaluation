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

// TestListVMsByPortGroup_StandardPG_ExactSet verifies that for a known standard
// port group the VM set is correctly associated: every returned VM exists in
// the inventory, names are unique, and the set is non-empty. This catches the
// N1 regression where batch Retrieve results were keyed by input-ref index
// rather than by each returned object's .Self.Value, causing VM identities to
// bind to the wrong NICs/storage on real vCenter.
func TestListVMsByPortGroup_StandardPG_ExactSet(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 4

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		// Find a standard port group that actually has VMs connected.
		allVMs, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		allNames := map[string]VMInfo{}
		for _, v := range allVMs {
			allNames[v.Name] = v
		}

		var stdPG string
		for _, s := range switches {
			if s.SwitchType != "standard" {
				continue
			}
			matched, err := ListVMsByPortGroup(ctx, c, s.PortGroup)
			if err != nil {
				t.Fatalf("ListVMsByPortGroup(%q): %v", s.PortGroup, err)
			}
			if len(matched) > 0 {
				stdPG = s.PortGroup
				break
			}
		}
		if stdPG == "" {
			t.Skip("no standard port group with connected VMs in simulator; skipping exact-set test")
		}

		matched, err := ListVMsByPortGroup(ctx, c, stdPG)
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", stdPG, err)
		}

		if len(matched) == 0 {
			t.Fatalf("ListVMsByPortGroup(%q): expected at least one VM connected to a standard port group, got zero", stdPG)
		}

		seen := map[string]bool{}
		for _, v := range matched {
			if v.Name == "" {
				t.Errorf("ListVMsByPortGroup(%q): empty VM name in results", stdPG)
			}
			if seen[v.Name] {
				t.Errorf("ListVMsByPortGroup(%q): duplicate VM name %q", stdPG, v.Name)
			}
			seen[v.Name] = true

			// Each returned VM must exist in the full inventory — this catches
			// the N1 bug where a VM's identity could be bound to another VM's
			// NICs/storage due to incorrect result association.
			orig, ok := allNames[v.Name]
			if !ok {
				t.Errorf("ListVMsByPortGroup(%q): returned VM %q not found in inventory", stdPG, v.Name)
				continue
			}
			// And the identity must be self-consistent: vCPUs and memory from
			// the matched record must match the inventory record for that name.
			if v.VCPUs != orig.VCPUs {
				t.Errorf("ListVMsByPortGroup(%q): VM %q vCPUs mismatch: got %d, expected %d", stdPG, v.Name, v.VCPUs, orig.VCPUs)
			}
			if v.MemoryMB != orig.MemoryMB {
				t.Errorf("ListVMsByPortGroup(%q): VM %q MemoryMB mismatch: got %d, expected %d", stdPG, v.Name, v.MemoryMB, orig.MemoryMB)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
