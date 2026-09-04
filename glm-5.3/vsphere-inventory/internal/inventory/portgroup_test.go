package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// TestPortGroupVMsDistributed: with the default model every VM NIC is
// backed by the first distributed port group, so the lookup must return
// exactly the full VM set.
func TestPortGroupVMsDistributed(t *testing.T) {
	const machinesPerPool = 2
	const wantVMs = 2 * machinesPerPool

	runSimulator(t, func(m *simulator.Model) {
		m.Machine = machinesPerPool
	}, func(ctx context.Context, c *govmomi.Client) {
		vms, err := ListPortGroupVMs(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("ListPortGroupVMs(DC0_DVPG0): %v", err)
		}
		if len(vms) != wantVMs {
			t.Fatalf("got %d VMs, want %d: %v", len(vms), wantVMs, namesOf(vms))
		}

		want := map[string]bool{
			"DC0_H0_VM0":     true,
			"DC0_H0_VM1":     true,
			"DC0_C0_RP0_VM0": true,
			"DC0_C0_RP0_VM1": true,
		}
		for _, vm := range vms {
			if !want[vm.Name] {
				t.Errorf("unexpected VM %q in port group lookup", vm.Name)
			}
			delete(want, vm.Name)
		}
		for name := range want {
			t.Errorf("expected VM %q attached to DC0_DVPG0", name)
		}
	})
}

// TestPortGroupVMsStandard reconfigures one VM's NIC onto the standard
// "VM Network" port group and asserts the lookup finds exactly that VM.
func TestPortGroupVMsStandard(t *testing.T) {
	runSimulator(t, func(m *simulator.Model) {
		m.Machine = 1
	}, func(ctx context.Context, c *govmomi.Client) {
		const vmName = "DC0_H0_VM0"

		vmRef, err := findVMByName(ctx, c, vmName)
		if err != nil {
			t.Fatalf("find %s: %v", vmName, err)
		}
		vm := object.NewVirtualMachine(c.Client, vmRef)

		devices, err := vm.Device(ctx)
		if err != nil {
			t.Fatalf("list devices: %v", err)
		}
		var nic types.BaseVirtualEthernetCard
		for _, d := range devices {
			if card, ok := d.(types.BaseVirtualEthernetCard); ok {
				nic = card
				break
			}
		}
		if nic == nil {
			t.Fatal("VM has no ethernet card")
		}
		nic.GetVirtualEthernetCard().Backing = &types.VirtualEthernetCardNetworkBackingInfo{
			VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
				DeviceName: "VM Network",
			},
		}
		spec := types.VirtualMachineConfigSpec{
			DeviceChange: []types.BaseVirtualDeviceConfigSpec{&types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
				Device:    nic.(types.BaseVirtualDevice),
			}},
		}
		task, err := vm.Reconfigure(ctx, spec)
		if err != nil {
			t.Fatalf("reconfigure: %v", err)
		}
		if err := task.Wait(ctx); err != nil {
			t.Fatalf("reconfigure task: %v", err)
		}

		vms, err := ListPortGroupVMs(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("ListPortGroupVMs(VM Network): %v", err)
		}
		if len(vms) != 1 || vms[0].Name != vmName {
			t.Fatalf("got %v, want exactly [%s]", namesOf(vms), vmName)
		}
	})
}

// TestPortGroupVMsNotFound asserts a clear error for an unknown port group.
func TestPortGroupVMsNotFound(t *testing.T) {
	runSimulator(t, nil, func(ctx context.Context, c *govmomi.Client) {
		_, err := ListPortGroupVMs(ctx, c, "does-not-exist")
		if err == nil {
			t.Fatal("expected error for unknown port group")
		}
		want := fmt.Sprintf("port group %q not found", "does-not-exist")
		if len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
			t.Fatalf("error %q does not mention missing port group", err)
		}
	})
}

func namesOf(vms []VMInfo) []string {
	out := make([]string, 0, len(vms))
	for _, vm := range vms {
		out = append(out, vm.Name)
	}
	return out
}

// findVMByName locates a VM's reference by name using a container view.
func findVMByName(ctx context.Context, c *govmomi.Client, name string) (types.ManagedObjectReference, error) {
	v, err := newContainerView(ctx, c, []string{"VirtualMachine"})
	if err != nil {
		return types.ManagedObjectReference{}, err
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name"}, &vms); err != nil {
		return types.ManagedObjectReference{}, err
	}
	for _, vm := range vms {
		if vm.Name == name {
			return vm.Reference(), nil
		}
	}
	return types.ManagedObjectReference{}, fmt.Errorf("VM %q not found", name)
}
