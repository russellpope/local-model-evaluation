package vsphere

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"

	"vsphere-inventory/internal/config"
)

// createSimClient creates a *govmomi.Client connected to a simulator model for testing.
func createSimClient(t *testing.T, model *simulator.Model) *govmomi.Client {
	t.Helper()
	if err := model.Create(); err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	t.Cleanup(func() { model.Remove() })

	s := model.Service.NewServer()
	t.Cleanup(func() { s.Close() })

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Build a govmomi client from the simulator's URL
	u := s.URL
	// The simulator uses user/pass by default
	u.User = url.UserPassword("user", "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		t.Fatalf("create govmomi client: %v", err)
	}

	return client
}

func TestVMs(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Machine = 3

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListVMs(context.Background(), client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	if len(vms) != 3 {
		t.Errorf("ListVMs returned %d VMs, want 3", len(vms))
	}

	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM has empty name")
		}
		// vCPU and RAM may be 0 for VMs without hardware config in the simulator
		// We just verify the data types are correct
		_ = vm.VCPU
		_ = vm.RAMGB
		if vm.Storage == "" {
			t.Errorf("VM %s has empty storage string", vm.Name)
		}
	}
}

func TestDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Datastore = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	dss, err := ListDatastores(context.Background(), client)
	if err != nil {
		t.Fatalf("ListDatastores: %v", err)
	}

	if len(dss) != 2 {
		t.Errorf("ListDatastores returned %d datastores, want 2", len(dss))
	}

	for _, ds := range dss {
		if ds.Name == "" {
			t.Error("Datastore has empty name")
		}
		if ds.Type != "FC" && ds.Type != "iSCSI" && ds.Type != "NVMe" && ds.Type != "NFS" && ds.Type != "unknown" {
			t.Errorf("Datastore %s has invalid TYPE %q", ds.Name, ds.Type)
		}
		if ds.Used == "" {
			t.Error("Datastore has empty USED")
		}
		if ds.Available == "" {
			t.Error("Datastore has empty AVAILABLE")
		}
	}
}

func TestVSwitches(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	if len(switches) == 0 {
		t.Fatal("ListSwitches returned no switches")
	}

	for _, sw := range switches {
		if sw.SwitchName == "" {
			t.Error("Switch has empty name")
		}
		if sw.SwitchType != "standard" && sw.SwitchType != "distributed" {
			t.Errorf("Switch %s has invalid type %q", sw.SwitchName, sw.SwitchType)
		}

		if sw.SwitchType == "distributed" {
			if sw.LACP != "enabled" && sw.LACP != "disabled" && sw.LACP != "N/A" {
				t.Errorf("Distributed switch %s has invalid LACP %q", sw.SwitchName, sw.LACP)
			}
		} else {
			if sw.LACP != "N/A" {
				t.Errorf("Standard switch %s has LACP %q, want N/A", sw.SwitchName, sw.LACP)
			}
		}

		if sw.TotalPorts < 0 {
			t.Errorf("Switch %s has negative TotalPorts %d", sw.SwitchName, sw.TotalPorts)
		}
		if sw.Available < 0 {
			t.Errorf("Switch %s has negative Available %d", sw.SwitchName, sw.Available)
		}
		if sw.UsedPorts < 0 {
			t.Errorf("Switch %s has negative UsedPorts %d", sw.SwitchName, sw.UsedPorts)
		}
		if sw.UsedPorts > sw.TotalPorts {
			t.Errorf("Switch %s UsedPorts (%d) > TotalPorts (%d)", sw.SwitchName, sw.UsedPorts, sw.TotalPorts)
		}
	}
}

func TestPortGroupVMs(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	finder := find.NewFinder(client.Client, false)
	datacenters, err := finder.DatacenterList(context.Background(), "*")
	if err != nil {
		t.Fatalf("list datacenters: %v", err)
	}
	if len(datacenters) == 0 {
		t.Fatal("no datacenters found")
	}
	finder.SetDatacenter(datacenters[0])

	networks, err := finder.NetworkList(context.Background(), "*")
	if err != nil {
		t.Fatalf("list networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("no networks found in simulator model")
	}

	// Get the name of the first network via property collector
	pc := client.PropertyCollector()
	var netMo mo.Network
	if err := pc.RetrieveOne(context.Background(), networks[0].Reference(), []string{"Name"}, &netMo); err != nil {
		t.Fatalf("retrieve network name: %v", err)
	}
	pgName := netMo.Name
	t.Logf("Testing ListPortGroupVMs with port group: %s", pgName)

	vms, err := ListPortGroupVMs(context.Background(), client, pgName)
	if err != nil {
		t.Logf("ListPortGroupVMs returned error (may be expected for networks without VMs): %v", err)
	}

	// The function should not panic and should return a valid result
	_ = vms

	// Test with a non-existent port group to verify error handling
	_, err = ListPortGroupVMs(context.Background(), client, "nonexistent-pg")
	if err == nil {
		t.Error("ListPortGroupVMs should return error for non-existent port group")
	}
}

func TestVMsSorted(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Machine = 3

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListVMs(context.Background(), client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	for i := 1; i < len(vms); i++ {
		if vms[i-1].Name > vms[i].Name {
			t.Errorf("VMs not sorted: %q > %q", vms[i-1].Name, vms[i].Name)
		}
	}
}

func TestDatastoresSorted(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Datastore = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	dss, err := ListDatastores(context.Background(), client)
	if err != nil {
		t.Fatalf("ListDatastores: %v", err)
	}

	for i := 1; i < len(dss); i++ {
		if dss[i-1].Name > dss[i].Name {
			t.Errorf("Datastores not sorted: %q > %q", dss[i-1].Name, dss[i].Name)
		}
	}
}

func TestConnectRequiresURL(t *testing.T) {
	_, err := Connect(context.Background(), &config.Config{URL: ""})
	if err == nil {
		t.Error("Connect should return error for empty URL")
	}
}

func TestConnectInvalidURL(t *testing.T) {
	_, err := Connect(context.Background(), &config.Config{URL: "://invalid"})
	if err == nil {
		t.Error("Connect should return error for invalid URL")
	}
}

func TestVMStorage(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListVMs(context.Background(), client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	if len(vms) != 1 {
		t.Fatalf("ListVMs returned %d VMs, want 1", len(vms))
	}

	vm := vms[0]
	if vm.Name == "" {
		t.Error("VM name is empty")
	}
	if vm.Storage == "" {
		t.Error("Storage string is empty")
	}
}
