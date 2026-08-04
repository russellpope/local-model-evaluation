package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/model"
)

func TestBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"bytes", 512, "512 B"},
		{"one KiB", 1024, "1.0 KiB"},
		{"two KiB", 2048, "2.0 KiB"},
		{"one MiB", 1048576, "1.0 MiB"},
		{"one GiB", 1073741824, "1.0 GiB"},
		{"one TiB", 1099511627776, "1.0 TiB"},
		{"one PiB", 1125899906842624, "1.0 PiB"},
		{"large value", 1152921504606846976, "1.0 EiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Bytes(tt.input)
			if result != tt.expected {
				t.Errorf("Bytes(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBytesExactness(t *testing.T) {
	capacity := int64(10737418240) // 10 GiB
	used := int64(3221225472)      // 3 GiB
	available := capacity - used

	usedStr := Bytes(used)
	availStr := Bytes(available)

	if usedStr != "3.0 GiB" {
		t.Errorf("Bytes(%d) = %q, want %q", used, usedStr, "3.0 GiB")
	}
	if availStr != "7.0 GiB" {
		t.Errorf("Bytes(%d) = %q, want %q", available, availStr, "7.0 GiB")
	}
	if used+available != capacity {
		t.Errorf("used + available (%d + %d) != capacity (%d)", used, available, capacity)
	}
}

func TestRAMBytes(t *testing.T) {
	tests := []struct {
		name     string
		ramMB    int
		expected string
	}{
		{"32 MB", 32, "32.0 MiB"},
		{"1024 MB", 1024, "1.0 GiB"},
		{"0 MB", 0, "0 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RAMBytes(tt.ramMB)
			if result != tt.expected {
				t.Errorf("RAMBytes(%d) = %q, want %q", tt.ramMB, result, tt.expected)
			}
		})
	}
}

func TestRenderVMs(t *testing.T) {
	vms := []model.VMInfo{
		{Name: "DC0_C0_RP0_VM1", VCPU: 1, RAMMB: 32, StorageBytes: 234},
		{Name: "DC0_C0_RP0_VM0", VCPU: 1, RAMMB: 32, StorageBytes: 234},
		{Name: "DC0_C0_RP0_VM2", VCPU: 1, RAMMB: 32, StorageBytes: 234},
	}

	var buf bytes.Buffer
	RenderVMs(&buf, vms)

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("output should contain NAME header")
	}
	if !strings.Contains(output, "VCPU") {
		t.Error("output should contain VCPU column")
	}
	if !strings.Contains(output, "RAM") {
		t.Error("output should contain RAM column")
	}
	if !strings.Contains(output, "STORAGE") {
		t.Error("output should contain STORAGE column")
	}
	if !strings.Contains(output, "DC0_C0_RP0_VM0") {
		t.Error("output should contain VM0")
	}
	if !strings.Contains(output, "DC0_C0_RP0_VM1") {
		t.Error("output should contain VM1")
	}
	if !strings.Contains(output, "DC0_C0_RP0_VM2") {
		t.Error("output should contain VM2")
	}
	if !strings.Contains(output, "32.0 MiB") {
		t.Error("output should contain 32.0 MiB for RAM")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (header + 3 VMs), got %d", len(lines))
	}

	if !strings.HasPrefix(lines[1], "DC0_C0_RP0_VM0") {
		t.Errorf("first VM should be DC0_C0_RP0_VM0 (sorted), got %q", lines[1])
	}
}

func TestRenderDatastores(t *testing.T) {
	datastores := []DatastoreInfo{
		{Name: "ds2", Type: "unknown", UsedBytes: 3221225472, AvailableBytes: 7516192768, CapacityBytes: 10737418240},
		{Name: "ds1", Type: "unknown", UsedBytes: 3221225472, AvailableBytes: 7516192768, CapacityBytes: 10737418240},
	}

	var buf bytes.Buffer
	RenderDatastores(&buf, datastores)

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("output should contain NAME header")
	}
	if !strings.Contains(output, "TYPE") {
		t.Error("output should contain TYPE column")
	}
	if !strings.Contains(output, "USED") {
		t.Error("output should contain USED column")
	}
	if !strings.Contains(output, "AVAILABLE") {
		t.Error("output should contain AVAILABLE column")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	header := lines[0]
	usedIdx := strings.Index(header, "USED")
	availIdx := strings.Index(header, "AVAILABLE")
	if usedIdx == -1 {
		t.Error("output should contain USED column")
	}
	if availIdx == -1 {
		t.Error("output should contain AVAILABLE column")
	}
	if usedIdx >= availIdx {
		t.Errorf("USED should come before AVAILABLE in header, got USED at %d, AVAILABLE at %d", usedIdx, availIdx)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 lines (header + 2 datastores), got %d", len(lines))
	}

	if !strings.HasPrefix(lines[1], "ds1") {
		t.Errorf("first datastore should be ds1 (sorted), got %q", lines[1])
	}
}

func TestRenderVSwitches(t *testing.T) {
	switches := []SwitchInfo{
		{Name: "DVS0", Type: "distributed", Portgroup: "DC0_DVPG1", VLAN: "0", Uplinks: "N/A", LACP: "N/A", TotalPorts: 5, UsedPorts: 0},
		{Name: "DVS0", Type: "distributed", Portgroup: "DC0_DVPG0", VLAN: "0", Uplinks: "N/A", LACP: "N/A", TotalPorts: 5, UsedPorts: 0},
		{Name: "vSwitch0", Type: "standard", Portgroup: "VM Network", VLAN: "0", Uplinks: "vmnic0", LACP: "disabled", TotalPorts: 1536, UsedPorts: 6},
	}

	var buf bytes.Buffer
	RenderVSwitches(&buf, switches)

	output := buf.String()
	if !strings.Contains(output, "SWITCH") {
		t.Error("output should contain SWITCH header")
	}
	if !strings.Contains(output, "SWITCH TYPE") {
		t.Error("output should contain SWITCH TYPE column")
	}
	if !strings.Contains(output, "PORTGROUP") {
		t.Error("output should contain PORTGROUP column")
	}
	if !strings.Contains(output, "VLAN") {
		t.Error("output should contain VLAN column")
	}
	if !strings.Contains(output, "UPLINKS") {
		t.Error("output should contain UPLINKS column")
	}
	if !strings.Contains(output, "LACP") {
		t.Error("output should contain LACP column")
	}
	if !strings.Contains(output, "PORTS") {
		t.Error("output should contain PORTS column")
	}
	if !strings.Contains(output, "USED") {
		t.Error("output should contain USED column")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (header + 3 switches), got %d", len(lines))
	}
}
