package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// runSimulator starts an in-process vCenter (vcsim model) configured by
// mutate, connects a client and runs f against it.
func runSimulator(t *testing.T, mutate func(m *simulator.Model), f func(ctx context.Context, c *govmomi.Client)) {
	t.Helper()

	model := simulator.VPX()
	if mutate != nil {
		mutate(model)
	}
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		f(ctx, &govmomi.Client{Client: vc})
	}, model)
}
