package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetSwitches(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()
	simulator.Run(func(ctx context.Context, c *vim25.Client) error {
		sw, err := GetSwitches(ctx, c)
		if err != nil {
			return err
		}
		if len(sw) == 0 {
			t.Log("no switches")
			return nil
		}
		for _, s := range sw {
			if s.Used > s.Ports {
				t.Errorf("used > ports")
			}
			if s.LACP != "enabled" && s.LACP != "disabled" && s.LACP != "N/A" {
				t.Errorf("bad LACP %s", s.LACP)
			}
		}
		return nil
	}, model)
}
