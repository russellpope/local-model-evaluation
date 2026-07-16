package tests

import (
	"strings"
	"testing"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/storage"
)

func TestHumanSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1536 * 1024, "1.5 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
		{1536 * 1024 * 1024, "1.5 GiB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TiB"},
		{1536 * 1024 * 1024 * 1024, "1.5 TiB"},
		{1073741824, "1.0 GiB"}, // 1 GiB exactly
		{2147483648, "2.0 GiB"}, // 2 GiB exactly
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := format.HumanSize(tc.input)
			if result != tc.expected {
				t.Errorf("HumanSize(%d) = %s; want %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatVMs(t *testing.T) {
	t.Parallel()

	vms := []storage.VMInfo{
		{Name: "vm1", VCPU: 2, RAMGB: 4.0, StorageGB: 20.0},
		{Name: "vm2", VCPU: 4, RAMGB: 8.0, StorageGB: 40.0},
	}

	var buf strings.Builder
	format.FormatVMs(&buf, vms)

	output := buf.String()

	// Check header
	if !strings.Contains(output, "NAME") {
		t.Errorf("output missing NAME header")
	}
	if !strings.Contains(output, "VCPU") {
		t.Errorf("output missing VCPU header")
	}
	if !strings.Contains(output, "RAM") {
		t.Errorf("output missing RAM header")
	}
	if !strings.Contains(output, "STORAGE") {
		t.Errorf("output missing STORAGE header")
	}

	// Check data rows
	if !strings.Contains(output, "vm1") {
		t.Errorf("output missing vm1")
	}
	if !strings.Contains(output, "vm2") {
		t.Errorf("output missing vm2")
	}
}

func TestFormatDatastores(t *testing.T) {
	t.Parallel()

	ds := []storage.DatastoreInfo{
		{Name: "ds1", Type: "NFS", UsedGB: 10.0, AvailableGB: 90.0},
		{Name: "ds2", Type: "unknown", UsedGB: 20.0, AvailableGB: 80.0},
	}

	var buf strings.Builder
	format.FormatDatastores(&buf, ds)

	output := buf.String()

	// Check header
	if !strings.Contains(output, "NAME") {
		t.Errorf("output missing NAME header")
	}
	if !strings.Contains(output, "TYPE") {
		t.Errorf("output missing TYPE header")
	}
	if !strings.Contains(output, "USED") {
		t.Errorf("output missing USED header")
	}
	if !strings.Contains(output, "AVAILABLE") {
		t.Errorf("output missing AVAILABLE header")
	}

	// Check data rows
	if !strings.Contains(output, "ds1") {
		t.Errorf("output missing ds1")
	}
	if !strings.Contains(output, "ds2") {
		t.Errorf("output missing ds2")
	}
}

func TestFormatSwitches(t *testing.T) {
	t.Parallel()

	sw := []storage.SwitchInfo{
		{
			Name:       "vSwitch0",
			SwitchType: "standard",
			PortGroups: []storage.PortGroupInfo{{Name: "pg1", VLAN: "100"}},
			Uplinks:    []string{"eth0"},
			LACP:       "N/A",
			TotalPorts: 100,
			UsedPorts:  10,
		},
	}

	var buf strings.Builder
	format.FormatSwitches(&buf, sw)

	output := buf.String()

	// Check header
	if !strings.Contains(output, "SWITCH") {
		t.Errorf("output missing SWITCH header")
	}
	if !strings.Contains(output, "SWITCH TYPE") {
		t.Errorf("output missing SWITCH TYPE header")
	}
	if !strings.Contains(output, "PORTGROUP") {
		t.Errorf("output missing PORTGROUP header")
	}
	if !strings.Contains(output, "VLAN") {
		t.Errorf("output missing VLAN header")
	}
	if !strings.Contains(output, "UPLINKS") {
		t.Errorf("output missing UPLINKS header")
	}
	if !strings.Contains(output, "LACP") {
		t.Errorf("output missing LACP header")
	}
	if !strings.Contains(output, "PORTS") {
		t.Errorf("output missing PORTS header")
	}
	if !strings.Contains(output, "USED") {
		t.Errorf("output missing USED header")
	}

	// Check data rows
	if !strings.Contains(output, "vSwitch0") {
		t.Errorf("output missing vSwitch0")
	}
	if !strings.Contains(output, "pg1") {
		t.Errorf("output missing pg1")
	}
}

func TestFormatVMsByPortGroup(t *testing.T) {
	t.Parallel()

	vms := []storage.VMInfoForPortGroup{
		{Name: "vm1", PortGroup: "pg1"},
		{Name: "vm2", PortGroup: "pg1"},
	}

	var buf strings.Builder
	format.FormatVMsByPortGroup(&buf, vms)

	output := buf.String()

	// Check header
	if !strings.Contains(output, "VM NAME") {
		t.Errorf("output missing VM NAME header")
	}
	if !strings.Contains(output, "PORT GROUP") {
		t.Errorf("output missing PORT GROUP header")
	}

	// Check data rows
	if !strings.Contains(output, "vm1") {
		t.Errorf("output missing vm1")
	}
	if !strings.Contains(output, "vm2") {
		t.Errorf("output missing vm2")
	}
}
