package inventory

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// newVPXModel returns a deterministic simulator model:
// 1 DC, 1 standalone host + 1 cluster of 2 hosts, 3 VMs per placement
// (9 VMs total), 2 local datastores per DC, 1 standard vSwitch with 2
// portgroups, 1 DVS with 2 dv portgroups. All VM NICs attach to DC0_DVPG0.
func newVPXModel() *simulator.Model {
	s := simulator.VPX()
	s.Datacenter = 1
	s.Host = 1
	s.Cluster = 1
	s.ClusterHost = 2
	s.Machine = 3
	s.Datastore = 2
	s.Portgroup = 2
	return s
}

func TestListVMsAgainstSimulator(t *testing.T) {
	model := newVPXModel()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := ListVMs(ctx, c)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		want := (model.Host + model.Cluster) * model.Machine
		if len(vms) != want {
			t.Fatalf("got %d VMs, want %d", len(vms), want)
		}
		for i := 1; i < len(vms); i++ {
			if vms[i-1].Name > vms[i].Name {
				t.Errorf("rows not sorted by name: %q before %q", vms[i-1].Name, vms[i].Name)
			}
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("VM with empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("%s: vCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.MemoryMB <= 0 {
				t.Errorf("%s: MemoryMB = %d, want > 0", vm.Name, vm.MemoryMB)
			}
			if vm.Storage < 0 {
				t.Errorf("%s: storage = %d, want >= 0", vm.Name, vm.Storage)
			}
		}
	}, model)
}

func TestListDatastoresAgainstSimulator(t *testing.T) {
	model := newVPXModel()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		stores, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		// The model creates model.Datastore shared local datastores per DC.
		want := model.Datacenter * model.Datastore
		if len(stores) != want {
			t.Fatalf("got %d datastores, want %d", len(stores), want)
		}
		for i := 1; i < len(stores); i++ {
			if stores[i-1].Name > stores[i].Name {
				t.Errorf("rows not sorted by name: %q before %q", stores[i-1].Name, stores[i].Name)
			}
		}
		for _, ds := range stores {
			if ds.Name == "" {
				t.Error("datastore with empty name")
			}
			if ds.FreeSpace > ds.Capacity {
				t.Errorf("%s: available %d > capacity %d", ds.Name, ds.FreeSpace, ds.Capacity)
			}
			used := ds.Capacity - ds.FreeSpace
			if used < 0 {
				used = 0
			}
			if used+ds.FreeSpace != ds.Capacity {
				t.Errorf("%s: used(%d)+available(%d) != capacity(%d)", ds.Name, used, ds.FreeSpace, ds.Capacity)
			}
			switch ds.Transport {
			case "FC", "iSCSI", "NVMe", "NFS", "unknown":
			default:
				t.Errorf("%s: transport %q not in FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Transport)
			}
		}
	}, model)
}

func TestListSwitchesAgainstSimulator(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		// The simulator attaches every VM NIC to DC0_DVPG0, which starts with
		// a single configured port. Widen it so usage <= capacity is a real
		// invariant we can assert against (as on a live vCenter).
		widenDVPortgroup(t, ctx, c, "DC0_DVPG0", 32)

		rows, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("no switches returned")
		}

		var std, dist int
		for _, r := range rows {
			switch r.SwitchType {
			case "standard":
				std++
				if r.LACP != "N/A" {
					t.Errorf("%s/%s: standard LACP = %q, want N/A", r.Switch, r.Portgroup, r.LACP)
				}
			case "distributed":
				dist++
				if r.LACP == "N/A" {
					t.Errorf("%s/%s: distributed LACP = N/A", r.Switch, r.Portgroup)
				}
			default:
				t.Fatalf("unexpected switch type %q", r.SwitchType)
			}
			if r.LACP != "enabled" && r.LACP != "disabled" && r.LACP != "N/A" {
				t.Errorf("%s/%s: LACP %q not in enabled/disabled/N-A", r.Switch, r.Portgroup, r.LACP)
			}
			if r.Ports < r.Used {
				t.Errorf("%s/%s: used ports %d > total %d", r.Switch, r.Portgroup, r.Used, r.Ports)
			}
			if r.VLAN == "" {
				t.Errorf("%s/%s: empty VLAN", r.Switch, r.Portgroup)
			}
			if r.Switch == "" || r.Portgroup == "" {
				t.Errorf("empty switch/portgroup name in %+v", r)
			}
		}
		if std == 0 {
			t.Error("no standard switches reported")
		}
		if dist == 0 {
			t.Error("no distributed switches reported")
		}
		for i := 1; i < len(rows); i++ {
			if rows[i-1].Switch > rows[i].Switch ||
				(rows[i-1].Switch == rows[i].Switch && rows[i-1].Portgroup > rows[i].Portgroup) {
				t.Fatalf("rows not sorted: %v then %v", rows[i-1], rows[i])
			}
		}
	}, newVPXModel())
}

