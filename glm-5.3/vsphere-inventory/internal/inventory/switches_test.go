package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
)

// TestSwitches covers both switch flavors: the model creates a standard
// vSwitch on every host plus one distributed switch with port groups.
func TestSwitches(t *testing.T) {
	runSimulator(t, func(m *simulator.Model) {
		m.Portgroup = 2
	}, func(ctx context.Context, c *govmomi.Client) {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(switches) == 0 {
			t.Fatal("no switches returned")
		}

		byKind := map[string]int{}
		for _, sw := range switches {
			byKind[sw.Kind]++

			if sw.Name == "" {
				t.Error("switch with empty name")
			}
			if sw.TotalPorts < 0 {
				t.Errorf("%s: negative total ports %d", sw.Name, sw.TotalPorts)
			}
			if sw.UsedPorts < 0 || sw.UsedPorts > sw.TotalPorts {
				t.Errorf("%s: used ports %d outside 0..%d", sw.Name, sw.UsedPorts, sw.TotalPorts)
			}

			switch sw.Kind {
			case SwitchStandard:
				if sw.LACP != LACPNotApplicable {
					t.Errorf("%s: standard switch LACP = %q, want N/A", sw.Name, sw.LACP)
				}
				if sw.TotalPorts == 0 {
					t.Errorf("%s: standard switch reports zero ports", sw.Name)
				}
			case SwitchDistributed:
				if sw.LACP != LACPEnabled && sw.LACP != LACPDisabled {
					t.Errorf("%s: distributed switch LACP = %q, want enabled/disabled", sw.Name, sw.LACP)
				}
				// The simulator creates port groups for the DVS.
				if len(sw.PortGroups) == 0 {
					t.Errorf("%s: distributed switch has no port groups", sw.Name)
				}
			default:
				t.Errorf("switch %q has unexpected kind %q", sw.Name, sw.Kind)
			}

			for _, pg := range sw.PortGroups {
				if pg.Name == "" {
					t.Errorf("%s: port group with empty name", sw.Name)
				}
				if _, err := ParseVLAN(pg.VLAN); err != nil {
					t.Errorf("%s/%s: VLAN %q does not parse: %v", sw.Name, pg.Name, pg.VLAN, err)
				}
			}

			// Port groups sorted by name.
			for i := 1; i < len(sw.PortGroups); i++ {
				if sw.PortGroups[i-1].Name > sw.PortGroups[i].Name {
					t.Fatalf("%s: port groups not sorted: %q before %q",
						sw.Name, sw.PortGroups[i-1].Name, sw.PortGroups[i].Name)
				}
			}
		}

		if byKind[SwitchStandard] == 0 {
			t.Error("no standard switches returned")
		}
		if byKind[SwitchDistributed] == 0 {
			t.Error("no distributed switches returned")
		}
	})
}
