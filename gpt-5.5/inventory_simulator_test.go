package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func TestListVMsWithSimulator(t *testing.T) {
	withSimulator(t, func(ctx context.Context, inv *Inventory, count simulator.Model) {
		vms, err := inv.ListVMs(ctx)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) != count.Machine {
			t.Fatalf("len(vms) = %d, want %d", len(vms), count.Machine)
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Fatalf("vm has empty name: %#v", vm)
			}
			if vm.VCPU <= 0 {
				t.Fatalf("%s VCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.RAMBytes <= 0 {
				t.Fatalf("%s RAMBytes = %d, want > 0", vm.Name, vm.RAMBytes)
			}
			if vm.StorageBytes < 0 {
				t.Fatalf("%s StorageBytes = %d, want >= 0", vm.Name, vm.StorageBytes)
			}
		}
		if !sort.SliceIsSorted(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name }) {
			t.Fatalf("vms are not sorted by name: %#v", vms)
		}
	})
}

func TestListDatastoresWithSimulator(t *testing.T) {
	withSimulator(t, func(ctx context.Context, inv *Inventory, count simulator.Model) {
		datastores, err := inv.ListDatastores(ctx)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(datastores) != 2 {
			t.Fatalf("len(datastores) = %d, want 2", len(datastores))
		}
		for _, ds := range datastores {
			if ds.Name == "" {
				t.Fatalf("datastore has empty name: %#v", ds)
			}
			if !validTransport(ds.Type) {
				t.Fatalf("%s Type = %q, want known enum", ds.Name, ds.Type)
			}
			if ds.AvailableBytes > ds.CapacityBytes {
				t.Fatalf("%s available %d exceeds capacity %d", ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}
			if ds.UsedBytes+ds.AvailableBytes != ds.CapacityBytes {
				t.Fatalf("%s used+available=%d, capacity=%d", ds.Name, ds.UsedBytes+ds.AvailableBytes, ds.CapacityBytes)
			}
		}
	})
}

func TestListSwitchesWithSimulator(t *testing.T) {
	withSimulator(t, func(ctx context.Context, inv *Inventory, count simulator.Model) {
		switches, err := inv.ListSwitches(ctx)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(switches) == 0 {
			t.Fatal("expected at least one switch row")
		}
		for _, sw := range switches {
			if sw.Switch == "" || sw.PortGroup == "" {
				t.Fatalf("switch row missing names: %#v", sw)
			}
			if sw.UsedPorts > sw.TotalPorts {
				t.Fatalf("%s/%s used ports %d > total %d", sw.Switch, sw.PortGroup, sw.UsedPorts, sw.TotalPorts)
			}
			if sw.LACP != "enabled" && sw.LACP != "disabled" && sw.LACP != "N/A" {
				t.Fatalf("unexpected LACP value %q", sw.LACP)
			}
			if _, err := parseVLAN(sw.VLAN); err != nil {
				t.Fatalf("VLAN %q did not parse: %v", sw.VLAN, err)
			}
		}
	})
}

func TestListVMsByPortGroupWithSimulator(t *testing.T) {
	withSimulator(t, func(ctx context.Context, inv *Inventory, count simulator.Model) {
		switches, err := inv.ListSwitches(ctx)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(switches) == 0 {
			t.Fatal("expected at least one switch row")
		}

		var portGroup string
		for _, sw := range switches {
			if sw.SwitchType == "distributed" && sw.PortGroup != "" {
				portGroup = sw.PortGroup
				break
			}
		}
		if portGroup == "" {
			t.Fatalf("no distributed portgroup was present in switch output: %#v", switches)
		}

		vms, err := inv.ListVMsByPortGroup(ctx, portGroup)
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", portGroup, err)
		}
		if len(vms) != count.Machine {
			t.Fatalf("len(vms attached to %q) = %d, want %d", portGroup, len(vms), count.Machine)
		}
		assertVMNames(t, portGroup, vms, "DC0_C0_RP0_VM0,DC0_C0_RP0_VM1,DC0_H0_VM0,DC0_H0_VM1")
	})
}

