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

	// Get all VMs for cross-validation
	allVMs, err := GetVMs(ctx, c)
	if err != nil {
		t.Fatalf("GetVMs: %v", err)
	}
	if len(allVMs) == 0 {
		t.Fatal("simulator should have at least 1 VM")
	}

	// Get switches to find a real port group name
	switches, err := GetVSwitches(ctx, c)
	if err != nil {
		t.Fatalf("GetVSwitches: %v", err)
	}
	if len(switches) == 0 {
		t.Fatal("simulator should have at least one switch")
	}

	// Find a port group that actually has VMs connected.
	// (In the VPX simulator, VMs attach to the distributed PG, not the
	// standard "VM Network".)
	var pgName string
	var vms []VMInfo
	for _, sw := range switches {
		for _, pg := range sw.PortGroups {
			result, err := GetVMsByPortGroup(ctx, c, pg.Name)
			if err != nil {
				t.Fatalf("GetVMsByPortGroup(%q): %v", pg.Name, err)
			}
			if len(result) > 0 {
				pgName = pg.Name
				vms = result
				break
			}
		}
		if pgName != "" {
			break
		}
	}
	if pgName == "" {
		t.Fatal("no port group with connected VMs found in simulator")
	}

	if len(vms) == 0 {
		t.Errorf("GetVMsByPortGroup(%q): expected at least 1 VM, got 0", pgName)
	}

	// Validate returned VMs have required fields populated
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
	}

	// Returned VMs must be a subset of all VMs
	allVMNames := make(map[string]bool, len(allVMs))
	for _, vm := range allVMs {
		allVMNames[vm.Name] = true
	}
	for _, vm := range vms {
		if !allVMNames[vm.Name] {
			t.Errorf("VM %q returned by port group lookup but not in full VM list", vm.Name)
		}
	}
}
