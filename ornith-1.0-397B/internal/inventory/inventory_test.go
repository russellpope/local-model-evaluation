package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func setupSimulator(t *testing.T) *vim25.Client {
	t.Helper()
	model := simulator.VPX()
	model.Datastore = 3
	model.Machine = 4

	if err := model.Create(); err != nil {
		t.Fatal(err)
	}

	sim := model.Service.NewServer()
	t.Cleanup(sim.Close)

	client, err := govmomi.NewClient(context.Background(), sim.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	return client.Client
}

func TestGetVMs(t *testing.T) {
	c := setupSimulator(t)
	ctx := context.Background()

	vms, err := GetVMs(ctx, c)
	if err != nil {
		t.Fatalf("GetVMs: %v", err)
	}

	if len(vms) == 0 {
		t.Error("expected at least 1 VM")
	}

	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM name should not be empty")
		}
		if vm.VCPU <= 0 {
			t.Errorf("VM %s: vCPU should be > 0, got %d", vm.Name, vm.VCPU)
		}
		if vm.RAMMB <= 0 {
			t.Errorf("VM %s: RAM should be > 0, got %d", vm.Name, vm.RAMMB)
		}
		if vm.Storage < 0 {
			t.Errorf("VM %s: storage should be >= 0, got %d", vm.Name, vm.Storage)
		}
	}

	// Verify sorted by name
	for i := 1; i < len(vms); i++ {
		if vms[i].Name < vms[i-1].Name {
			t.Errorf("VMs not sorted: %s before %s", vms[i-1].Name, vms[i].Name)
		}
	}
}

func TestGetDatastores(t *testing.T) {
	c := setupSimulator(t)
	ctx := context.Background()

	dss, err := GetDatastores(ctx, c)
	if err != nil {
		t.Fatalf("GetDatastores: %v", err)
	}

	if len(dss) == 0 {
		t.Error("expected at least 1 datastore")
	}

	validTypes := map[string]bool{
		"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true,
	}

	for _, ds := range dss {
		if ds.Name == "" {
			t.Error("datastore name should not be empty")
		}
		if !validTypes[ds.Type] {
			t.Errorf("datastore %s: invalid type %q", ds.Name, ds.Type)
		}
		if ds.Available < 0 {
			t.Errorf("datastore %s: available should be >= 0", ds.Name)
		}
		if ds.Used < 0 {
			t.Errorf("datastore %s: used should be >= 0", ds.Name)
		}
		// used + available should be <= capacity (within rounding)
		// We can't check exact capacity from our struct, but used should be reasonable
	}

	// Verify sorted by name
	for i := 1; i < len(dss); i++ {
		if dss[i].Name < dss[i-1].Name {
			t.Errorf("datastores not sorted: %s before %s", dss[i-1].Name, dss[i].Name)
		}
	}
}

func TestGetVSwitches(t *testing.T) {
	c := setupSimulator(t)
	ctx := context.Background()

	switches, err := GetVSwitches(ctx, c)
	if err != nil {
		t.Fatalf("GetVSwitches: %v", err)
	}

	if len(switches) == 0 {
		t.Error("expected at least one switch")
	}

	validLACP := map[string]bool{
		"enabled": true, "disabled": true, "N/A": true,
	}

	for _, sw := range switches {
		if sw.SwitchName == "" {
			t.Error("switch name should not be empty")
		}
		if sw.SwitchType != "standard" && sw.SwitchType != "distributed" {
			t.Errorf("switch %s: invalid type %q", sw.SwitchName, sw.SwitchType)
		}
		if !validLACP[sw.LACP] {
			t.Errorf("switch %s: invalid LACP %q", sw.SwitchName, sw.LACP)
		}
		if sw.Ports > 0 && sw.UsedPorts > sw.Ports {
			t.Errorf("switch %s: used ports (%d) > total ports (%d)", sw.SwitchName, sw.UsedPorts, sw.Ports)
		}
		// VLAN values should parse (not empty for non-N/A cases)
		for _, pg := range sw.PortGroups {
			if pg.Name == "" {
				t.Errorf("switch %s: port group name should not be empty", sw.SwitchName)
			}
		}
	}
}

func TestGetVMsByPortGroup(t *testing.T) {
	c := setupSimulator(t)
	ctx := context.Background()

	// First, get the list of switches to find a real port group name
	switches, err := GetVSwitches(ctx, c)
	if err != nil {
		t.Fatalf("GetVSwitches: %v", err)
	}

	if len(switches) == 0 {
		t.Skip("no switches found")
	}

	// Find a port group name
	var pgName string
	for _, sw := range switches {
		if len(sw.PortGroups) > 0 {
			pgName = sw.PortGroups[0].Name
			break
		}
	}

	if pgName == "" {
		t.Skip("no port group found")
	}

	vms, err := GetVMsByPortGroup(ctx, c, pgName)
	if err != nil {
		t.Fatalf("GetVMsByPortGroup(%q): %v", pgName, err)
	}

	// Should return some VMs (simulator attaches VMs to port groups)
	// The exact count depends on simulator model, but should not error
	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM name should not be empty")
		}
	}
}
