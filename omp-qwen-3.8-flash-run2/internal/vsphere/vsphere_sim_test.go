package vsphere

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// model returns a deterministic VPX model: 1 DC, 1 cluster with 1 host, a
// fixed number of VMs / datastores / port groups.
func model(vms, dss, pgs int) *simulator.Model {
	m := simulator.VPX()
	m.Datacenter = 1
	m.Cluster = 1
	m.ClusterHost = 1
	m.Host = 0
	m.Machine = vms
	m.Pool = 1
	m.App = 0
	m.Datastore = dss
	m.Portgroup = pgs
	return m
}

// withSim runs fn against an in-process vCenter with a connected
// govmomi.Client built on the simulator's authenticated session.
func withSim(t *testing.T, m *simulator.Model, fn func(ctx context.Context, c *govmomi.Client)) {
	t.Helper()
	err := m.Run(func(ctx context.Context, vc *vim25.Client) error {
		c := &govmomi.Client{
			Client:         vc,
			SessionManager: session.NewManager(vc),
		}
		fn(ctx, c)
		return nil
	})
	if err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}

func TestListVMs(t *testing.T) {
	const want = 4
	withSim(t, model(want, 2, 2), func(ctx context.Context, c *govmomi.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) != want {
			t.Fatalf("got %d VMs, want %d", len(vms), want)
		}
		for i, vm := range vms {
			if vm.Name == "" {
				t.Errorf("vm[%d]: empty name", i)
			}
			if vm.VCPU <= 0 {
				t.Errorf("vm %q: vCPU %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.RAMMB <= 0 {
				t.Errorf("vm %q: RAM %d MiB, want > 0", vm.Name, vm.RAMMB)
			}
			if vm.CommittedStorage < 0 {
				t.Errorf("vm %q: committed storage %d, want >= 0", vm.Name, vm.CommittedStorage)
			}
		}
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Errorf("not sorted: %q before %q", vms[i-1].Name, vms[i].Name)
			}
		}
	})
}

func TestListVMsCommittedNotProvisioned(t *testing.T) {
	withSim(t, model(1, 1, 1), func(ctx context.Context, c *govmomi.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		vm := vms[0]
		// The simulator populates summary.storage.committed from the VM's
		// file layout; a zero here means we read the wrong property.
		if vm.CommittedStorage <= 0 {
			t.Fatalf("simulator populates summary.storage.committed; got %d — collector likely reading the wrong field", vm.CommittedStorage)
		}
	})
}

