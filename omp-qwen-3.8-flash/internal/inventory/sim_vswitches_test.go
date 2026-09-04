package inventory

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

// TestSwitches validates the combined standard + distributed port group
// listing against the simulator: both switch types appear, VLAN values parse,
// used <= total, and LACP is one of enabled/disabled/N/A (N/A for standard,
// since LACP is distributed-only).
func TestSwitches(t *testing.T) {
	mustRun(t, vpxModel(2, 1, 2), func(ctx context.Context, c *vim25.Client) error {
		pgs, err := Switches(ctx, c)
		if err != nil {
			t.Fatalf("Switches: %v", err)
		}
		if len(pgs) == 0 {
			t.Fatal("Switches returned no rows")
		}

		var sawStandard, sawDistributed bool
		lacpLegal := map[string]bool{"enabled": true, "disabled": true, "N/A": true}
		for _, p := range pgs {
			switch p.SwitchType {
			case SwitchStandard:
				sawStandard = true
				// LACP does not apply to standard vSwitches.
				if p.LACP != LACPNA {
					t.Errorf("standard %s/%s: LACP = %q, want N/A", p.Switch, p.Portgroup, p.LACP)
				}
			case SwitchDistributed:
				sawDistributed = true
				if !lacpLegal[p.LACP] {
					t.Errorf("distributed %s/%s: LACP = %q not in enabled/disabled/N/A", p.Switch, p.Portgroup, p.LACP)
				}
			default:
				t.Errorf("row %s/%s: unknown switch type %q", p.Switch, p.Portgroup, p.SwitchType)
			}
			if p.Switch == "" || p.Portgroup == "" {
				t.Errorf("empty switch/portgroup in %+v", p)
			}
			if p.Uplinks == "" {
				t.Errorf("%s/%s: empty uplinks field", p.Switch, p.Portgroup)
			}
			// VLAN must parse: plain ID, or a labelled range/trunk/private-vlan.
			if !vlanParses(p.VLAN) {
				t.Errorf("%s/%s: VLAN %q does not parse", p.Switch, p.Portgroup, p.VLAN)
			}
			if p.Used > p.Ports {
				t.Errorf("%s/%s: used %d > total %d", p.Switch, p.Portgroup, p.Used, p.Ports)
			}
			if p.Ports < 0 {
				t.Errorf("%s/%s: negative port count %d", p.Switch, p.Portgroup, p.Ports)
			}
		}
		if !sawStandard {
			t.Error("no standard vSwitch rows returned")
		}
		if !sawDistributed {
			t.Error("no distributed port group rows returned")
		}
		for i := 1; i < len(pgs); i++ {
			a, b := pgs[i-1], pgs[i]
			if a.Switch > b.Switch || (a.Switch == b.Switch && a.Portgroup > b.Portgroup) {
				t.Errorf("rows not sorted: %s/%s before %s/%s", a.Switch, a.Portgroup, b.Switch, b.Portgroup)
			}
		}
		return nil
	})
}

// TestSwitchesUplinkPortgroupExcluded ensures the switch's internal uplink
// port group is never listed as a workload port group.
func TestSwitchesUplinkPortgroupExcluded(t *testing.T) {
	mustRun(t, vpxModel(1, 1, 1), func(ctx context.Context, c *vim25.Client) error {
		pgs, err := Switches(ctx, c)
		if err != nil {
			t.Fatalf("Switches: %v", err)
		}
		for _, p := range pgs {
			if strings.Contains(strings.ToUpper(p.Portgroup), "UPLINK") {
				t.Errorf("uplink port group leaked into listing: %+v", p)
			}
		}
		return nil
	})
}

func vlanParses(v string) bool {
	if v == "unknown" {
		return true
	}
	s := v
	for _, label := range []string{"0 (native)", "4095 (trunk)", "private-vlan"} {
		if s == label {
			return true
		}
	}
	if strings.HasPrefix(s, "trunk ") {
		for _, part := range strings.Split(strings.TrimPrefix(s, "trunk "), ",") {
			if rng := strings.SplitN(part, "-", 2); len(rng) == 2 {
				if _, err := strconv.Atoi(rng[0]); err != nil {
					return false
				}
				if _, err := strconv.Atoi(rng[1]); err != nil {
					return false
				}
				continue
			}
			if _, err := strconv.Atoi(part); err != nil {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(s, "private-vlan ") {
		if _, err := strconv.Atoi(strings.TrimPrefix(s, "private-vlan ")); err == nil {
			return true
		}
	}
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	return false
}
