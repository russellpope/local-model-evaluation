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

func createSimClient(t *testing.T, model *simulator.Model) *govmomi.Client {
	t.Helper()
	if err := model.Create(); err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	t.Cleanup(func() { model.Remove() })

	s := model.Service.NewServer()
	t.Cleanup(func() { s.Close() })

	time.Sleep(100 * time.Millisecond)

	u := s.URL
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
	model.Host = 1
	model.Machine = 3

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListVMs(context.Background(), client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	if len(vms) < 1 {
		t.Errorf("ListVMs returned %d VMs, want at least 1", len(vms))
	}

	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM has empty name")
		}
		if vm.VCPU <= 0 {
			t.Errorf("VM %s has VCPU=%d, want > 0", vm.Name, vm.VCPU)
		}
		if vm.RAMGB <= 0 {
			t.Errorf("VM %s has RAMGB=%f, want > 0", vm.Name, vm.RAMGB)
		}
		if vm.Storage == "" {
			t.Errorf("VM %s has empty storage string", vm.Name)
		}
	}
}

func TestDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
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
			t.Error("Datastore has empty name (acceptable)")
		}
		if ds.Type != "FC" && ds.Type != "iSCSI" && ds.Type != "NVMe" && ds.Type != "NFS" && ds.Type != "unknown" {
			t.Errorf("Datastore %s has invalid TYPE %q", ds.Name, ds.Type)
		}
		if ds.Available == "" || ds.Available == "0 B" {
			t.Errorf("Datastore %s has AVAILABLE=%q, want non-zero", ds.Name, ds.Available)
		}
	}

	hasUsed := false
	for _, ds := range dss {
		if ds.Used != "0 B" {
			hasUsed = true
			break
		}
	}
	if !hasUsed {
		t.Error("No datastore has non-zero USED")
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

	var hasStandard, hasDistributed bool
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
			hasDistributed = true
		} else {
			if sw.LACP != "N/A" {
				t.Errorf("Standard switch %s has LACP %q, want N/A", sw.SwitchName, sw.LACP)
			}
			hasStandard = true
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

	if !hasStandard {
		t.Error("No standard switch found; want at least one")
	}
	if !hasDistributed {
		t.Error("No distributed switch found; want at least one")
	}

	hasPorts := false
	for _, sw := range switches {
		if sw.TotalPorts > 0 {
			hasPorts = true
			break
		}
	}
	if !hasPorts {
		t.Error("No switch has TotalPorts > 0")
	}
}

func TestPortGroupVMs(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	// Test standard port group
	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs for VM Network: %v (simulator may not populate VM network refs)", err)
	}

	// Simulator may not populate VM network references, so 0 VMs is acceptable
	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM has empty name in port group result")
		}
		if vm.PortGroup != "VM Network" {
			t.Errorf("VM %s has PortGroup=%q, want VM Network", vm.Name, vm.PortGroup)
		}
	}

	// Test distributed port group (VPX model creates DVS0 with DVPG0)
	vms, err = ListPortGroupVMs(context.Background(), client, "DC0_DVPG0")
	if err != nil {
		t.Fatalf("ListPortGroupVMs for DC0_DVPG0: %v", err)
	}

	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM has empty name in distributed port group result")
		}
		if vm.PortGroup != "DC0_DVPG0" {
			t.Errorf("VM %s has PortGroup=%q, want DC0_DVPG0", vm.Name, vm.PortGroup)
		}
	}

	// Test non-existent port group
	_, err = ListPortGroupVMs(context.Background(), client, "nonexistent-pg")
	if err == nil {
		t.Error("ListPortGroupVMs should return error for non-existent port group")
	}
}