func TestListVMsByStandardPortGroupWithSimulator(t *testing.T) {
	withSimulatorPortgroups(t, 0, func(ctx context.Context, inv *Inventory, count simulator.Model) {
		switches, err := inv.ListSwitches(ctx)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if !hasSwitchPortGroup(switches, "standard", "VM Network") {
			t.Fatalf("standard VM Network was not present in switch output: %#v", switches)
		}

		vms, err := inv.ListVMsByPortGroup(ctx, "VM Network")
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", "VM Network", err)
		}
		if len(vms) != count.Machine {
			t.Fatalf("len(vms attached to VM Network) = %d, want %d", len(vms), count.Machine)
		}
		assertVMNames(t, "VM Network", vms, "DC0_C0_RP0_VM0,DC0_C0_RP0_VM1,DC0_H0_VM0,DC0_H0_VM1")

		empty, err := inv.ListVMsByPortGroup(ctx, "Management Network")
		if err != nil {
			t.Fatalf("ListVMsByPortGroup(%q): %v", "Management Network", err)
		}
		if len(empty) != 0 {
			t.Fatalf("len(vms attached to Management Network) = %d, want 0: %#v", len(empty), empty)
		}

		if _, err := inv.ListVMsByPortGroup(ctx, "NoSuchPG"); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("ListVMsByPortGroup(%q) error = %v, want not found", "NoSuchPG", err)
		}
	})
}

func withSimulator(t *testing.T, run func(context.Context, *Inventory, simulator.Model)) {
	withSimulatorPortgroups(t, 1, run)
}

func withSimulatorPortgroups(t *testing.T, portgroups int, run func(context.Context, *Inventory, simulator.Model)) {
	t.Helper()

	model := simulator.VPX()
	model.Machine = 2
	model.Datastore = 2
	model.Portgroup = portgroups
	if err := model.Create(); err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	defer model.Remove()

	server := model.Service.NewServer()
	defer server.Close()

	u, err := url.Parse(server.URL.String())
	if err != nil {
		t.Fatalf("parse simulator url: %v", err)
	}
	u.User = url.UserPassword("user", "pass")

	ctx := context.Background()
	client, err := NewClient(ctx, Config{URL: u.String(), Insecure: true})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Logout(ctx)

	run(ctx, NewInventory(client.Client), model.Count())
}

func hasSwitchPortGroup(switches []SwitchInfo, switchType, portGroup string) bool {
	for _, sw := range switches {
		if sw.SwitchType == switchType && sw.PortGroup == portGroup {
			return true
		}
	}
	return false
}

func assertVMNames(t *testing.T, portGroup string, vms []VMInfo, want string) {
	t.Helper()

	names := make([]string, 0, len(vms))
	for _, vm := range vms {
		names = append(names, vm.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != want {
		t.Fatalf("VMs attached to %q = %v, want %s", portGroup, names, want)
	}
}

func validTransport(t StorageTransport) bool {
	switch t {
	case TransportFC, TransportISCSI, TransportNVMe, TransportNFS, TransportUnknown:
		return true
	default:
		return false
	}
}

func parseVLAN(v string) (int, error) {
	if v == "" {
		return 0, fmt.Errorf("empty VLAN")
	}
	if v == "N/A" {
		return 0, nil
	}
	if strings.HasPrefix(v, "trunk") {
		ranges := strings.TrimSpace(strings.TrimPrefix(v, "trunk"))
		if ranges == "" {
			return 0, nil
		}
		for _, part := range strings.Split(ranges, ",") {
			if err := parseVLANRange(part); err != nil {
				return 0, err
			}
		}
		return 0, nil
	}
	if strings.HasPrefix(v, "private ") {
		return parseVLANID(strings.TrimPrefix(v, "private "))
	}
	return parseVLANID(v)
}

func parseVLANRange(v string) error {
	parts := strings.Split(v, "-")
	if len(parts) == 1 {
		_, err := parseVLANID(parts[0])
		return err
	}
	if len(parts) != 2 {
		return fmt.Errorf("invalid VLAN range %q", v)
	}
	start, err := parseVLANID(parts[0])
	if err != nil {
		return err
	}
	end, err := parseVLANID(parts[1])
	if err != nil {
		return err
	}
	if start > end {
		return fmt.Errorf("invalid VLAN range %q", v)
	}
	return nil
}

func parseVLANID(v string) (int, error) {
	id, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	if id < 0 || id > 4095 {
		return 0, fmt.Errorf("VLAN %d outside 0-4095", id)
	}
	return id, nil
}
