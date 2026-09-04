package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/example/govc-inventory/internal/vsphere"
)

func TestWriteVMsTable(t *testing.T) {
	var buf bytes.Buffer
	vms := []vsphere.VMInfo{
		{Name: "DC0_H0_VM0", VCPU: 2, RAMMB: 4096, CommittedStorage: 5 * 1 << 30},
		{Name: "DC0_H0_VM1", VCPU: 4, RAMMB: 8192, CommittedStorage: 3 * 1 << 40},
	}
	if err := writeVMs(&buf, vms); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want header + 2 rows, got %d lines:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "NAME") || !strings.Contains(lines[0], "STORAGE") {
		t.Errorf("bad header: %q", lines[0])
	}
	if !strings.Contains(out, "4.0 GiB") {
		t.Errorf("RAM not in GB: %q", out)
	}
	if !strings.Contains(out, "5.0 GiB") {
		t.Errorf("storage GiB format wrong: %q", out)
	}
	if !strings.Contains(out, "3.0 TiB") {
		t.Errorf("storage TiB format wrong: %q", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("color codes in output")
	}
}

func TestWriteDatastoresTable(t *testing.T) {
	var buf bytes.Buffer
	dss := []vsphere.DatastoreInfo{
		{Name: "LocalDS_0", Transport: "FC", Capacity: 100<<30 + 1<<29, Free: 60 << 30},
		{Name: "nfs-ds", Transport: "NFS", Capacity: 5 << 30, Free: 2 << 30},
	}
	if err := writeDatastores(&buf, dss); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "USED") || !strings.Contains(out, "AVAILABLE") {
		t.Errorf("missing header:\n%s", out)
	}
	// used = capacity - free = 40.5 GiB
	if !strings.Contains(out, "40.5 GiB") {
		t.Errorf("used math/format wrong:\n%s", out)
	}
	if !strings.Contains(out, "60.0 GiB") {
		t.Errorf("available wrong:\n%s", out)
	}
}

func TestWriteSwitchesTable(t *testing.T) {
	var buf bytes.Buffer
	rows := []vsphere.PortGroupInfo{
		{
			SwitchName: "DVS0", SwitchType: "distributed", Name: "DC0_DVPG0",
			VLAN: "100", Uplinks: []string{"uplink-1", "uplink-2"}, LACP: "enabled",
			NumPorts: 512, UsedPorts: 4,
		},
		{
			SwitchName: "vSwitch0", SwitchType: "standard", Name: "VM Network",
			VLAN: "0", Uplinks: []string{"vmnic0"}, LACP: "N/A",
			NumPorts: 1536, UsedPorts: 6,
		},
	}
	if err := writeSwitches(&buf, rows); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	hj := lines[0]
	for _, want := range []string{"SWITCH", "PORTGROUP", "VLAN", "UPLINKS", "LACP", "PORTS", "USED"} {
		if !strings.Contains(hj, want) {
			t.Errorf("missing column %q in header %q", want, hj)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("want header + 2 rows, got:\n%s", out)
	}
	if !strings.Contains(out, "uplink-1,uplink-2") {
		t.Errorf("uplinks not comma-joined:\n%s", out)
	}
	if !strings.Contains(out, "enabled") || !strings.Contains(out, "N/A") {
		t.Errorf("LACP values missing:\n%s", out)
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		user    string
		want    string
		wantErr bool
	}{
		{"full url", "https://vc.lab/sdk", "u", "https://u:pw@vc.lab/sdk", false},
		{"bare host", "vc.lab", "u", "https://u:pw@vc.lab/sdk", false},
		{"host port", "127.0.0.1:8989", "u", "https://u:pw@127.0.0.1:8989/sdk", false},
		{"missing path", "https://vc.lab", "u", "https://u:pw@vc.lab/sdk", false},
		{"creds in url", "https://a:b@vc.lab/sdk", "", "https://a:b@vc.lab/sdk", false},
		{"empty", "", "u", "", true},
		{"no user", "https://vc.lab/sdk", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := parseURL(tc.raw, tc.user, "pw")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", u)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if u.String() != tc.want {
				t.Errorf("got %q, want %q", u.String(), tc.want)
			}
		})
	}
}

func TestResolveConfigDefaults(t *testing.T) {
	fs := newConnFlagSet()
	s, err := resolveConfig(fs)
	if err != nil {
		t.Fatal(err)
	}
	if s.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", s.Timeout)
	}
}

func TestResolveConfigFlagBeatsEnv(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://env.example/sdk")
	fs := newConnFlagSet()
	if err := fs.Parse([]string{"--url", "https://flag.example/sdk"}); err != nil {
		t.Fatal(err)
	}
	s, err := resolveConfig(fs)
	if err != nil {
		t.Fatal(err)
	}
	if s.URL != "https://flag.example/sdk" {
		t.Errorf("URL = %q, want flag to override env", s.URL)
	}
}
