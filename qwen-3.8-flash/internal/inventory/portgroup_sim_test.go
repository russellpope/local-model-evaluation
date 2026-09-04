package inventory

import (
	"context"
	"sort"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// All VMs in the model attach their only NIC to DC0_DVPG0, so the lookup
// must return exactly that set.
func TestVMsOnDistributedPortgroup(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		want, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(want) == 0 {
			t.Fatal("model has no VMs")
		}

		got, err := VMsOnPortgroup(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("VMsOnPortgroup: %v", err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %d VMs on DC0_DVPG0, want %d", len(got), len(want))
		}
		for i := range got {
			if got[i].Name != want[i].Name {
				t.Errorf("VM set mismatch at %d: got %q want %q", i, got[i].Name, want[i].Name)
			}
		}

		// The second dv portgroup exists but nothing is attached to it.
		empty, err := VMsOnPortgroup(ctx, c, "DC0_DVPG1")
		if err != nil {
			t.Fatalf("VMsOnPortgroup(DVPG1): %v", err)
		}
		if len(empty) != 0 {
			t.Errorf("DC0_DVPG1 has %d VMs, want 0", len(empty))
		}

		// Unknown portgroup names must produce an error, not an empty list.
		if _, err := VMsOnPortgroup(ctx, c, "does-not-exist"); err == nil {
			t.Error("expected error for unknown portgroup")
		}
	}, newVPXModel())
}

// Attach a VM NIC to the model's standard "VM Network" portgroup through the
// API, then assert the lookup finds exactly that VM.
func TestVMsOnStandardPortgroup(t *testing.T) {
	const pgName = "VM Network"
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		var machines []mo.VirtualMachine
		retrieveAll(t, ctx, c, "VirtualMachine", []string{"name", "summary", "config.hardware.device"}, &machines)
		names := make([]string, len(machines))
		for i, m := range machines {
			names[i] = m.Name
		}
		sort.Strings(names)
		var target *mo.VirtualMachine
		for i := range machines {
			if machines[i].Name == names[0] {
				target = &machines[i]
			}
		}
		if target == nil {
			t.Fatal("no target VM found")
		}

		card := &types.VirtualE1000e{
			VirtualEthernetCard: types.VirtualEthernetCard{
				VirtualDevice: types.VirtualDevice{
					Backing: &types.VirtualEthernetCardNetworkBackingInfo{
						VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
							DeviceName: pgName,
						},
					},
					Connectable: &types.VirtualDeviceConnectInfo{StartConnected: true, Connected: true},
				},
			},
		}
		vmObj := object.NewVirtualMachine(c, target.Self)
		if err := vmObj.AddDevice(ctx, card); err != nil {
			t.Fatalf("AddDevice: %v", err)
		}

		got, err := VMsOnPortgroup(ctx, c, pgName)
		if err != nil {
			t.Fatalf("VMsOnPortgroup(%s): %v", pgName, err)
		}
		if len(got) != 1 || got[0].Name != target.Name {
			t.Fatalf("standard portgroup VMs = %v, want exactly [%s]", got, target.Name)
		}

		// The standard portgroup must also appear in the switch listing.
		rows, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		found := false
		for _, r := range rows {
			if r.Portgroup == pgName && r.SwitchType == "standard" {
				found = true
				if r.Switch != "vSwitch0" {
					t.Errorf("%s on switch %q, want vSwitch0", pgName, r.Switch)
				}
				if r.LACP != "N/A" {
					t.Errorf("%s LACP = %q, want N/A", pgName, r.LACP)
				}
			}
		}
		if !found {
			t.Errorf("portgroup %s missing from switch listing", pgName)
		}
	}, newVPXModel())
}
