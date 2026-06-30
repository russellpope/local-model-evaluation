package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetVMs(t *testing.T) {
	simulator.Test(func(ctx context.Context, client *vim25.Client) {
		vms, err := GetVMs(ctx, client)
		if err != nil {
			t.Fatalf("GetVMs failed: %v", err)
		}

		if len(vms) == 0 {
			t.Error("expected at least one VM")
		}

		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("expected VM to have a name")
			}
			if vm.VCPU <= 0 {
				t.Errorf("VM %s has invalid VCPU: %d", vm.Name, vm.VCPU)
			}
			if vm.RAMGB <= 0 {
				t.Errorf("VM %s has invalid RAM: %f", vm.Name, vm.RAMGB)
			}
		}
	}, simulator.VPX())
}

func TestGetDatastores(t *testing.T) {
	simulator.Test(func(ctx context.Context, client *vim25.Client) {
		dstores, err := GetDatastores(ctx, client)
		if err != nil {
			t.Fatalf("GetDatastores failed: %v", err)
		}

		if len(dstores) == 0 {
			t.Error("expected at least one datastore")
		}

		for _, ds := range dstores {
			if ds.Name == "" {
				t.Error("expected datastore to have a name")
			}
		}
	}, simulator.VPX())
}
