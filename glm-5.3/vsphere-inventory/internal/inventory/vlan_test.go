package inventory

import (
	"reflect"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestFormatStandardVLAN(t *testing.T) {
	tests := []struct {
		name string
		in   int32
		want string
	}{
		{"untagged", 0, "none"},
		{"single id", 100, "100"},
		{"trunk all", 4095, "0-4095"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatStandardVLAN(tt.in); got != tt.want {
				t.Errorf("FormatStandardVLAN(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatDistributedVLAN(t *testing.T) {
	inherited := types.VmwareDistributedVirtualSwitchVlanIdSpec{
		VmwareDistributedVirtualSwitchVlanSpec: types.VmwareDistributedVirtualSwitchVlanSpec{
			InheritablePolicy: types.InheritablePolicy{Inherited: true},
		},
	}
	tests := []struct {
		name string
		in   types.BaseDVPortSetting
		want string
	}{
		{
			name: "nil setting",
			in:   nil,
			want: "none",
		},
		{
			name: "vlan id spec",
			in:   &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{VlanId: 42}},
			want: "42",
		},
		{
			name: "vlan id 0",
			in:   &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{}},
			want: "none",
		},
		{
			name: "inherited",
			in:   &types.VMwareDVSPortSetting{Vlan: &inherited},
			want: "none",
		},
		{
			name: "trunk single range",
			in: &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
				VlanId: []types.NumericRange{{Start: 10, End: 20}},
			}},
			want: "10-20",
		},
		{
			name: "trunk multiple ranges",
			in: &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
				VlanId: []types.NumericRange{{Start: 0, End: 4094}},
			}},
			want: "0-4094",
		},
		{
			name: "trunk single id range",
			in: &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
				VlanId: []types.NumericRange{{Start: 30, End: 30}},
			}},
			want: "30",
		},
		{
			name: "private vlan",
			in:   &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchPvlanSpec{PvlanId: 1001}},
			want: "private(1001)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDistributedVLAN(tt.in); got != tt.want {
				t.Errorf("FormatDistributedVLAN(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseVLAN(t *testing.T) {
	valid := []struct {
		in   string
		want []int
	}{
		{"none", nil},
		{"unknown", nil},
		{"", nil},
		{"100", []int{100}},
		{"0", []int{0}},
		{"4095", []int{4095}},
		{"10-12", []int{10, 11, 12}},
		{"0-4095", nil}, // valid but huge; only validity is checked below
		{"10-11,30", []int{10, 11, 30}},
		{"private(1001)", []int{1001}},
	}
	for _, tt := range valid {
		t.Run(tt.in, func(t *testing.T) {
			ids, err := ParseVLAN(tt.in)
			if err != nil {
				t.Fatalf("ParseVLAN(%q): %v", tt.in, err)
			}
			if tt.in == "0-4095" {
				if len(ids) != 4096 {
					t.Fatalf("ParseVLAN(%q) = %d ids, want 4096", tt.in, len(ids))
				}
				return
			}
			if !reflect.DeepEqual(ids, tt.want) {
				t.Fatalf("ParseVLAN(%q) = %v, want %v", tt.in, ids, tt.want)
			}
		})
	}

	invalid := []string{"abc", "-1", "4096", "20-10", "10-", "-12", "1,,2", "private(", "private(x)", "10-20x"}
	for _, in := range invalid {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseVLAN(in); err == nil {
				t.Fatalf("ParseVLAN(%q) unexpectedly succeeded", in)
			}
		})
	}
}
