package inventory

import (
	"context"
	"strconv"
	"testing"
)

// findSwitchRows returns the rows for the named switch.
func findSwitchRows(t *testing.T, rows []SwitchInfo, name string) []SwitchInfo {
	t.Helper()
	var out []SwitchInfo
	for _, r := range rows {
		if r.Switch == name {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no switch rows for %q in %+v", name, rows)
	}
	return out
}

func TestListSwitches(t *testing.T) {
	ctx := context.Background()
	c, done := newTestClient(t)
	defer done()

	switches, err := ListSwitches(ctx, c)
	if err != nil {
		t.Fatalf("ListSwitches() error = %v", err)
	}
	if len(switches) < 2 {
		t.Fatalf("ListSwitches() returned %d rows, want at least one standard and one distributed switch", len(switches))
	}

	validLACP := map[string]bool{"enabled": true, "disabled": true, "N/A": true}

	for i, sw := range switches {
		if i > 0 {
			prev := switches[i-1]
			if prev.Switch > sw.Switch || (prev.Switch == sw.Switch && prev.PortGroup >= sw.PortGroup) {
				t.Errorf("ListSwitches() not sorted at index %d: %v before %v", i, prev, sw)
			}
		}
		if !validLACP[sw.LACP] {
			t.Errorf("switch %q port group %q: LACP = %q, want enabled/disabled/N/A", sw.Switch, sw.PortGroup, sw.LACP)
		}
		if sw.SwitchType != typeStandard && sw.SwitchType != typeDistributed {
			t.Errorf("switch %q: SWITCH TYPE = %q, want standard or distributed", sw.Switch, sw.SwitchType)
		}
		if sw.UsedPorts > sw.TotalPorts {
			t.Errorf("switch %q: used ports %d > total ports %d", sw.Switch, sw.UsedPorts, sw.TotalPorts)
		}
		if _, err := strconv.Atoi(sw.VLAN); sw.VLAN != "-" && sw.VLAN != "unknown" && !stringsHasPrefix(sw.VLAN, "trunk") && !stringsHasPrefix(sw.VLAN, "pvlan") && err != nil {
			t.Errorf("switch %q port group %q: VLAN = %q does not parse", sw.Switch, sw.PortGroup, sw.VLAN)
		}
	}

	// Standard switch expectations from the simulator host model.
	std := findSwitchRows(t, switches, "vSwitch0")
	names := map[string]bool{}
	for _, r := range std {
		names[r.PortGroup] = true
		if r.LACP != "N/A" {
			t.Errorf("standard vSwitch LACP must be N/A, got %q", r.LACP)
		}
		if r.Uplinks == "" || r.Uplinks == "-" {
			t.Errorf("standard vSwitch uplinks not reported for port group %q", r.PortGroup)
		}
	}
	for _, want := range []string{"VM Network", "Management Network"} {
		if !names[want] {
			t.Errorf("vSwitch0 missing standard port group %q; got %v", want, names)
		}
	}
	if std[0].TotalPorts != 1536 || std[0].UsedPorts != 6 {
		t.Errorf("vSwitch0 ports = %d/%d used, want 1536 total with 6 used (1536 available)", std[0].UsedPorts, std[0].TotalPorts)
	}

	// Distributed switch expectations from the simulator model.
	dist := findSwitchRows(t, switches, "DVS0")
	pgs := map[string]bool{}
	for _, r := range dist {
		pgs[r.PortGroup] = true
		if r.SwitchType != typeDistributed {
			t.Errorf("DVS0 row %q has type %q, want distributed", r.PortGroup, r.SwitchType)
		}
		if r.LACP == "N/A" {
			t.Errorf("distributed switch LACP must be enabled/disabled, got N/A")
		}
	}
	for _, want := range []string{"DC0_DVPG0", "DC0_DVPG1"} {
		if !pgs[want] {
			t.Errorf("DVS0 missing distributed port group %q; got %v", want, pgs)
		}
	}
}

func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func TestListPortGroupVMs(t *testing.T) {
	ctx := context.Background()
	c, done := newTestClient(t)
	defer done()

	// Every simulated VM's NIC backs onto the first distributed port group,
	// so the lookup must return exactly those VMs.
	want := []string{"DC0_C0_RP1_VM0", "DC0_C0_RP1_VM1", "DC0_C0_RP1_VM2"}
	got, err := ListPortGroupVMs(ctx, c, "DC0_DVPG0")
	if err != nil {
		t.Fatalf("ListPortGroupVMs(DC0_DVPG0) error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ListPortGroupVMs(DC0_DVPG0) returned %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("ListPortGroupVMs(DC0_DVPG0)[%d] = %q, want %q", i, got[i].Name, want[i])
		}
	}

	// A distributed port group with nothing attached returns an empty list,
	// not an error.
	empty, err := ListPortGroupVMs(ctx, c, "DC0_DVPG1")
	if err != nil {
		t.Fatalf("ListPortGroupVMs(DC0_DVPG1) error = %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ListPortGroupVMs(DC0_DVPG1) returned %v, want empty", empty)
	}

	// A standard port group lookup works through the same code path;
	// no VMs are attached to it in this model, so expect an empty list.
	std, err := ListPortGroupVMs(ctx, c, "VM Network")
	if err != nil {
		t.Fatalf("ListPortGroupVMs(VM Network) error = %v", err)
	}
	if len(std) != 0 {
		t.Errorf("ListPortGroupVMs(VM Network) returned %v, want empty", std)
	}

	// An unknown port group is a hard error with an actionable message.
	if _, err := ListPortGroupVMs(ctx, c, "no-such-portgroup"); err == nil {
		t.Error("ListPortGroupVMs(no-such-portgroup): expected error, got nil")
	}
}
