package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetVMs(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()
	simulator.Run(func(ctx context.Context, c *vim25.Client) error {
		vms, err := GetVMs(ctx, c)
		if err != nil {
			return err
		}
		if len(vms) == 0 {
			t.Fatalf("expected VMs")
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Errorf("empty name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VCPU <=0 for %s", vm.Name)
			}
			if vm.RAMGiB <= 0 {
				t.Errorf("RAM <=0 for %s", vm.Name)
			}
		}
		return nil
	}, model)
}
