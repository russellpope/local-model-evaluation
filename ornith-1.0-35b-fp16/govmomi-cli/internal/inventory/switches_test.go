package inventory

import (
	"context"
	"strconv"
	"testing"

	vim25 "github.com/vmware/govmomi/vim25"
)

func TestListSwitches_Simulator(t *testing.T) {
	runWithSimulator(t, nil, func(ctx context.Context, c *vim25.Client) error {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("ListSwitches returned no port groups")
		}

		hasStandard := false
		hasDVS := false
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

			// Standard port groups must have a host associated (per-host inventory).
			if s.SwitchType == "standard" && s.Host == "" {
				t.Errorf("ListSwitches standard %q/%q: HOST is empty, expected ESXi host name", s.Switch, s.PortGroup)
			}

			// VLAN values should either be parseable as a single int or match known patterns.
			if s.VLAN != "" {
				if _, err := strconv.Atoi(s.VLAN); err != nil {
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

			if s.SwitchType == "standard" {
				hasStandard = true
				// Standard switches backed by HostVirtualSwitch must expose a strictly
				// lower UsedPorts than TotalPorts (NumPortsAvailable is non-zero on real
				// vSwitches, including against the VPX simulator).
				if s.UsedPorts >= s.TotalPorts {
					t.Errorf("ListSwitches standard %q/%q: USED (%d) must be < TOTAL (%d), got Used==Total",
						s.Switch, s.PortGroup, s.UsedPorts, s.TotalPorts)
				}
				if !s.UsedPortsValid {
					t.Errorf("ListSwitches standard %q/%q: UsedPortsValid must be true", s.Switch, s.PortGroup)
				}
			}

			if s.SwitchType == "distributed" {
				hasDVS = true
				// DVS port groups cannot derive UsedPorts from the API — the column
				// must be marked invalid so callers render N/A rather than Total-0.
				if s.UsedPortsValid {
					t.Errorf("ListSwitches distributed %q/%q: UsedPortsValid must be false (cannot derive used ports from API)", s.Switch, s.PortGroup)
				}
			}
		}

		if !hasStandard {
			t.Error("ListSwitches: no standard switch port group found (C3 fix should surface them)")
		}
		if !hasDVS {
			t.Error("ListSwitches: no distributed switch port group found")
		}

		return nil
	})
}
