package inventory

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// TestVMsByDistributedPortgroup configures the model so the known set of VMs
// is attached to a distributed port group, then asserts the lookup returns
// exactly that set.
func TestVMsByDistributedPortgroup(t *testing.T) {
	m := simulator.VPX()
	m.Cluster = 0
	m.Host = 1
	m.Machine = 4
	m.Portgroup = 1

	runModel(t, m, func(ctx context.Context, c *vim25.Client) error {
		all, err := FetchVMs(ctx, c)
		if err != nil {
			return fmt.Errorf("FetchVMs: %w", err)
		}
		if len(all) != 4 {
			return fmt.Errorf("model has %d VMs, want 4", len(all))
		}
		vms, err := FetchVMsByPortgroup(ctx, c, "DC0_DVPG0")
		if err != nil {
			return fmt.Errorf("FetchVMsByPortgroup: %w", err)
		}
		return sameSet(all, vms)
	})
}

// TestVMsByStandardPortgroup runs the same assertion against a standard
// (host vSwitch) port group: the ESX model attaches every VM to "VM Network".
func TestVMsByStandardPortgroup(t *testing.T) {
	m := simulator.ESX()
	m.Machine = 2

	runModel(t, m, func(ctx context.Context, c *vim25.Client) error {
		all, err := FetchVMs(ctx, c)
		if err != nil {
			return fmt.Errorf("FetchVMs: %w", err)
		}
		if len(all) != 2 {
			return fmt.Errorf("model has %d VMs, want 2", len(all))
		}
		vms, err := FetchVMsByPortgroup(ctx, c, "VM Network")
		if err != nil {
			return fmt.Errorf("FetchVMsByPortgroup: %w", err)
		}
		return sameSet(all, vms)
	})
}

func TestVMsByUnknownPortgroup(t *testing.T) {
	m := simulator.VPX()
	m.Cluster = 0
	m.Host = 1
	m.Machine = 1
	m.Portgroup = 1

	runModel(t, m, func(ctx context.Context, c *vim25.Client) error {
		if _, err := FetchVMsByPortgroup(ctx, c, "no-such-portgroup"); err == nil {
			return fmt.Errorf("FetchVMsByPortgroup for unknown portgroup = nil error, want error")
		}
		if _, err := FetchVMsByPortgroup(ctx, c, "   "); err == nil {
			return fmt.Errorf("FetchVMsByPortgroup for blank portgroup = nil error, want error")
		}
		return nil
	})
}

func sameSet(all, subset []VMInfo) error {
	want := names(all)
	got := names(subset)
	if len(want) != len(got) {
		return fmt.Errorf("portgroup VMs = %v, want %v", got, want)
	}
	for i := range want {
		if want[i] != got[i] {
			return fmt.Errorf("portgroup VMs = %v, want %v", got, want)
		}
	}
	if len(got) > 0 && got[0] == "" {
		return fmt.Errorf("portgroup VM lookup returned an empty name")
	}
	return nil
}

func names(vms []VMInfo) []string {
	out := make([]string, len(vms))
	for i, vm := range vms {
		out[i] = vm.Name
	}
	sort.Strings(out)
	return out
}
