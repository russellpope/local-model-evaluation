package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestListDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 1
	model.ClusterHost = 1
	model.Datastore = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		client := &govmomi.Client{
			Client:         c,
			SessionManager: session.NewManager(c),
		}

		dss, err := ListDatastores(ctx, client)
		if err != nil {
			t.Fatalf("ListDatastores failed: %v", err)
		}

		if len(dss) != model.Datastore {
			t.Errorf("expected %d datastores, got %d", model.Datastore, len(dss))
		}

		for _, ds := range dss {
			if ds.Name == "" {
				t.Error("datastore has empty name")
			}
			if ds.Capacity < 0 {
				t.Errorf("datastore %s has negative capacity: %d", ds.Name, ds.Capacity)
			}
			if ds.Available < 0 {
				t.Errorf("datastore %s has negative available: %d", ds.Name, ds.Available)
			}
			if ds.Available > ds.Capacity {
				t.Errorf("datastore %s available (%d) > capacity (%d)", ds.Name, ds.Available, ds.Capacity)
			}
			if ds.Used+ds.Available != ds.Capacity {
				t.Errorf("datastore %s: used (%d) + available (%d) != capacity (%d)",
					ds.Name, ds.Used, ds.Available, ds.Capacity)
			}
			valid := false
			for _, typ := range []string{"FC", "iSCSI", "NVMe", "NFS", "local", "VSAN", "VVOL", "unknown"} {
				if ds.Type == typ {
					valid = true
					break
				}
			}
			if !valid {
				t.Errorf("datastore %s has invalid type: %q", ds.Name, ds.Type)
			}
		}
	}, model)
}