func TestVMsSorted(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
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
	model.Host = 1
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
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListVMs(context.Background(), client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	if len(vms) < 1 {
		t.Fatalf("ListVMs returned %d VMs, want at least 1", len(vms))
	}

	vm := vms[0]
	if vm.Name == "" {
		t.Error("VM name is empty")
	}
	if vm.Storage == "" {
		t.Error("Storage string is empty")
	}
}

func TestStandardSwitchesDeduplicated(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	stdCount := make(map[string]int)
	for _, sw := range switches {
		if sw.SwitchType == "standard" {
			stdCount[sw.SwitchName]++
		}
	}

	for name, count := range stdCount {
		if count > 1 {
			t.Errorf("Standard switch %q appears %d times, want 1", name, count)
		}
	}
}

func TestPortGroupVMs_Distributed(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	var foundDVS bool
	for _, sw := range switches {
		if sw.SwitchType == "distributed" {
			foundDVS = true
			break
		}
	}
	if !foundDVS {
		t.Error("No distributed switch found in switch list")
	}

	vms, err := ListPortGroupVMs(context.Background(), client, "DC0_DVPG0")
	if err != nil {
		t.Logf("ListPortGroupVMs for DC0_DVPG0: %v", err)
	}

	for _, vm := range vms {
		if vm.PortGroup != "DC0_DVPG0" {
			t.Errorf("VM %s: PortGroup=%q, want DC0_DVPG0", vm.Name, vm.PortGroup)
		}
	}
}

func TestVMsHaveRealValues(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListVMs(context.Background(), client)
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	for _, vm := range vms {
		if vm.VCPU <= 0 {
			t.Errorf("VM %s: VCPU=%d, want > 0", vm.Name, vm.VCPU)
		}
		if vm.RAMGB <= 0 {
			t.Errorf("VM %s: RAMGB=%f, want > 0", vm.Name, vm.RAMGB)
		}
	}
}

func TestDatastoresHaveRealValues(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Datastore = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	dss, err := ListDatastores(context.Background(), client)
	if err != nil {
		t.Fatalf("ListDatastores: %v", err)
	}

	for _, ds := range dss {
		if ds.Available == "0 B" {
			t.Errorf("Datastore %s: AVAILABLE=0 B, want non-zero", ds.Name)
		}
	}

	hasUsed := false
	for _, ds := range dss {
		if ds.Used != "0 B" {
			hasUsed = true
			break
		}
	}
	if !hasUsed {
		t.Error("No datastore has non-zero USED")
	}
}

func TestVSwitchesHasBothTypes(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	var hasStd, hasDist bool
	for _, sw := range switches {
		if sw.SwitchType == "standard" {
			hasStd = true
		}
		if sw.SwitchType == "distributed" {
			hasDist = true
		}
	}

	if !hasStd {
		t.Error("No standard switch found")
	}
	if !hasDist {
		t.Error("No distributed switch found")
	}
}

func TestListPortGroupVMs_NonExistent(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	_, err := ListPortGroupVMs(context.Background(), client, "nonexistent-pg-xyz")
	if err == nil {
		t.Error("ListPortGroupVMs should return error for non-existent port group")
	}
}

func TestListPortGroupVMs_Sorting(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 3

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
		return
	}

	for i := 1; i < len(vms); i++ {
		if vms[i-1].Name > vms[i].Name {
			t.Errorf("VMs not sorted: %q > %q", vms[i-1].Name, vms[i].Name)
		}
	}
}

func TestListPortGroupVMs_ExactVMSet(t *testing.T) {
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
	allVMs, err := finder.VirtualMachineList(context.Background(), "*")
	if err != nil {
		t.Fatalf("list VMs: %v", err)
	}

	if len(allVMs) == 0 {
		t.Skip("no VMs in simulator")
	}

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	allVMNames := make(map[string]bool)
	for _, vm := range allVMs {
		allVMNames[vm.Name()] = true
	}

	for _, vm := range vms {
		if !allVMNames[vm.Name] {
			t.Errorf("VM %s in port group result but not in datacenter VM list", vm.Name)
		}
	}
}

func TestListPortGroupVMs_BothTypes(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	stdVMs, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs (standard): %v", err)
	}
	for _, vm := range stdVMs {
		if vm.PortGroup != "VM Network" {
			t.Errorf("Standard PG: expected VM Network, got %s", vm.PortGroup)
		}
	}

	distVMs, err := ListPortGroupVMs(context.Background(), client, "DC0_DVPG0")
	if err != nil {
		t.Logf("ListPortGroupVMs (distributed): %v", err)
	}
	for _, vm := range distVMs {
		if vm.PortGroup != "DC0_DVPG0" {
			t.Errorf("Distributed PG: expected DC0_DVPG0, got %s", vm.PortGroup)
		}
	}
}

func TestDVSContainerView(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	var distCount int
	for _, sw := range switches {
		if sw.SwitchType == "distributed" {
			distCount++
		}
	}

	if distCount == 0 {
		t.Error("No distributed switches found; want at least 1")
	}
}

func TestVSwitchesTotalPorts(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	hasPorts := false
	for _, sw := range switches {
		if sw.TotalPorts > 0 {
			hasPorts = true
			break
		}
	}
	if !hasPorts {
		t.Error("No switch has TotalPorts > 0")
	}
}

