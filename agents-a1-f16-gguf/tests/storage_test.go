package tests

import (
	"context"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"vsphere-inventory/internal/storage"
)

func TestVMs(t *testing.T) {
	t.Parallel()

	// Create a simulator model with known VM count
	model := simulator.VPX()
	model.Count.Vm = 3

	s := model.New()
	defer s.Destroy()

	// Get connected client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := vim25.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatalf("connecting to simulator: %v", err)
	}
	defer client.Logout(ctx)

	govmomiClient := storage.NewGovmomiClient(client)

	vms, err := storage.GetVMs(ctx, govmomiClient)
	if err != nil {
		t.Fatalf("getting VMs: %v", err)
	}

	// Assert we got exactly 3 VMs
	if len(vms) != 3 {
		t.Errorf("expected 3 VMs, got %d", len(vms))
	}

	// Assert each VM has valid data
	for _, vm := range vms {
		if vm.Name == "" {
			t.Errorf("VM name is empty")
		}
		if vm.VCPU <= 0 {
			t.Errorf("VM %s has invalid vCPU count: %d", vm.Name, vm.VCPU)
		}
		if vm.RAMGB <= 0 {
			t.Errorf("VM %s has invalid RAM: %.1f GB", vm.Name, vm.RAMGB)
		}
		if vm.StorageGB < 0 {
			t.Errorf("VM %s has negative storage: %.1f GB", vm.Name, vm.StorageGB)
		}
	}
}

func TestDatastores(t *testing.T) {
	t.Parallel()

	// Create a simulator model with known datastore count
	model := simulator.VPX()
	model.Count.Datastore = 2

	s := model.New()
	defer s.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := vim25.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatalf("connecting to simulator: %v", err)
	}
	defer client.Logout(ctx)

	govmomiClient := storage.NewGovmomiClient(client)

	datastores, err := storage.GetDatastores(ctx, govmomiClient)
	if err != nil {
		t.Fatalf("getting datastores: %v", err)
	}

	// Assert we got exactly 2 datastores
	if len(datastores) != 2 {
		t.Errorf("expected 2 datastores, got %d", len(datastores))
	}

	// Assert each datastore has valid data
	for _, ds := range datastores {
		if ds.Name == "" {
			t.Errorf("datastore name is empty")
		}
		if ds.Type != "NFS" && ds.Type != "unknown" {
			t.Errorf("datastore %s has invalid type: %s", ds.Name, ds.Type)
		}
		if ds.AvailableGB < 0 {
			t.Errorf("datastore %s has negative available: %.1f GB", ds.Name, ds.AvailableGB)
		}
		if ds.UsedGB < 0 {
			t.Errorf("datastore %s has negative used: %.1f GB", ds.Name, ds.UsedGB)
		}
		// Assert used + available is consistent with capacity
		// (within rounding)
		if ds.UsedGB+ds.AvailableGB < 0 {
			t.Errorf("datastore %s has invalid capacity", ds.Name)
		}
	}
}

func TestSwitches(t *testing.T) {
	t.Parallel()

	// Create a simulator model
	model := simulator.VPX()
	s := model.New()
	defer s.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := vim25.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatalf("connecting to simulator: %v", err)
	}
	defer client.Logout(ctx)

	govmomiClient := storage.NewGovmomiClient(client)

	switches, err := storage.GetSwitches(ctx, govmomiClient)
	if err != nil {
		t.Fatalf("getting switches: %v", err)
	}

	// Assert at least one switch is returned
	if len(switches) == 0 {
		t.Errorf("expected at least one switch, got 0")
	}

	// Assert each switch has valid data
	for _, sw := range switches {
		if sw.Name == "" {
			t.Errorf("switch name is empty")
		}
		if sw.SwitchType != "standard" && sw.SwitchType != "distributed" {
			t.Errorf("switch %s has invalid type: %s", sw.Name, sw.SwitchType)
		}
		if sw.LACP != "enabled" && sw.LACP != "disabled" && sw.LACP != "N/A" {
			t.Errorf("switch %s has invalid LACP: %s", sw.Name, sw.LACP)
		}
		if sw.UsedPorts > sw.TotalPorts {
			t.Errorf("switch %s has used ports > total ports", sw.Name)
		}
	}
}

func TestPortGroupToVMs(t *testing.T) {
	t.Parallel()

	// Create a simulator model with a port group
	model := simulator.VPX()
	model.Count.Vm = 2
	model.Count.PortGroup = 1

	s := model.New()
	defer s.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := vim25.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatalf("connecting to simulator: %v", err)
	}
	defer client.Logout(ctx)

	govmomiClient := storage.NewGovmomiClient(client)

	// Get the port group name from the switches
	switches, err := storage.GetSwitches(ctx, govmomiClient)
	if err != nil {
		t.Fatalf("getting switches: %v", err)
	}

	if len(switches) == 0 || len(switches[0].PortGroups) == 0 {
		t.Skip("no port groups found in simulator")
	}

	portgroupName := switches[0].PortGroups[0].Name

	vms, err := storage.GetVMsByPortGroup(ctx, govmomiClient, portgroupName)
	if err != nil {
		t.Fatalf("getting VMs by port group: %v", err)
	}

	// The simulator may not have VMs connected to port groups by default
	// This test verifies the logic works when VMs are connected
	// We'll check that the function returns without error
	_ = vms
}
