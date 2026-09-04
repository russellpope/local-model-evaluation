package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

func TestRenderSwitchesShape(t *testing.T) {
	var buf bytes.Buffer
	rows := []inventory.SwitchInfo{
		{Switch: "DVS0", SwitchType: "distributed", Portgroup: "DC0_DVPG0", VLAN: "0", Uplinks: "uplink1,uplink2", LACP: "disabled", Ports: 8, Used: 2},
		{Switch: "vSwitch0", SwitchType: "standard", Portgroup: "VM Network", VLAN: "0", Uplinks: "vmnic0", LACP: "N/A", Ports: 128, Used: 0},
	}
	if err := renderSwitches(&buf, rows); err != nil {
		t.Fatalf("renderSwitches: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header + 2 rows:\n%s", len(lines), buf.String())
	}
	header := strings.Join(strings.Fields(lines[0]), " ")
	wantHeader := "SWITCH SWITCH TYPE PORTGROUP VLAN UPLINKS LACP PORTS USED"
	if header != wantHeader {
		t.Errorf("header = %q, want %q", header, wantHeader)
	}
	if !strings.Contains(lines[1], "DVS0") || !strings.Contains(lines[1], "disabled") {
		t.Errorf("distributed row wrong: %q", lines[1])
	}
	if !strings.Contains(lines[2], "N/A") {
		t.Errorf("standard row missing LACP N/A: %q", lines[2])
	}
}

func TestRenderVMsAndDatastores(t *testing.T) {
	var buf bytes.Buffer
	if err := renderVMs(&buf, []inventory.VMInfo{{Name: "DC0_H0_VM0", NumCPU: 2, MemoryMB: 8192, Committed: 3 * (1 << 30)}}); err != nil {
		t.Fatalf("renderVMs: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "DC0_H0_VM0") || !strings.Contains(out, "8 GB") || !strings.Contains(out, "3.0 GiB") {
		t.Errorf("VM table wrong:\n%s", out)
	}

	buf.Reset()
	if err := renderDatastores(&buf, []inventory.DatastoreInfo{{Name: "LocalDS_0", Type: "unknown", Capacity: 2 << 40, Used: 1 << 40, Available: 1 << 40}}); err != nil {
		t.Fatalf("renderDatastores: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "TYPE") || !strings.Contains(out, "1.0 TiB") || !strings.Contains(out, "unknown") {
		t.Errorf("datastore table wrong:\n%s", out)
	}
}