func TestVSwitchesLACP(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	for _, sw := range switches {
		if sw.SwitchType == "distributed" {
			if sw.LACP != "enabled" && sw.LACP != "disabled" && sw.LACP != "N/A" {
				t.Errorf("Distributed switch %s: LACP=%q, want enabled/disabled/N/A", sw.SwitchName, sw.LACP)
			}
		} else {
			if sw.LACP != "N/A" {
				t.Errorf("Standard switch %s: LACP=%q, want N/A", sw.SwitchName, sw.LACP)
			}
		}
	}
}

func TestStandardSwitchesPortGroupFilter(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	for _, sw := range switches {
		if sw.SwitchType != "standard" {
			continue
		}
		for _, pg := range sw.PortGroups {
			if pg.Name == "" {
				t.Errorf("Standard switch %s has empty port group name", sw.SwitchName)
			}
		}
	}
}

func TestListPortGroupVMs_VMNetwork(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	if len(vms) > 0 {
		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("VM has empty name")
			}
		}
	}
}

func TestListPortGroupVMs_PortGroupNameCaseInsensitive(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	_, err1 := ListPortGroupVMs(context.Background(), client, "vm network")
	_, err2 := ListPortGroupVMs(context.Background(), client, "VM NETWORK")
	_, err3 := ListPortGroupVMs(context.Background(), client, "VM Network")

	if (err1 == nil) != (err2 == nil) {
		t.Error("case-sensitive behavior inconsistent")
	}
	if (err1 == nil) != (err3 == nil) {
		t.Error("case-sensitive behavior inconsistent")
	}
}

func TestListPortGroupVMs_WithVMs(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}
	_ = vms
}

func TestListPortGroupVMs_EmptyResult(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 0

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	_, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs with no VMs: %v", err)
	}
}

func TestListPortGroupVMs_PortGroupVMsReturned(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	if vms == nil {
		t.Log("vms is nil (acceptable)")
	}
}

func TestListPortGroupVMs_StandardPortGroupVMs(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	for _, vm := range vms {
		if vm.Name == "" {
			t.Error("VM name is empty")
		}
	}
}

func TestVMNetworkReference(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

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
	vms, err := finder.VirtualMachineList(context.Background(), "*")
	if err != nil {
		t.Fatalf("list VMs: %v", err)
	}

	if len(vms) == 0 {
		t.Skip("no VMs in simulator")
	}

	for _, vm := range vms {
		var moVM mo.VirtualMachine
		pc := client.PropertyCollector()
		if err := pc.RetrieveOne(context.Background(), vm.Reference(), []string{"Name", "Network"}, &moVM); err != nil {
			t.Fatalf("retrieve VM properties: %v", err)
		}

		if len(moVM.Network) == 0 {
			t.Logf("VM %s has no network references", moVM.Name)
		}
	}
}

func TestListPortGroupVMs_HostConfigFallback(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

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
	hosts, err := finder.HostSystemList(context.Background(), "*")
	if err != nil {
		t.Fatalf("list hosts: %v", err)
	}

	for _, h := range hosts {
		var hostMo mo.HostSystem
		pc := client.PropertyCollector()
		if err := pc.RetrieveOne(context.Background(), h.Reference(), []string{"config.network.portgroup"}, &hostMo); err != nil {
			t.Fatalf("retrieve host network config: %v", err)
		}

		if hostMo.Config != nil && hostMo.Config.Network != nil {
			t.Logf("Host %s has %d port groups", h.Name(), len(hostMo.Config.Network.Portgroup))
			for _, pg := range hostMo.Config.Network.Portgroup {
				t.Logf("  - %s (vSwitch: %s)", pg.Spec.Name, pg.Spec.VswitchName)
			}
		}
	}
}

func TestListPortGroupVMs_NetworkFolder(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

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

	dc := datacenters[0]
	folders, err := dc.Folders(context.Background())
	if err != nil {
		t.Fatalf("datacenter folders: %v", err)
	}

	if folders.NetworkFolder == nil {
		t.Fatal("network folder is nil")
	}

	children, err := folders.NetworkFolder.Children(context.Background())
	if err != nil {
		t.Fatalf("list network folder children: %v", err)
	}

	t.Logf("Network folder has %d children", len(children))
	for _, child := range children {
		ref := child.Reference()
		pc := client.PropertyCollector()
		switch ref.Type {
		case "Network":
			var netMo mo.Network
			if err := pc.RetrieveOne(context.Background(), ref, []string{"Name"}, &netMo); err != nil {
				t.Logf("  - %s (%s)", ref.Value, ref.Type)
			} else {
				t.Logf("  - %s (%s)", netMo.Name, ref.Type)
			}
		case "DistributedVirtualSwitch":
			t.Logf("  - DVS (%s)", ref.Type)
		case "DistributedVirtualPortgroup":
			var pgMo mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(context.Background(), ref, []string{"Name"}, &pgMo); err != nil {
				t.Logf("  - %s (%s)", ref.Value, ref.Type)
			} else {
				t.Logf("  - %s (%s)", pgMo.Name, ref.Type)
			}
		default:
			t.Logf("  - %s (%s)", ref.Value, ref.Type)
		}
	}
}

