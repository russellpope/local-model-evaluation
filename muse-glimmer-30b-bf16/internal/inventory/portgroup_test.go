package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetVMsForPortGroup(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()
	simulator.Run(func(ctx context.Context, c *vim25.Client) error {
		vms, err := GetVMsForPortGroup(ctx, c, "does-not-exist")
		if err != nil {
			return err
		}
		if len(vms) != 0 {
			t.Errorf("expected empty for unknown port group")
		}
		return nil
	}, model)
}
