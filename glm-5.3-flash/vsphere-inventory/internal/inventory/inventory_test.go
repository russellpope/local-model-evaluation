package inventory

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
)

// simClient boots an in-process vCenter (simulator.VPX model, hermetic — no
// external vcsim binary) and returns a connected client. The model is
// configured to fixed counts so the assertions below are deterministic:
//
//   - 1 datacenter, 1 cluster with 2 hosts, no standalone hosts
//   - 2 datastores (LocalDS_0, LocalDS_1)
//   - 2 VMs in the cluster's resource pool, each with one vNIC on the first DVPG
//   - 1 DVS with 2 distributed port groups (DC0_DVPG0, DC0_DVPG1)
func simClient(t *testing.T) *govmomi.Client {
	t.Helper()
	model := simulator.VPX()
	model.Host = 0
	model.ClusterHost = 2
	model.Datastore = 2
	model.Machine = 2
	model.Portgroup = 2
	model.App = 0
	model.Pod = 0
	model.Folder = 0
	if err := model.Create(); err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	t.Cleanup(model.Remove)

	server := model.Service.NewServer()
	t.Cleanup(server.Close)

	client, err := govmomi.NewClient(context.Background(), server.URL, true)
	if err != nil {
		t.Fatalf("connect to simulator: %v", err)
	}
	t.Cleanup(func() { _ = client.Logout(context.Background()) })
	return client
}

func TestListVMs(t *testing.T) {
	client := simClient(t)

	got, err := ListVMs(context.Background(), client.Client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	const wantCount = 2 // model.Machine VMs in the cluster resource pool
	if len(got) != wantCount {
		t.Fatalf("ListVMs returned %d VMs, want %d (names: %v)", len(got), wantCount, vmNames(got))
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Name < got[j].Name }) {
		t.Error("ListVMs result is not sorted by name")
	}
	for _, vm := range got {
		if vm.Name == "" {
			t.Error("VM with empty name")
		}
		if vm.VCPU <= 0 {
			t.Errorf("VM %q: VCPU = %d, want > 0", vm.Name, vm.VCPU)
		}
		if vm.RAMMib <= 0 {
			t.Errorf("VM %q: RAM = %d MiB, want > 0", vm.Name, vm.RAMMib)
		}
		if vm.CommittedBytes < 0 {
			t.Errorf("VM %q: committed storage = %d, want >= 0", vm.Name, vm.CommittedBytes)
		}
	}
}

