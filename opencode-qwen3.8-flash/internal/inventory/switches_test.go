package inventory

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// validVLAN reports whether a VLAN cell is a number/range/comma-list, an
// annotated trunk / private-VLAN form, or the documented "unknown" degrade.
func validVLAN(v string) bool {
	if v == unknownField {
		return true
	}
	if strings.HasSuffix(v, "(trunk)") {
		v = strings.TrimSpace(strings.TrimSuffix(v, "(trunk)"))
		return rangeListRe.MatchString(v)
	}
	if strings.HasPrefix(v, "private-vlan") {
		return true
	}
	return rangeListRe.MatchString(v)
}

var rangeListRe = regexp.MustCompile(`^[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*$`)

func TestFetchSwitches(t *testing.T) {
	m := simulator.VPX()
	m.Cluster = 0
	m.Host = 1
	m.Machine = 1
	m.Portgroup = 2
	m.Datastore = 1

	runModel(t, m, func(ctx context.Context, c *vim25.Client) error {
		switches, err := FetchSwitches(ctx, c)
		if err != nil {
			return fmt.Errorf("FetchSwitches: %w", err)
		}
		if len(switches) == 0 {
			return fmt.Errorf("FetchSwitches returned no switches")
		}
		var standard, distributed int
		for _, sw := range switches {
			if sw.Switch == "" || sw.Portgroup == "" {
				return fmt.Errorf("switch row has empty name/portgroup: %+v", sw)
			}
			switch sw.SwitchType {
			case SwitchStandard:
				standard++
				if sw.LACP != LACPNABlank {
					return fmt.Errorf("standard switch %q: LACP = %q, want %q", sw.Switch, sw.LACP, LACPNABlank)
				}
			case SwitchDistributed:
				distributed++
				if sw.LACP != LACPEnabled && sw.LACP != LACPDisabled {
					return fmt.Errorf("distributed switch %q: LACP = %q, want enabled/disabled", sw.Switch, sw.LACP)
				}
			default:
				return fmt.Errorf("switch %q: unknown SwitchType %q", sw.Switch, sw.SwitchType)
			}
			if !validVLAN(sw.VLAN) {
				return fmt.Errorf("portgroup %q: VLAN %q does not parse", sw.Portgroup, sw.VLAN)
			}
			if sw.Used > sw.Ports {
				return fmt.Errorf("portgroup %q: Used(%d) > Ports(%d)", sw.Portgroup, sw.Used, sw.Ports)
			}
			if sw.Uplinks == "" {
				return fmt.Errorf("portgroup %q: empty Uplinks", sw.Portgroup)
			}
			if sw.SwitchType == SwitchStandard {
				if _, err := strconv.Atoi(firstVLAN(sw.VLAN)); err != nil {
					return fmt.Errorf("portgroup %q: VLAN %q: %w", sw.Portgroup, sw.VLAN, err)
				}
			}
		}
		if standard == 0 {
			return fmt.Errorf("no standard switch rows in %+v", switches)
		}
		if distributed == 0 {
			return fmt.Errorf("no distributed switch rows in %+v", switches)
		}
		for i := 1; i < len(switches); i++ {
			prev, cur := switches[i-1], switches[i]
			if prev.Switch > cur.Switch || (prev.Switch == cur.Switch && prev.Portgroup > cur.Portgroup) {
				return fmt.Errorf("switch rows not sorted: %+v before %+v", prev, cur)
			}
		}
		return nil
	})
}

func firstVLAN(v string) string {
	v = strings.SplitN(v, " ", 2)[0]
	return strings.SplitN(v, "-", 2)[0]
}
