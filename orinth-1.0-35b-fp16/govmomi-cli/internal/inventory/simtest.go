package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vim25 "github.com/vmware/govmomi/vim25"
)

// runWithSimulator creates a VPX simulator model, optionally configures it
// via cfg (which may be nil), and runs fn inside model.Run(). It wraps any
// error from fn with context so test failures clearly show which simulator
// test failed.
func runWithSimulator(t *testing.T, cfg func(*simulator.Model), fn func(ctx context.Context, c *vim25.Client) error) {
	t.Helper()
	model := simulator.VPX()
	if cfg != nil {
		cfg(model)
	}
	if err := model.Run(fn); err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