func TestListDatastores(t *testing.T) {
	const want = 3
	valid := map[string]bool{"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true}
	withSim(t, model(2, want, 1), func(ctx context.Context, c *govmomi.Client) {
		dss, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(dss) != want {
			t.Fatalf("got %d datastores, want %d", len(dss), want)
		}
		for _, ds := range dss {
			if ds.Name == "" {
				t.Error("datastore with empty name")
			}
			if !valid[ds.Transport] {
				t.Errorf("datastore %q: transport %q not in FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Transport)
			}
			if ds.Free > ds.Capacity {
				t.Errorf("datastore %q: free %d > capacity %d", ds.Name, ds.Free, ds.Capacity)
			}
			used := ds.Capacity - ds.Free
			if used < 0 {
				used = 0
			}
			if used+ds.Free != ds.Capacity {
				t.Errorf("datastore %q: used+avail %d != capacity %d", ds.Name, used+ds.Free, ds.Capacity)
			}
		}
		for i := 1; i < len(dss); i++ {
			if dss[i-1].Name > dss[i].Name {
				t.Errorf("not sorted: %q before %q", dss[i-1].Name, dss[i].Name)
			}
		}
	})
}

func TestListSwitches(t *testing.T) {
	withSim(t, model(2, 1, 2), func(ctx context.Context, c *govmomi.Client) {
		rows, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("no switches returned")
		}
		sawStandard, sawDistributed := false, false
		for _, r := range rows {
			switch r.SwitchType {
			case SwitchStandard:
				sawStandard = true
			case SwitchDistributed:
				sawDistributed = true
			default:
				t.Errorf("switch %q: bad type %q", r.SwitchName, r.SwitchType)
			}
			if r.SwitchName == "" || r.Name == "" {
				t.Errorf("empty switch/portgroup name in %+v", r)
			}
			if !validVLAN(r.VLAN) {
				t.Errorf("portgroup %q: unparseable VLAN %q", r.Name, r.VLAN)
			}
			if r.LACP != LACPEnabled && r.LACP != LACPDisabled && r.LACP != LACPNA {
				t.Errorf("portgroup %q: LACP %q not in enabled/disabled/N/A", r.Name, r.LACP)
			}
			if r.UsedPorts > r.NumPorts {
				t.Errorf("portgroup %q: used %d > total %d", r.Name, r.UsedPorts, r.NumPorts)
			}
			if r.SwitchType == SwitchStandard && r.LACP != LACPNA {
				t.Errorf("standard vswitch %q must report LACP N/A, got %q", r.SwitchName, r.LACP)
			}
		}
		if !sawStandard {
			t.Error("expected at least one standard vSwitch (simulator hosts run vSwitch0)")
		}
		if !sawDistributed {
			t.Error("expected at least one distributed switch (model has port groups)")
		}
	})
}

// validVLAN accepts the renderings formatDVLAN/formatStandardVLAN produce:
// a bare id, trunk(range[,range...]), pvlan(id), or "unknown".
func validVLAN(v string) bool {
	if v == "unknown" {
		return true
	}
	if strings.HasPrefix(v, "trunk(") && strings.HasSuffix(v, ")") {
		return true
	}
	if strings.HasPrefix(v, "pvlan(") && strings.HasSuffix(v, ")") {
		return true
	}
	if v == "" {
		return false
	}
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func TestVMsOnPortGroup(t *testing.T) {
	// The simulator attaches every model VM's first NIC to the first DVPG
	// (DC0_DVPG0). All VMs must come back for it, none for a standard
	// group that no VM uses.
	const wantVMs = 3
	withSim(t, model(wantVMs, 1, 2), func(ctx context.Context, c *govmomi.Client) {
		all, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(all) != wantVMs {
			t.Fatalf("model VM count %d, want %d", len(all), wantVMs)
		}

		pg := "DC0_DVPG0"
		got, err := VMsOnPortGroup(ctx, c, pg)
		if err != nil {
			t.Fatalf("VMsOnPortGroup(%q): %v", pg, err)
		}
		if len(got) != wantVMs {
			t.Fatalf("DVPG %q: got VMs %v, want all %d model VMs", pg, names(got), wantVMs)
		}
		want := map[string]bool{}
		for _, vm := range all {
			want[vm.Name] = true
		}
		for _, vm := range got {
			if !want[vm.Name] {
				t.Errorf("DVPG %q: unexpected VM %q", pg, vm.Name)
			}
		}

		// Standard port group with no attached VMs → empty, no error.
		std, err := VMsOnPortGroup(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("VMsOnPortGroup(VM Network): %v", err)
		}
		if len(std) != 0 {
			t.Errorf("VM Network: got %v, want none (model VMs sit on DVPG0)", names(std))
		}

		// Unknown name → error mentioning known groups.
		_, err = VMsOnPortGroup(ctx, c, "no-such-pg")
		if err == nil {
			t.Fatal("expected error for unknown port group")
		}
		if !strings.Contains(err.Error(), pg) {
			t.Errorf("error should list known port groups: %v", err)
		}
	})
}

func names(vms []VMInfo) []string {
	out := make([]string, len(vms))
	for i, vm := range vms {
		out[i] = vm.Name
	}
	return out
}

func TestContextCancellation(t *testing.T) {
	// Collectors must honor a cancelled context instead of hanging.
	m := model(1, 1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.Run(func(_ context.Context, vc *vim25.Client) error {
		c := &govmomi.Client{Client: vc, SessionManager: session.NewManager(vc)}
		if _, err := ListVMs(ctx, c); err == nil {
			t.Error("ListVMs with cancelled ctx: want error")
		} else if !errors.Is(err, context.Canceled) {
			t.Errorf("ListVMs cancelled ctx error = %v, want context.Canceled descendant", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
