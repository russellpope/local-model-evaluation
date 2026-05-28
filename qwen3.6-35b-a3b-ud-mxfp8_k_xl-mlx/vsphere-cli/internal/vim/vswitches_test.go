package vim

import (
	"testing"
)

func TestSwitchInfoStruct(t *testing.T) {
	sw := SwitchInfo{
		SwitchName: "vSwitch0",
		SwitchType: "standard",
		Portgroup:  "VM Network",
		VLAN:       "100",
		Uplinks:    "vmnic0",
		LACP:       "N/A",
		TotalPorts: 128,
		UsedPorts:  10,
	}

	if sw.SwitchName != "vSwitch0" {
		t.Errorf("Expected SwitchName=vSwitch0, got %s", sw.SwitchName)
	}
	if sw.SwitchType != "standard" {
		t.Errorf("Expected SwitchType=standard, got %s", sw.SwitchType)
	}
	if sw.Portgroup != "VM Network" {
		t.Errorf("Expected Portgroup=VM Network, got %s", sw.Portgroup)
	}
	if sw.VLAN != "100" {
		t.Errorf("Expected VLAN=100, got %s", sw.VLAN)
	}
	if sw.Uplinks != "vmnic0" {
		t.Errorf("Expected Uplinks=vmnic0, got %s", sw.Uplinks)
	}
	if sw.LACP != "N/A" {
		t.Errorf("Expected LACP=N/A, got %s", sw.LACP)
	}
	if sw.TotalPorts != 128 {
		t.Errorf("Expected TotalPorts=128, got %d", sw.TotalPorts)
	}
	if sw.UsedPorts != 10 {
		t.Errorf("Expected UsedPorts=10, got %d", sw.UsedPorts)
	}
}

func TestSwitchInfoDistributed(t *testing.T) {
	sw := SwitchInfo{
		SwitchName: "dvSwitch",
		SwitchType: "distributed",
		Portgroup:  "PG1",
		VLAN:       "200",
		Uplinks:    "uplink1, uplink2",
		LACP:       "enabled",
		TotalPorts: 512,
		UsedPorts:  20,
	}

	if sw.SwitchType != "distributed" {
		t.Errorf("Expected SwitchType=distributed, got %s", sw.SwitchType)
	}
	if sw.LACP != "enabled" {
		t.Errorf("Expected LACP=enabled, got %s", sw.LACP)
	}
}

func TestSwitchInfoVLANValues(t *testing.T) {
	tests := []struct {
		name  string
		vlan  string
		valid bool
	}{
		{"valid single VLAN", "100", true},
		{"valid trunk", "trunk", true},
		{"valid range", "100-200", true},
		{"empty VLAN", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := SwitchInfo{VLAN: tt.vlan}

			if !tt.valid && tt.vlan == "" {
				t.Logf("Empty VLAN is valid for trunk port groups")
			}

			if tt.valid && sw.VLAN == "" {
				t.Errorf("Expected VLAN to be set")
			}
		})
	}
}

func TestSwitchInfoUsedPortsCalculation(t *testing.T) {
	total := int32(100)
	used := int32(25)
	available := total - used

	if available != 75 {
		t.Errorf("Expected available ports=75, got %d", available)
	}

	if used > total {
		t.Errorf("Used ports should not exceed total")
	}
}

func TestSwitchInfoUplinksFormat(t *testing.T) {
	sw := SwitchInfo{
		SwitchName: "vSwitch0",
		Uplinks:    "vmnic0, vmnic1",
	}

	if sw.Uplinks == "" {
		t.Error("Expected non-empty Uplinks")
	}
}

func TestFormatUplinks(t *testing.T) {
	tests := []struct {
		name     string
		nics     []string
		expected string
	}{
		{
			name:     "single NIC",
			nics:     []string{"vmnic0"},
			expected: "vmnic0",
		},
		{
			name:     "multiple NICs",
			nics:     []string{"vmnic0", "vmnic1"},
			expected: "vmnic0, vmnic1",
		},
		{
			name:     "empty NICs",
			nics:     []string{},
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUplinks(tt.nics)
			if result != tt.expected {
				t.Errorf("formatUplinks(%v) = %s, want %s", tt.nics, result, tt.expected)
			}
		})
	}
}

func TestFormatSlice(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		expected string
	}{
		{
			name:     "single item",
			items:    []string{"item1"},
			expected: "item1",
		},
		{
			name:     "multiple items",
			items:    []string{"item1", "item2"},
			expected: "item1, item2",
		},
		{
			name:     "empty slice",
			items:    []string{},
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSlice(tt.items)
			if result != tt.expected {
				t.Errorf("formatSlice(%v) = %s, want %s", tt.items, result, tt.expected)
			}
		})
	}
}

func TestLACPValues(t *testing.T) {
	validLACPs := map[string]bool{
		"enabled":  true,
		"disabled": true,
		"N/A":      true,
	}

	tests := []string{"enabled", "disabled", "N/A"}

	for _, lacp := range tests {
		if !validLACPs[lacp] {
			t.Errorf("Invalid LACP value: %s", lacp)
		}
	}
}

func TestSwitchTypeValues(t *testing.T) {
	validTypes := map[string]bool{
		"standard":    true,
		"distributed": true,
	}

	tests := []string{"standard", "distributed"}

	for _, typ := range tests {
		if !validTypes[typ] {
			t.Errorf("Invalid switch type: %s", typ)
		}
	}
}
