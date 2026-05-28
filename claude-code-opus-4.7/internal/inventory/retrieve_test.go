package inventory

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func vmNames(vms []VMInfo) []string {
	out := make([]string, len(vms))
	for i, v := range vms {
		out[i] = v.Name
	}
	sort.Strings(out)
	return out
}

// TestGetVMs verifies the vms feature against the embedded simulator. The
// default VPX model (govmomi v0.54.0) deterministically creates four VMs.
func TestGetVMs(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := GetVMs(ctx, c)
		if err != nil {
			t.Fatalf("GetVMs: %v", err)
		}
		if len(vms) != 4 {
			t.Fatalf("len(vms) = %d, want 4", len(vms))
		}
		if !sort.SliceIsSorted(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name }) {
			t.Errorf("vms not sorted by name: %v", vmNames(vms))
		}
		for _, v := range vms {
			if v.Name == "" {
				t.Errorf("VM has empty name")
			}
			if v.NumCPU <= 0 {
				t.Errorf("%s: NumCPU = %d, want > 0", v.Name, v.NumCPU)
			}
			if v.MemoryMB <= 0 {
				t.Errorf("%s: MemoryMB = %d, want > 0", v.Name, v.MemoryMB)
			}
			if v.CommittedBytes < 0 {
				t.Errorf("%s: CommittedBytes = %d, want >= 0", v.Name, v.CommittedBytes)
			}
		}
	})
}

// TestGetDatastores verifies the datastores feature: capacity math is
// consistent and the transport type is always one of the allowed values
// (unknown against the simulator, which does not model storage transport).
func TestGetDatastores(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dss, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores: %v", err)
		}
		if len(dss) == 0 {
			t.Fatalf("no datastores returned")
		}
		if !sort.SliceIsSorted(dss, func(i, j int) bool { return dss[i].Name < dss[j].Name }) {
			t.Errorf("datastores not sorted by name")
		}
		valid := map[string]bool{
			TransportFC: true, TransportISCSI: true, TransportNVMe: true,
			TransportNFS: true, TransportUnknown: true,
		}
		for _, d := range dss {
			if d.Name == "" {
				t.Errorf("datastore has empty name")
			}
			if d.FreeBytes > d.CapacityBytes {
				t.Errorf("%s: free %d > capacity %d", d.Name, d.FreeBytes, d.CapacityBytes)
			}
			if got := d.UsedBytes(); got != d.CapacityBytes-d.FreeBytes {
				t.Errorf("%s: UsedBytes = %d, want %d", d.Name, got, d.CapacityBytes-d.FreeBytes)
			}
			if d.UsedBytes() < 0 {
				t.Errorf("%s: negative used bytes %d", d.Name, d.UsedBytes())
			}
			if !valid[d.Type] {
				t.Errorf("%s: type %q not in FC/iSCSI/NVMe/NFS/unknown", d.Name, d.Type)
			}
		}
	})
}

// TestGetSwitches verifies the vswitches feature covers both standard and
// distributed switches, that LACP is distributed-only, that VLAN values render
// non-empty, and that used ports never exceed total ports.
func TestGetSwitches(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		sws, err := GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches: %v", err)
		}
		if len(sws) == 0 {
			t.Fatalf("no switches returned")
		}
		validLACP := map[string]bool{LACPEnabled: true, LACPDisabled: true, LACPNA: true}
		var haveStandard, haveDistributed, haveUplink bool
		for _, s := range sws {
			if !validLACP[s.LACP] {
				t.Errorf("%s: LACP %q not in enabled/disabled/N/A", s.Name, s.LACP)
			}
			switch s.Type {
			case SwitchStandard:
				haveStandard = true
				if s.LACP != LACPNA {
					t.Errorf("standard switch %s: LACP = %q, want N/A", s.Name, s.LACP)
				}
				for _, u := range s.Uplinks {
					if u == "vmnic0" {
						haveUplink = true
					}
				}
			case SwitchDistributed:
				haveDistributed = true
				if s.LACP == LACPNA {
					t.Errorf("distributed switch %s: LACP must not be N/A", s.Name)
				}
			default:
				t.Errorf("%s: invalid switch type %q", s.Name, s.Type)
			}
			for _, pg := range s.PortGroups {
				if pg.VLAN == "" {
					t.Errorf("%s/%s: empty VLAN", s.Name, pg.Name)
				}
				if pg.UsedPorts > pg.TotalPorts {
					t.Errorf("%s/%s: used ports %d > total %d", s.Name, pg.Name, pg.UsedPorts, pg.TotalPorts)
				}
			}
		}
		if !haveStandard {
			t.Errorf("expected at least one standard switch")
		}
		if !haveDistributed {
			t.Errorf("expected at least one distributed switch")
		}
		if !haveUplink {
			t.Errorf("expected a standard switch with uplink vmnic0")
		}
	})
}

// TestGetPortgroupVMs verifies the port-group lookup for a distributed port
// group (all four default VMs attach to DC0_DVPG0) and that a standard port
// group resolves (with no VMs attached) while an unknown name errors.
func TestGetPortgroupVMs(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vms, err := GetPortgroupVMs(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("GetPortgroupVMs(DC0_DVPG0): %v", err)
		}
		got := vmNames(vms)
		want := []string{"DC0_C0_RP0_VM0", "DC0_C0_RP0_VM1", "DC0_H0_VM0", "DC0_H0_VM1"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DC0_DVPG0 VMs = %v, want %v", got, want)
		}

		vmNet, err := GetPortgroupVMs(ctx, c, "VM Network")
		if err != nil {
			t.Fatalf("GetPortgroupVMs(VM Network): %v", err)
		}
		if len(vmNet) != 0 {
			t.Errorf("VM Network VMs = %d, want 0 in the default model", len(vmNet))
		}

		if _, err := GetPortgroupVMs(ctx, c, "no-such-portgroup"); err == nil {
			t.Errorf("expected error for unknown port group, got nil")
		}
	})
}