// TestVlanValuesParse checks VLAN rendering: numeric IDs parse as integers
// and trunk/private-VLAN forms carry their type label.
func TestVlanValuesParse(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		rows, err := ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		seenTrunk := false
		for _, r := range rows {
			v := r.VLAN
			if v == "unknown" || v == "trunk" {
				continue
			}
			if strings.HasPrefix(v, "private-vlan:") {
				continue
			}
			// numeric or numeric ranges over separators
			for _, part := range strings.Split(v, ",") {
				for _, num := range strings.Split(part, "-") {
					if _, err := strconv.Atoi(num); err != nil {
						t.Errorf("VLAN %q in %s/%s does not parse as numbers: %v", v, r.Switch, r.Portgroup, err)
					}
				}
			}
			if strings.Contains(v, "-") {
				seenTrunk = true
			}
		}
		if !seenTrunk {
			t.Log("no trunked VLAN present in model; range rendering not exercised (unit tests cover it)")
		}
	}, newVPXModel())
}

func TestContextTimeoutHonored(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		tctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // guarantee expiry
		if _, err := ListVMs(tctx, c); err == nil {
			t.Fatal("expected error from expired context, got nil")
		}
	}, newVPXModel())
}

// widenDVPortgroup grows a dv portgroup's configured port count through the
// real API so that "used <= total" can be asserted meaningfully.
func widenDVPortgroup(t *testing.T, ctx context.Context, c *vim25.Client, name string, ports int32) {
	t.Helper()
	var pgs []mo.DistributedVirtualPortgroup
	retrieveAll(t, ctx, c, "DistributedVirtualPortgroup", []string{"name", "key", "config", "portKeys"}, &pgs)
	for _, pg := range pgs {
		if pg.Config.Name != name {
			continue
		}
		dpg := object.NewDistributedVirtualPortgroup(c, pg.Self)
		spec := types.DVPortgroupConfigSpec{
			Name:                 pg.Config.Name,
			ConfigVersion:        pg.Config.ConfigVersion,
			NumPorts:             ports,
			Type:                 pg.Config.Type,
			DefaultPortConfig:    pg.Config.DefaultPortConfig,
			Policy:               pg.Config.Policy,
			VendorSpecificConfig: pg.Config.VendorSpecificConfig,
			AutoExpand:           pg.Config.AutoExpand,
		}
		task, err := dpg.Reconfigure(ctx, spec)
		if err != nil {
			t.Fatalf("reconfigure %s: %v", name, err)
		}
		if err := task.Wait(ctx); err != nil {
			t.Fatalf("reconfigure %s wait: %v", name, err)
		}
		return
	}
	t.Fatalf("portgroup %s not found", name)
}

func retrieveAll(t *testing.T, ctx context.Context, c *vim25.Client, kind string, props []string, dst any) {
	t.Helper()
	refs, err := listKind(ctx, c, kind)
	if err != nil {
		t.Fatalf("list %s: %v", kind, err)
	}
	if err := collect(ctx, c, refs, props, dst); err != nil {
		t.Fatalf("collect %s: %v", kind, err)
	}
}
