package inventory

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/simulator/vpx"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// testModel builds a deterministic simulator: a single datacenter with one
// cluster host, two VMs, one distributed port group, and one datastore. This
// keeps the VM count and port-group set stable across runs.
func testModel() *simulator.Model {
	return &simulator.Model{
		ServiceContent: vpx.ServiceContent,
		RootFolder:     vpx.RootFolder,
		Autostart:      true,
		Datacenter:     1,
		Cluster:        1,
		ClusterHost:    1,
		Machine:        2,
		Portgroup:      1,
		Datastore:      1,
	}
}

// withSimulator runs f against an in-process vCenter built from model (VPX by
// default) and fails the test if the model fails to start.
func withSimulator(t *testing.T, model *simulator.Model, f func(context.Context, *vim25.Client)) {
	t.Helper()
	if model == nil {
		model = simulator.VPX()
	}
	if err := model.Run(func(ctx context.Context, client *vim25.Client) error {
		f(ctx, client)
		return nil
	}); err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}

func TestListVMs(t *testing.T) {
	withSimulator(t, testModel(), func(ctx context.Context, client *vim25.Client) {
		vms, err := ListVMs(ctx, client)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) != 2 {
			t.Fatalf("want 2 VMs, got %d", len(vms))
		}
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Errorf("VMs not sorted by name: %q then %q", vms[i-1].Name, vms[i].Name)
			}
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Errorf("VM with empty name")
			}
			if vm.VCPUs <= 0 {
				t.Errorf("VM %q: vCPU = %d, want > 0", vm.Name, vm.VCPUs)
			}
			if vm.RAMGB <= 0 {
				t.Errorf("VM %q: RAM = %f GB, want > 0", vm.Name, vm.RAMGB)
			}
			if vm.StorageBytes < 0 {
				t.Errorf("VM %q: negative storage %d", vm.Name, vm.StorageBytes)
			}
		}
	})
}

