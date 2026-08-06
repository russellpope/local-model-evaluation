package vswitch

import (
	"context"
	"strconv"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetSwitches(t *testing.T) {
	model := simulator.VPX()
	model.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		results, err := GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches: %v", err)
		}

		if len(results) < 1 {
			t.Fatal("expected at least 1 switch, got 0")
		}

		validSwitchTypes := map[string]bool{"standard": true, "distributed": true}
		validLACP := map[string]bool{"enabled": true, "disabled": true, "N/A": true}

		for _, sw := range results {
			if sw.SwitchName == "" {
				t.Error("switch has empty name")
			}
			if !validSwitchTypes[sw.SwitchType] {
				t.Errorf("switch %q has invalid type %q", sw.SwitchName, sw.SwitchType)
			}
			if sw.PortGroupName == "" {
				t.Errorf("switch %q has empty port group name", sw.SwitchName)
			}
			if !validLACP[sw.LACP] {
				t.Errorf("switch %q has invalid LACP %q", sw.SwitchName, sw.LACP)
			}
			if sw.UsedPorts > sw.Ports {
				t.Errorf("switch %q has used ports (%d) > total ports (%d)", sw.SwitchName, sw.UsedPorts, sw.Ports)
			}
			if sw.VLAN == "" {
				t.Errorf("switch %q has empty VLAN", sw.SwitchName)
			}
			if sw.Uplinks == "" {
				t.Errorf("switch %q has empty uplinks", sw.SwitchName)
			}
		}

		for i := 1; i < len(results); i++ {
			if results[i-1].SwitchName > results[i].SwitchName {
				t.Error("switches are not sorted by name")
			}
		}
	}, model)
}

func TestGetSwitchesDefault(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		results, err := GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches: %v", err)
		}

		if len(results) < 1 {
			t.Fatal("expected at least 1 switch, got 0")
		}

		hasStandard := false
		hasDistributed := false

		for _, sw := range results {
			if sw.SwitchType == "standard" {
				hasStandard = true
			}
			if sw.SwitchType == "distributed" {
				hasDistributed = true
			}
		}

		if !hasStandard {
			t.Error("expected at least one standard switch")
		}
		if !hasDistributed {
			t.Error("expected at least one distributed switch")
		}
	})
}

func TestGetVMsForPortGroup(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 3
	model.Portgroup = 1

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches: %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("no switches found")
		}

		var portGroupName string
		for _, sw := range switches {
			if sw.SwitchType == "distributed" {
				portGroupName = sw.PortGroupName
				break
			}
		}

		if portGroupName == "" {
			for _, sw := range switches {
				if sw.SwitchType == "standard" && sw.PortGroupName == "VM Network" {
					portGroupName = sw.PortGroupName
					break
				}
			}
		}

		if portGroupName == "" {
			for _, sw := range switches {
				portGroupName = sw.PortGroupName
				break
			}
		}

		vms, err := GetVMsForPortGroup(ctx, c, portGroupName)
		if err != nil {
			t.Fatalf("GetVMsForPortGroup: %v", err)
		}

		if len(vms) < 1 {
			t.Errorf("expected at least 1 VM connected to port group %q, got 0", portGroupName)
		}

		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("VM has empty name")
			}
		}

		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Error("VMs are not sorted by name")
			}
		}
	}, model)
}

func TestGetVMsForPortGroupVMNetwork(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 3
	model.Portgroup = 0

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := GetVMsForPortGroup(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("GetVMsForPortGroup: %v", err)
		}

		if len(vms) < 1 {
			t.Errorf("expected at least 1 VM connected to 'VM Network', got 0")
		}
	}, model)
}

func TestGetVMsForPortGroupNonExistent(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := GetVMsForPortGroup(ctx, c, "NonExistentPortGroup")
		if err != nil {
			t.Fatalf("GetVMsForPortGroup: %v", err)
		}

		if len(vms) != 0 {
			t.Errorf("expected 0 VMs for non-existent port group, got %d", len(vms))
		}
	})
}

func TestVLANFormatStandard(t *testing.T) {
	tests := []struct {
		vlanId   int32
		expected string
	}{
		{0, "N/A"},
		{1, "1"},
		{100, "100"},
		{4094, "4094"},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(int(tt.vlanId)), func(t *testing.T) {
			result := formatStandardVLAN(tt.vlanId)
			if result != tt.expected {
				t.Errorf("formatStandardVLAN(%d) = %q, want %q", tt.vlanId, result, tt.expected)
			}
		})
	}
}
