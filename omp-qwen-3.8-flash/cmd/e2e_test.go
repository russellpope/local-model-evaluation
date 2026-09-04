package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// runRoot executes the real command tree against a live in-process simulator
// and captures stdout. This covers the connection wiring, config resolution,
// and tabwriter presentation that the inventory unit tests deliberately
// exclude.
func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	root.SetArgs(args)
	var buf bytes.Buffer
	stdout = &buf
	t.Cleanup(func() { stdout = os.Stdout })

	err := root.ExecuteContext(context.Background())
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Errorf("output contains ANSI color codes: %q", out)
	}
	return out, err
}

func startSim(t *testing.T) (*simulator.Model, func()) {
	t.Helper()
	m := simulator.VPX()
	m.Datacenter = 1
	m.Host = 1
	m.Cluster = 0
	m.ClusterHost = 0
	m.Machine = 3
	m.Datastore = 2
	m.Portgroup = 1
	if err := m.Create(); err != nil {
		t.Fatalf("simulator create: %v", err)
	}
	m.Service.TLS = new(tls.Config)
	m.Service.RegisterEndpoints = true
	s := m.Service.NewServer()
	t.Setenv("VSPHERE_URL", s.URL.String())
	t.Setenv("VSPHERE_USERNAME", "user")
	t.Setenv("VSPHERE_PASSWORD", "pass")
	t.Setenv("VSPHERE_INSECURE", "true")
	t.Setenv("VSPHERE_TIMEOUT", "30s")
	return m, func() { s.Close(); m.Remove() }
}

var headerRx = map[string]*regexp.Regexp{
	"vms":  regexp.MustCompile(`^NAME\s+VCPU\s+RAM\s+STORAGE$`),
	"data": regexp.MustCompile(`^NAME\s+TYPE\s+USED\s+AVAILABLE$`),
	"sw":   regexp.MustCompile(`^SWITCH\s+SWITCH TYPE\s+PORTGROUP\s+VLAN\s+UPLINKS\s+LACP\s+PORTS\s+USED$`),
}

func TestE2EVSubcommand(t *testing.T) {
	_, stop := startSim(t)
	defer stop()

	out, err := runRoot(t, "vms")
	if err != nil {
		t.Fatalf("vms: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1+3 {
		t.Fatalf("vms: want header + 3 rows, got %d lines:\n%s", len(lines), out)
	}
	if !headerRx["vms"].MatchString(lines[0]) {
		t.Errorf("vms header = %q", lines[0])
	}
	prev := ""
	for _, ln := range lines[1:] {
		name := strings.Fields(ln)[0]
		if name <= prev && prev != "" {
			t.Errorf("vms rows not sorted: %q after %q", name, prev)
		}
		prev = name
		if !strings.Contains(ln, "GiB") {
			t.Errorf("vms row lacks GiB units: %q", ln)
		}
	}
}

func TestE2EDatastores(t *testing.T) {
	_, stop := startSim(t)
	defer stop()

	out, err := runRoot(t, "datastores")
	if err != nil {
		t.Fatalf("datastores: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1+2 {
		t.Fatalf("datastores: want header + 2 rows, got %d:\n%s", len(lines), out)
	}
	if !headerRx["data"].MatchString(lines[0]) {
		t.Errorf("header = %q", lines[0])
	}
	for _, ln := range lines[1:] {
		cols := strings.Fields(ln)
		if cols[1] == "" {
			t.Errorf("empty TYPE column in %q", ln)
		}
	}
}

func TestE2EVSwitchesBothTypes(t *testing.T) {
	_, stop := startSim(t)
	defer stop()

	out, err := runRoot(t, "vswitches")
	if err != nil {
		t.Fatalf("vswitches: %v", err)
	}
	if !strings.Contains(out, "standard") {
		t.Errorf("no standard switch rows:\n%s", out)
	}
	if !strings.Contains(out, "distributed") {
		t.Errorf("no distributed switch rows:\n%s", out)
	}
	for _, ln := range strings.Split(out, "\n")[1:] {
		if ln == "" {
			continue
		}
		// LACP column must be N/A for standard rows.
		if strings.Contains(ln, "standard") && !strings.Contains(ln, "N/A") {
			t.Errorf("standard row without N/A LACP: %q", ln)
		}
	}
}

func TestE2EVSwitchesPortgroup(t *testing.T) {
	_, stop := startSim(t)
	defer stop()

	out, err := runRoot(t, "vswitches")
	if err != nil {
		t.Fatalf("vswitches: %v", err)
	}
	// Discover a distributed port group from our own output. The VLAN cell may
	// contain spaces ("0 (native)"), so parse from both ends: SWITCH(0),
	// SWITCH TYPE(1), ..., LACP, PORTS, USED are fixed; the trailing VLAN
	// tokens are 1-2 fields. Port group names are the first cell after the
	// switch type and never contain the VLAN keywords.
	pg := ""
	for _, ln := range strings.Split(out, "\n")[1:] {
		fields := strings.Fields(ln)
		if len(fields) < 8 || fields[1] != "distributed" {
			continue
		}
		// Walk backwards: USED, PORTS, LACP; VLAN is 1 or 2 fields; UPLINKS 1+;
		// PORTGROUP starts at index 2 and runs until VLAN/UPLINKS. For the
		// simulator shape (all single-token except VLAN) take field 2.
		pg = fields[2]
		break
	}
	if pg == "" {
		t.Fatalf("no distributed port group discovered in output:\n%s", out)
	}

	out, err = runRoot(t, "vswitches", "--portgroup", pg)
	if err != nil {
		t.Fatalf("vswitches --portgroup %q: %v", pg, err)
	}
	rows := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(rows) != 1+3 {
		t.Fatalf("portgroup %q: want header + 3 VM rows, got:\n%s", pg, out)
	}
	if !strings.HasPrefix(rows[0], "NAME") {
		t.Errorf("portgroup lookup output not in VM-table shape:\n%s", out)
	}
}

func TestE2EUnknownPortgroupFails(t *testing.T) {
	_, stop := startSim(t)
	defer stop()

	if _, err := runRoot(t, "vswitches", "--portgroup", "no-such-pg"); err == nil {
		t.Fatal("want error for unknown port group, got nil")
	}
}

func TestE2EAuthFailureWrapped(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://127.0.0.1:1/sdk") // refused
	t.Setenv("VSPHERE_USERNAME", "u")
	t.Setenv("VSPHERE_PASSWORD", "p")
	t.Setenv("VSPHERE_INSECURE", "true")
	t.Setenv("VSPHERE_TIMEOUT", "5s")

	_, err := runRoot(t, "vms")
	if err == nil {
		t.Fatal("want connection error, got nil")
	}
	if !strings.Contains(err.Error(), "connecting to") {
		t.Errorf("error not wrapped with actionable context: %v", err)
	}
}
