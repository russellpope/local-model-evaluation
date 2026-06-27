package inventory

import (
	"context"
	"strconv"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vim25 "github.com/vmware/govmomi/vim25"
)

func TestListSwitches_Simulator(t *testing.T) {
	model := simulator.VPX()

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("ListSwitches returned no port groups")
		}

		for _, s := range switches {
			if s.Switch == "" {
				t.Error("ListSwitches: empty switch name")
			}
			switch s.SwitchType {
			case "standard", "distributed":
				// valid
			default:
				t.Errorf("ListSwitches %q/%q: SWITCH TYPE = %q, want standard|distributed", s.Switch, s.PortGroup, s.SwitchType)
			}

			if s.PortGroup == "" {
				t.Error("ListSwitches: empty port group name")
			}

			// VLAN values should either be parseable as a single int or match known patterns.
			if s.VLAN != "" {
				if _, err := strconv.Atoi(s.VLAN); err != nil {
					// Could be "1-4094" (trunk range) or PVLAN format — those are valid too.
					validRange := false
					for i := 0; i < len(s.VLAN)-4; i++ {
						if s.VLAN[i] == '-' {
							if _, err1 := strconv.Atoi(s.VLAN[:i]); err1 == nil {
								if _, err2 := strconv.Atoi(s.VLAN[i+1:]); err2 == nil {
									validRange = true
									break
								}
							}
						}
					}
					if !validRange && len(s.VLAN) > 5 && s.VLAN[:5] != "pvlan" {
						t.Errorf("ListSwitches %q/%q: VLAN = %q, cannot parse as int or range", s.Switch, s.PortGroup, s.VLAN)
					}
				}
			}

			if s.UsedPorts > s.TotalPorts {
				t.Errorf("ListSwitches %q/%q: used (%d) > total (%d)", s.Switch, s.PortGroup, s.UsedPorts, s.TotalPorts)
			}

			switch s.LACP {
			case "enabled", "disabled", "N/A":
				// valid
			default:
				t.Errorf("ListSwitches %q/%q: LACP = %q, want enabled|disabled|N/A", s.Switch, s.PortGroup, s.LACP)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
