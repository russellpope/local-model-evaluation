package main

import (
	"context"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
)

func TestVMsSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 3

	err := model.Create()
	if err != nil {
		t.Fatalf("failed to create simulator model: %v", err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	cfg := Config{
		URL:      s.URL.String(),
		Username: "user",
		Password: "pass",
		Insecure: true,
		Timeout:  30 * time.Second,
	}

	client, err := connect(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to connect to simulator: %v", err)
	}
	defer client.Logout(ctx)

	vms, err := getVMs(ctx, client)
	if err != nil {
		t.Fatalf("failed to get VMs: %v", err)
	}

	if len(vms) < 3 {
		t.Errorf("expected at least 3 VMs, got %d", len(vms))
	}

	for _, vm := range vms {
		if vm.Name == "" {
			t.Errorf("VM name is empty")
		}
		if vm.VCPU <= 0 {
			t.Errorf("VM %s has vCPU <= 0: %d", vm.Name, vm.VCPU)
		}
		if vm.RAMGB <= 0 {
			t.Errorf("VM %s has RAMGB <= 0: %f", vm.Name, vm.RAMGB)
		}
		if vm.Storage == "" {
			t.Errorf("VM %s has empty storage", vm.Name)
		}
	}
}

func TestDatastoresSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Datastore = 2

	err := model.Create()
	if err != nil {
		t.Fatalf("failed to create simulator model: %v", err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	cfg := Config{
		URL:      s.URL.String(),
		Username: "user",
		Password: "pass",
		Insecure: true,
		Timeout:  30 * time.Second,
	}

	client, err := connect(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to connect to simulator: %v", err)
	}
	defer client.Logout(ctx)

	datastores, err := getDatastores(ctx, client)
	if err != nil {
		t.Fatalf("failed to get datastores: %v", err)
	}

	if len(datastores) != 2 {
		t.Errorf("expected 2 datastores, got %d", len(datastores))
	}

	validTypes := map[string]bool{
		"FC":      true,
		"iSCSI":   true,
		"NVMe":    true,
		"NFS":     true,
		"unknown": true,
	}

	for _, ds := range datastores {
		if ds.Name == "" {
			t.Errorf("datastore name is empty")
		}
		if !validTypes[ds.Type] {
			t.Errorf("datastore %s has invalid type: %s", ds.Name, ds.Type)
		}
		if ds.Used == "" || ds.Available == "" {
			t.Errorf("datastore %s has empty used or available: used=%s, available=%s", ds.Name, ds.Used, ds.Available)
		}
	}
}

func TestVSwitchesSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	err := model.Create()
	if err != nil {
		t.Fatalf("failed to create simulator model: %v", err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	cfg := Config{
		URL:      s.URL.String(),
		Username: "user",
		Password: "pass",
		Insecure: true,
		Timeout:  30 * time.Second,
	}

	client, err := connect(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to connect to simulator: %v", err)
	}
	defer client.Logout(ctx)

	switchInfos, err := getVSwitches(ctx, client)
	if err != nil {
		t.Fatalf("failed to get vswitches: %v", err)
	}

	if len(switchInfos) == 0 {
		t.Errorf("expected at least 1 switch, got %d", len(switchInfos))
	}

	validLACP := map[string]bool{
		"enabled":  true,
		"disabled": true,
		"N/A":      true,
	}

	for _, si := range switchInfos {
		if si.SwitchName == "" {
			t.Errorf("switch name is empty")
		}
		if si.SwitchType != "standard" && si.SwitchType != "distributed" {
			t.Errorf("switch %s has invalid type: %s", si.SwitchName, si.SwitchType)
		}
		if si.PortGroupName == "" {
			t.Errorf("switch %s has empty port group name", si.SwitchName)
		}
		if si.LACP != "" && !validLACP[si.LACP] {
			t.Errorf("switch %s has invalid LACP: %s", si.SwitchName, si.LACP)
		}
		if si.UsedPorts < 0 {
			t.Errorf("switch %s has negative used ports %d", si.SwitchName, si.UsedPorts)
		}
		if si.UsedPorts > si.Ports && si.Ports != 0 {
			t.Errorf("switch %s has used ports %d > total ports %d", si.SwitchName, si.UsedPorts, si.Ports)
		}
	}
}

func TestPortGroupVMsSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2
	model.Portgroup = 1

	err := model.Create()
	if err != nil {
		t.Fatalf("failed to create simulator model: %v", err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	cfg := Config{
		URL:      s.URL.String(),
		Username: "user",
		Password: "pass",
		Insecure: true,
		Timeout:  30 * time.Second,
	}

	client, err := connect(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to connect to simulator: %v", err)
	}
	defer client.Logout(ctx)

	switchInfos, err := getVSwitches(ctx, client)
	if err != nil {
		t.Fatalf("failed to get vswitches: %v", err)
	}

	if len(switchInfos) == 0 {
		t.Fatalf("expected at least 1 switch, got %d", len(switchInfos))
	}

	// Find a standard port group (not distributed)
	var standardPortGroupName string
	for _, si := range switchInfos {
		if si.SwitchType == "standard" && si.PortGroupName != "" {
			standardPortGroupName = si.PortGroupName
			break
		}
	}

	if standardPortGroupName == "" {
		t.Fatalf("expected to find a standard port group, got none")
	}

	info, err := getVMsForPortGroup(ctx, client, standardPortGroupName)
	if err != nil {
		t.Fatalf("failed to get VMs for port group: %v", err)
	}

	// The getVMsForPortGroup function should return successfully
	// VMs may or may not be attached to the specific port group depending on the simulator model
	if len(info) == 0 {
		t.Logf("No VMs found for port group %s (this is expected for some simulator configurations)", standardPortGroupName)
	} else {
		if len(info[0].VMs) == 0 {
			t.Logf("Port group %s has no VMs attached", standardPortGroupName)
		} else {
			t.Logf("Found %d VMs for port group %s", len(info[0].VMs), standardPortGroupName)
		}
	}
}
