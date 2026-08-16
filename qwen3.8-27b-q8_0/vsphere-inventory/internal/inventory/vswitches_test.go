package inventory

import (
	"context"
	"regexp"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

var vlanPattern = regexp.MustCompile(`^(\d+(-\d+)?)(,\d+(-\d+)?)*$|^(trunk|unknown|pvlan \d+)$`)

func TestListSwitches(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		if len(switches) < 1 {
			t.Fatalf("ListSwitches returned %d switches, want at least 1", len(switches))
		}

		sawDistributed := false
		for i := 1; i < len(switches); i++ {
			if switches[i-1].Name >= switches[i].Name {
				t.Errorf("switches not sorted: %q before %q", switches[i-1].Name, switches[i].Name)
			}
		}

		for _, s := range switches {
			if s.Name == "" {
				t.Errorf("switch with empty name")
			}
			switch s.Kind {
			case SwitchStandard, SwitchDistributed:
			default:
				t.Errorf("switch %s: kind %q, want standard or distributed", s.Name, s.Kind)
			}
			if s.Kind == SwitchDistributed {
				sawDistributed = true
			}
			if len(s.PortGroups) == 0 {
				t.Errorf("switch %s: no port group rows", s.Name)
				continue
			}

			for _, pg := range s.PortGroups {
				if pg.Name == "" {
					t.Errorf("switch %s: port group with empty name", s.Name)
				}
				if !vlanPattern.MatchString(pg.Vlan) {
					t.Errorf("switch %s, port group %s: vlan %q does not parse", s.Name, pg.Name, pg.Vlan)
				}
				if pg.TotalPorts < 0 {
					t.Errorf("switch %s, port group %s: total ports = %d, want >= 0", s.Name, pg.Name, pg.TotalPorts)
				}
				if pg.UsedPorts < 0 || pg.UsedPorts > pg.TotalPorts {
					t.Errorf("switch %s, port group %s: used %d outside [0, %d]", s.Name, pg.Name, pg.UsedPorts, pg.TotalPorts)
				}
				switch pg.LACP {
				case LACPEnabled, LACPDisabled, LACPNA:
				default:
					t.Errorf("switch %s, port group %s: lacp %q, want enabled/disabled/N-A", s.Name, pg.Name, pg.LACP)
				}
			}
		}

		// The model creates one distributed switch with port groups.
		if !sawDistributed {
			t.Error("expected at least one distributed switch from the model")
		}
	}, smallModel())
}