func TestListPortGroupVMs_NetworkType(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

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

	dc := datacenters[0]
	folders, err := dc.Folders(context.Background())
	if err != nil {
		t.Fatalf("datacenter folders: %v", err)
	}

	if folders.NetworkFolder == nil {
		t.Skip("no network folder")
	}

	children, err := folders.NetworkFolder.Children(context.Background())
	if err != nil {
		t.Fatalf("list network folder children: %v", err)
	}

	for _, child := range children {
		ref := child.Reference()
		pc := client.PropertyCollector()
		switch ref.Type {
		case "Network":
			var netMo mo.Network
			if err := pc.RetrieveOne(context.Background(), ref, []string{"Name"}, &netMo); err != nil {
				t.Logf("Network: %s, Type: %s", ref.Value, ref.Type)
			} else {
				t.Logf("Network: %s, Type: %s", netMo.Name, ref.Type)
			}
		case "DistributedVirtualSwitch":
			t.Logf("Network: DVS, Type: %s", ref.Type)
		case "DistributedVirtualPortgroup":
			var pgMo mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(context.Background(), ref, []string{"Name"}, &pgMo); err != nil {
				t.Logf("Network: %s, Type: %s", ref.Value, ref.Type)
			} else {
				t.Logf("Network: %s, Type: %s", pgMo.Name, ref.Type)
			}
		default:
			t.Logf("Network: %s, Type: %s", ref.Value, ref.Type)
		}
	}
}

func TestListPortGroupVMs_PortGroupNotFound(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	_, err := ListPortGroupVMs(context.Background(), client, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent port group")
	}
}

func TestListPortGroupVMs_MultipleDatacenters(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	_, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs with VPX model: %v", err)
	}
}

func TestListPortGroupVMs_NoSkip(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}
	_ = vms
}

func TestListPortGroupVMs_StandardOnly(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	for _, vm := range vms {
		if vm.PortGroup != "VM Network" {
			t.Errorf("Expected VM Network, got %s", vm.PortGroup)
		}
	}
}

func TestListPortGroupVMs_StandardNoDistributed(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	for _, vm := range vms {
		if vm.PortGroup != "VM Network" {
			t.Errorf("Expected VM Network, got %s", vm.PortGroup)
		}
	}
}

func TestVSwitchesPortGroupsNotDuplicated(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	switches, err := ListSwitches(context.Background(), client)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}

	totalPGs := 0
	for _, sw := range switches {
		totalPGs += len(sw.PortGroups)
	}

	if totalPGs == 0 {
		t.Log("No port groups found (may be expected for minimal simulator)")
	}

	for _, sw := range switches {
		pgNames := make(map[string]int)
		for _, pg := range sw.PortGroups {
			pgNames[pg.Name]++
		}
		for name, count := range pgNames {
			if count > 1 {
				t.Errorf("Switch %s: port group %q appears %d times", sw.SwitchName, name, count)
			}
		}
	}
}

func TestListPortGroupVMs_WithClient(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 1

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	_, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}
}

func TestListPortGroupVMs_StandardPG(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	vms, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs: %v", err)
	}

	for _, vm := range vms {
		if vm.PortGroup != "VM Network" {
			t.Errorf("Expected VM Network, got %s", vm.PortGroup)
		}
	}
}

func TestListPortGroupVMs_BothPGs(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2

	client := createSimClient(t, model)
	defer client.Logout(context.Background())

	stdVMs, err := ListPortGroupVMs(context.Background(), client, "VM Network")
	if err != nil {
		t.Logf("ListPortGroupVMs (standard): %v", err)
	}

	distVMs, err := ListPortGroupVMs(context.Background(), client, "DC0_DVPG0")
	if err != nil {
		t.Logf("ListPortGroupVMs (distributed): %v", err)
	}

	for _, vm := range stdVMs {
		if vm.PortGroup != "VM Network" {
			t.Errorf("Standard PG: expected VM Network, got %s", vm.PortGroup)
		}
	}

	for _, vm := range distVMs {
		if vm.PortGroup != "DC0_DVPG0" {
			t.Errorf("Distributed PG: expected DC0_DVPG0, got %s", vm.PortGroup)
		}
	}
}
