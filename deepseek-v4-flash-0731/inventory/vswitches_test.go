package inventory

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

func TestListSwitchesAgainstSimulator(t *testing.T) {
	model := simulator.VPX() // default: 1 cluster, 3 hosts (standard vSwitch0), 1 DVS + DVPG

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(switches) == 0 {
			t.Fatal("ListSwitches returned no switches")
		}

		validTypes := map[string]bool{"standard": true, "distributed": true}
		validLACP := map[string]bool{"enabled": true, "disabled": true, "N/A": true}

		seenStandard := false
		seenDistributed := false
		for _, s := range switches {
			if s.PortGroup == "" {
				t.Errorf("switch %q/%q has empty port group name", s.SwitchType, s.Switch)
			}
			if !validTypes[s.SwitchType] {
				t.Errorf("switch %q has invalid type %q", s.Switch, s.SwitchType)
			}
			if !vlanParses(s.Vlan) {
				t.Errorf("row %s/%s: unparseable VLAN %q", s.Switch, s.PortGroup, s.Vlan)
			}
			if s.Used > s.Ports {
				t.Errorf("row %s/%s: used ports %d > total ports %d", s.Switch, s.PortGroup, s.Used, s.Ports)
			}
			if !validLACP[s.LACP] {
				t.Errorf("row %s/%s: invalid LACP value %q", s.Switch, s.PortGroup, s.LACP)
			}
			switch s.SwitchType {
			case "standard":
				seenStandard = true
				if s.LACP != "N/A" {
					t.Errorf("standard switch %q must report LACP N/A, got %q", s.Switch, s.LACP)
				}
			case "distributed":
				seenDistributed = true
			}
		}

		if !seenStandard {
			t.Error("expected at least one standard vSwitch row (simulator host has vSwitch0)")
		}
		if !seenDistributed {
			t.Error("expected at least one distributed switch row (VPX model creates a DVS)")
		}
	}, model)
}

// vlanParses accepts the renderings ListSwitches can emit: a single ID, a
// comma-joined trunk range list ("1-10,20"), or a tagged sentinel.
func vlanParses(v string) bool {
	switch v {
	case "untagged", "trunk", "unknown":
		return true
	}
	if strings.HasPrefix(v, "pvlan-") {
		_, err := strconv.Atoi(strings.TrimPrefix(v, "pvlan-"))
		return err == nil
	}
	if _, err := strconv.Atoi(v); err == nil {
		return true
	}
	for _, part := range strings.Split(v, ",") {
		if _, err := strconv.Atoi(part); err == nil {
			continue
		}
		ends := strings.Split(part, "-")
		if len(ends) != 2 {
			return false
		}
		a, err1 := strconv.Atoi(ends[0])
		b, err2 := strconv.Atoi(ends[1])
		if err1 != nil || err2 != nil || a > b {
			return false
		}
	}
	return true
}

func TestStandardVLAN(t *testing.T) {
	tests := []struct {
		name string
		in   int32
		want string
	}{
		{"vlan twenty", 20, "20"},
		{"zero means default", 0, "0"},
		{"untagged", -1, "untagged"},
		{"trunk", 4095, "trunk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := standardVLAN(tt.in); got != tt.want {
				t.Errorf("standardVLAN(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDistributedVLAN(t *testing.T) {
	tests := []struct {
		name string
		vlan types.BaseVmwareDistributedVirtualSwitchVlanSpec
		want string
	}{
		{"single id", &types.VmwareDistributedVirtualSwitchVlanIdSpec{VlanId: 42}, "42"},
		{"trunk ranges", &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
			VlanId: []types.NumericRange{{Start: 1, End: 5}, {Start: 20, End: 20}},
		}, "1-5,20"},
		{"private vlan", &types.VmwareDistributedVirtualSwitchPvlanSpec{PvlanId: 7}, "pvlan-7"},
		{"untagged via nil spec", nil, "untagged"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &types.DVPortgroupConfigInfo{}
			if tt.vlan != nil {
				cfg.DefaultPortConfig = &types.VMwareDVSPortSetting{Vlan: tt.vlan}
			}
			if got := distributedVLAN(cfg); got != tt.want {
				t.Errorf("distributedVLAN(%+v) = %q, want %q", tt.vlan, got, tt.want)
			}
		})
	}
}
