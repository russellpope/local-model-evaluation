package datastores

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 1
	model.Datastore = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dsList, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores() error = %v", err)
		}

		if len(dsList) != 3 {
			t.Fatalf("GetDatastores() returned %d datastores, want 3", len(dsList))
		}

		for _, ds := range dsList {
			if ds.Name == "" {
				t.Error("Datastore name should not be empty")
			}

			if ds.UsedBytes+ds.AvailableBytes != ds.CapacityBytes {
				t.Errorf("Datastore %s: used + available (%d + %d) != capacity (%d)",
					ds.Name, ds.UsedBytes, ds.AvailableBytes, ds.CapacityBytes)
			}

			if ds.AvailableBytes > ds.CapacityBytes {
				t.Errorf("Datastore %s: available (%d) > capacity (%d)",
					ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}

			if ds.Type != "unknown" {
				t.Errorf("Datastore %s: type = %q, want %q (vcsim LocalDatastoreInfo should be unknown)",
					ds.Name, ds.Type, "unknown")
			}
		}
	}, model)
}