func TestListDatastores(t *testing.T) {
	validTypes := map[string]bool{"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true}

	withSimulator(t, nil, func(ctx context.Context, client *vim25.Client) {
		dss, err := ListDatastores(ctx, client)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(dss) == 0 {
			t.Fatal("want at least one datastore")
		}
		for _, ds := range dss {
			if ds.Name == "" {
				t.Errorf("datastore with empty name")
			}
			if ds.AvailableBytes > ds.CapacityBytes {
				t.Errorf("datastore %q: available %d > capacity %d", ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}
			if usedAndAvailable := ds.UsedBytes + ds.AvailableBytes; usedAndAvailable != ds.CapacityBytes {
				t.Errorf("datastore %q: used+available %d != capacity %d", ds.Name, usedAndAvailable, ds.CapacityBytes)
			}
			if !validTypes[ds.Type] {
				t.Errorf("datastore %q: transport type %q not in {FC,iSCSI,NVMe,NFS,unknown}", ds.Name, ds.Type)
			}
		}
	})
}

func TestListSwitches(t *testing.T) {
	validLACP := map[string]bool{"enabled": true, "disabled": true, "N/A": true}

	withSimulator(t, nil, func(ctx context.Context, client *vim25.Client) {
		rows, err := ListSwitches(ctx, client)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("want at least one switch port-group row")
		}
		hasStandard, hasDistributed := false, false
		for _, r := range rows {
			if r.Switch == "" {
				t.Errorf("row with empty switch name")
			}
			if r.SwitchType != "standard" && r.SwitchType != "distributed" {
				t.Errorf("row %q: switch type %q invalid", r.PortGroup, r.SwitchType)
			}
			if r.PortGroup == "" {
				t.Errorf("row for switch %q: empty port group", r.Switch)
			}
			if lo, hi, err := parseVLAN(r.VLAN); err != nil {
				t.Errorf("row %q: VLAN %q does not parse: %v", r.PortGroup, r.VLAN, err)
			} else if lo > hi {
				t.Errorf("row %q: VLAN range %d > %d", r.PortGroup, lo, hi)
			}
			if r.UsedPorts > r.Ports {
				t.Errorf("row %q/%q: used ports %d > total %d", r.Switch, r.PortGroup, r.UsedPorts, r.Ports)
			}
			if !validLACP[r.LACP] {
				t.Errorf("row %q: LACP %q invalid", r.PortGroup, r.LACP)
			}
			if r.SwitchType == "standard" && r.LACP != "N/A" {
				t.Errorf("standard switch %q: LACP = %q, want N/A", r.Switch, r.LACP)
			}
			if r.SwitchType == "distributed" && r.LACP == "N/A" {
				t.Errorf("distributed switch %q: LACP = N/A, want enabled/disabled", r.Switch)
			}
			if r.SwitchType == "standard" {
				hasStandard = true
			} else {
				hasDistributed = true
			}
		}
		if !hasStandard {
			t.Error("no standard switch rows returned")
		}
		if !hasDistributed {
			t.Error("no distributed switch rows returned")
		}
	})
}

func TestVMsForPortGroup(t *testing.T) {
	withSimulator(t, testModel(), func(ctx context.Context, client *vim25.Client) {
		vms, err := ListVMs(ctx, client)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) == 0 {
			t.Fatal("no VMs in simulator")
		}
		want := make(map[string]bool, len(vms))
		for _, vm := range vms {
			want[vm.Name] = true
		}

		pgNames := discoverDVSPGs(ctx, client, t)
		if len(pgNames) == 0 {
			t.Fatal("no distributed port groups found")
		}

		// At least one distributed port group must resolve to exactly the full
		// set of virtual machines.
		found := false
		for _, name := range pgNames {
			got, err := VMsForPortGroup(ctx, client, name)
			if err != nil {
				continue
			}
			if equalVMSets(got, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no distributed port group resolved to the full VM set %v", want)
		}
	})
}

// TestVMsForStandardPortGroup exercises the standard (host vSwitch) branch of the
// port-group lookup. A model with Portgroup == 0 backs each VM's single NIC on
// the default standard "VM Network" port group instead of a distributed one.
func TestVMsForStandardPortGroup(t *testing.T) {
	withSimulator(t, &simulator.Model{
		ServiceContent: vpx.ServiceContent,
		RootFolder:     vpx.RootFolder,
		Autostart:      true,
		Datacenter:     1,
		Cluster:        1,
		ClusterHost:    1,
		Machine:        2,
		Portgroup:      0,
		Datastore:      1,
	}, func(ctx context.Context, client *vim25.Client) {
		vms, err := ListVMs(ctx, client)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) == 0 {
			t.Fatal("no VMs in simulator")
		}
		want := make(map[string]bool, len(vms))
		for _, vm := range vms {
			want[vm.Name] = true
		}

		got, err := VMsForPortGroup(ctx, client, "VM Network")
		if err != nil {
			t.Fatalf("VMsForPortGroup(VM Network): %v", err)
		}
		if !equalVMSets(got, want) {
			t.Errorf("standard port group VM Network = %v, want %v", got, want)
		}
	})
}

// discoverDVSPGs returns the names of every distributed port group.
func discoverDVSPGs(ctx context.Context, client *vim25.Client, t *testing.T) []string {
	t.Helper()

	dvsList, err := distributedSwitches(ctx, client)
	if err != nil {
		t.Fatalf("distributed switches: %v", err)
	}

	pc := property.DefaultCollector(client)
	var names []string
	for _, dvs := range dvsList {
		for _, pgRef := range dvs.Portgroup {
			var pg mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(ctx, pgRef, nil, &pg); err != nil {
				t.Fatalf("reading distributed port group: %v", err)
			}
			if pg.Config.Name != "" {
				names = append(names, pg.Config.Name)
			}
		}
	}
	return names
}

func equalVMSets(got []PortGroupVM, want map[string]bool) bool {
	set := make(map[string]bool, len(got))
	for _, v := range got {
		set[v.Name] = true
	}
	if len(set) != len(want) {
		return false
	}
	for k := range want {
		if !set[k] {
			return false
		}
	}
	return true
}

// parseVLAN parses a VLAN cell produced by the vswitches formatter. It returns
// the low/high VLAN ids and an error when the cell is not a valid value.
func parseVLAN(s string) (int, int, error) {
	switch s {
	case "none", "":
		return 0, 0, nil
	case "trunk":
		return 0, 4095, nil
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil {
			return 0, 0, err1
		}
		if err2 != nil {
			return 0, 0, err2
		}
		return lo, hi, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, err
	}
	return v, v, nil
}
