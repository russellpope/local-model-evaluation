package inventory

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

func validVLAN(v string) bool {
	switch v {
	case "-", "trunk", "unknown":
		return true
	}
	for _, part := range strings.Split(v, ",") {
		if r := strings.SplitN(part, "-", 2); len(r) == 2 {
			if _, err := strconv.Atoi(r[0]); err != nil {
				return false
			}
			if _, err := strconv.Atoi(r[1]); err != nil {
				return false
			}
		} else if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func TestListSwitches(t *testing.T) {
	withClient(t, func(ctx context.Context, c *vim25.Client) {
		ss, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(ss) == 0 {
			t.Fatalf("expected at least one switch, got none")
		}

		knownLACP := map[string]bool{"enabled": true, "disabled": true, "N/A": true}
		seenStandard, seenDistributed := false, false

		for _, s := range ss {
			if s.Switch == "" {
				t.Fatalf("switch row with empty name: %+v", s)
			}
			switch s.SwitchType {
			case "standard":
				seenStandard = true
			case "distributed":
				seenDistributed = true
			default:
				t.Fatalf("switch %s: switch type %q, want standard or distributed", s.Switch, s.SwitchType)
			}
			if !knownLACP[s.LACP] {
				t.Fatalf("switch %s port group %s: LACP %q, want enabled/disabled/N/A", s.Switch, s.Portgroup, s.LACP)
			}
			if !validVLAN(s.VLAN) {
				t.Fatalf("switch %s port group %s: VLAN %q does not parse", s.Switch, s.Portgroup, s.VLAN)
			}
			if s.Used > s.Ports {
				t.Fatalf("switch %s port group %s: used ports %d > total ports %d", s.Switch, s.Portgroup, s.Used, s.Ports)
			}
			if s.SwitchType == "standard" && s.LACP != "N/A" {
				t.Fatalf("standard switch %s: LACP %q, want N/A", s.Switch, s.LACP)
			}
		}

		if !seenStandard {
			t.Fatalf("expected at least one standard switch, rows: %+v", ss)
		}
		if !seenDistributed {
			t.Fatalf("expected at least one distributed switch, rows: %+v", ss)
		}
	})
}