func TestListDatastores(t *testing.T) {
	client := simClient(t)

	got, err := ListDatastores(context.Background(), client.Client)
	if err != nil {
		t.Fatalf("ListDatastores: %v", err)
	}

	const wantCount = 2
	if len(got) != wantCount {
		t.Fatalf("ListDatastores returned %d datastores, want %d", len(got), wantCount)
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Name < got[j].Name }) {
		t.Error("ListDatastores result is not sorted by name")
	}

	allowed := map[string]bool{"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true}
	for _, ds := range got {
		if ds.Name == "" {
			t.Error("datastore with empty name")
		}
		if !allowed[ds.Type] {
			t.Errorf("datastore %q: TYPE = %q, want one of FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Type)
		}
		if ds.CapacityBytes <= 0 {
			t.Errorf("datastore %q: capacity = %d, want > 0", ds.Name, ds.CapacityBytes)
		}
		if ds.AvailableBytes < 0 || ds.AvailableBytes > ds.CapacityBytes {
			t.Errorf("datastore %q: available %d out of range [0, %d]",
				ds.Name, ds.AvailableBytes, ds.CapacityBytes)
		}
		if ds.UsedBytes+ds.AvailableBytes != ds.CapacityBytes {
			t.Errorf("datastore %q: used(%d) + available(%d) != capacity(%d)",
				ds.Name, ds.UsedBytes, ds.AvailableBytes, ds.CapacityBytes)
		}
	}
}

func TestListSwitches(t *testing.T) {
	client := simClient(t)

	got, err := ListSwitches(context.Background(), client.Client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("ListSwitches returned %d switches, want at least one standard and one distributed", len(got))
	}

	var haveStandard, haveDistributed bool
	validLACP := map[string]bool{LACPEnabled: true, LACPDisabled: true, LACPNA: true}

	for _, sw := range got {
		if sw.Name == "" {
			t.Error("switch with empty name")
		}
		if !validLACP[sw.LACP] {
			t.Errorf("switch %q: LACP = %q, want enabled/disabled/N-A", sw.Name, sw.LACP)
		}
		if sw.Used > sw.Ports {
			t.Errorf("switch %q: used ports %d > total %d", sw.Name, sw.Used, sw.Ports)
		}
		for _, pg := range sw.PortGroups {
			assertValidVLAN(t, pg.VLAN)
			if pg.Used > pg.Ports {
				t.Errorf("port group %q: used ports %d > total %d", pg.Name, pg.Used, pg.Ports)
			}
		}
		if sw.Used > 0 && sw.Used > sw.Ports {
			t.Errorf("switch %q: used %d > ports %d", sw.Name, sw.Used, sw.Ports)
		}

		switch sw.Type {
		case SwitchTypeStandard:
			haveStandard = true
			if sw.LACP != LACPNA {
				t.Errorf("standard switch %q: LACP = %q, want %q", sw.Name, sw.LACP, LACPNA)
			}
			if sw.Ports <= 0 {
				t.Errorf("standard switch %q: ports = %d, want > 0", sw.Name, sw.Ports)
			}
			if sw.Used <= 0 {
				t.Errorf("standard switch %q: used = %d, want > 0 (simulator reports 6 busy ports)", sw.Name, sw.Used)
			}
		case SwitchTypeDistributed:
			haveDistributed = true
			if len(sw.PortGroups) < 2 { // DVUplinks + 2 model port groups
				t.Errorf("distributed switch %q: %d port groups, want >= 2", sw.Name, len(sw.PortGroups))
			}
		default:
			t.Errorf("switch %q: unknown type %q", sw.Name, sw.Type)
		}
	}
	if !haveStandard {
		t.Error("no standard switch in result (want vSwitch0)")
	}
	if !haveDistributed {
		t.Error("no distributed switch in result (want DC0_DVS0)")
	}
}

// assertValidVLAN accepts a single ID, a trunk range list, a pvlan marker, or
// the "unknown" degradation — everything else is a formatting bug.
func assertValidVLAN(t *testing.T, vlan string) {
	t.Helper()
	if _, err := strconv.Atoi(vlan); err == nil {
		return
	}
	for _, prefix := range []string{"trunk(", "pvlan("} {
		if strings.HasPrefix(vlan, prefix) && strings.HasSuffix(vlan, ")") {
			return
		}
	}
	if vlan == "unknown" || vlan == "trunk" {
		return
	}
	t.Errorf("VLAN value %q does not parse", vlan)
}

func TestVMsOnPortgroup(t *testing.T) {
	client := simClient(t)
	ctx := context.Background()

	allVMs, err := ListVMs(ctx, client.Client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	if len(allVMs) == 0 {
		t.Fatal("model created no VMs")
	}

	// Every model VM's single vNIC is backed by the first DVPG.
	onFirst, err := VMsOnPortgroup(ctx, client.Client, "DC0_DVPG0")
	if err != nil {
		t.Fatalf("VMsOnPortgroup(DC0_DVPG0): %v", err)
	}
	if !sameVMNames(onFirst, allVMs) {
		t.Errorf("DC0_DVPG0 = %v, want all model VMs %v", vmNames(onFirst), vmNames(allVMs))
	}

	// The second DVPG has no VMs attached.
	onSecond, err := VMsOnPortgroup(ctx, client.Client, "DC0_DVPG1")
	if err != nil {
		t.Fatalf("VMsOnPortgroup(DC0_DVPG1): %v", err)
	}
	if len(onSecond) != 0 {
		t.Errorf("DC0_DVPG1 = %v, want empty", vmNames(onSecond))
	}

	// Standard port groups work too: attach a second vNIC on the first VM
	// to the standard "VM Network" port group and look it up.
	first, err := findVMByName(ctx, client, allVMs[0].Name)
	if err != nil {
		t.Fatalf("find VM %q: %v", allVMs[0].Name, err)
	}
	if err := attachToStandardPortgroup(ctx, first, "VM Network"); err != nil {
		t.Fatalf("attach %q to VM Network: %v", allVMs[0].Name, err)
	}
	onStd, err := VMsOnPortgroup(ctx, client.Client, "VM Network")
	if err != nil {
		t.Fatalf("VMsOnPortgroup(VM Network): %v", err)
	}
	if len(onStd) != 1 || onStd[0].Name != allVMs[0].Name {
		t.Errorf("VM Network = %v, want exactly [%s]", vmNames(onStd), allVMs[0].Name)
	}

	// Unknown port groups fail with a clear error.
	if _, err := VMsOnPortgroup(ctx, client.Client, "no-such-portgroup"); err == nil {
		t.Error("VMsOnPortgroup(no-such-portgroup) should fail")
	}
}

func findVMByName(ctx context.Context, client *govmomi.Client, name string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(client.Client, false)
	dc, err := finder.Datacenter(ctx, "*")
	if err != nil {
		return nil, err
	}
	finder.SetDatacenter(dc)
	return finder.VirtualMachine(ctx, name)
}

// attachToStandardPortgroup adds a vNIC backed by the named standard port
// group to the VM.
func attachToStandardPortgroup(ctx context.Context, vm *object.VirtualMachine, pg string) error {
	var devices object.VirtualDeviceList
	nic, err := devices.CreateEthernetCard("e1000",
		&types.VirtualEthernetCardNetworkBackingInfo{
			VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{DeviceName: pg},
		})
	if err != nil {
		return err
	}
	spec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{
			&types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationAdd,
				Device:    nic,
			},
		},
	}
	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return err
	}
	return task.Wait(ctx)
}

func vmNames(vms []VMInfo) []string {
	out := make([]string, 0, len(vms))
	for _, vm := range vms {
		out = append(out, vm.Name)
	}
	sort.Strings(out)
	return out
}

func sameVMNames(a, b []VMInfo) bool {
	an, bn := vmNames(a), vmNames(b)
	if len(an) != len(bn) {
		return false
	}
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	return true
}
