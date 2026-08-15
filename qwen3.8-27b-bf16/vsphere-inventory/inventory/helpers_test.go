package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// testModel returns a deterministic simulator model:
//
//   - 1 datacenter (DC0)
//   - 1 standalone host (DC0_H0) carrying the standard vSwitch "vSwitch0"
//   - 2 local datastores (LocalDS_0, LocalDS_1)
//   - 1 DVS (DVS0) with 1 distributed port group (DC0_DVPG0)
//   - 3 VMs (DC0_H0_VM0..2), each with a single NIC backed by DC0_DVPG0
func testModel() *simulator.Model {
	m := simulator.VPX()
	m.Datacenter = 1
	m.Host = 1
	m.Cluster = 0
	m.ClusterHost = 0
	m.Datastore = 2
	m.Machine = 3
	m.Portgroup = 1
	m.Autostart = true
	return m
}

// withClient runs fn against an in-process vCenter simulator built from
// testModel. Failures inside fn must be reported via the *testing.T.
func withClient(t *testing.T, fn func(ctx context.Context, c *vim25.Client)) {
	t.Helper()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		fn(ctx, c)
	}, testModel())
}
