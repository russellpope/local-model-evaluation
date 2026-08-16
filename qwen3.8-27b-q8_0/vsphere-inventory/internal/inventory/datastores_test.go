package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

var validTransportTypes = map[string]bool{
	TransportFC:      true,
	TransportISCSI:   true,
	TransportNVMe:    true,
	TransportNFS:     true,
	TransportUnknown: true,
}

func TestListDatastores(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dss, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}

		if len(dss) != 1 {
			t.Fatalf("ListDatastores returned %d datastores, want 1", len(dss))
		}

		for _, ds := range dss {
			if ds.Name == "" {
				t.Errorf("datastore with empty name")
			}
			if ds.Available > ds.Capacity {
				t.Errorf("datastore %s: available %d > capacity %d", ds.Name, ds.Available, ds.Capacity)
			}
			if want := clampUsed(ds.Capacity, ds.Available); ds.Used != want {
				t.Errorf("datastore %s: used = %d, want capacity - available = %d", ds.Name, ds.Used, want)
			}
			if !validTransportTypes[ds.Type] {
				t.Errorf("datastore %s: type %q, want one of FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Type)
			}
			// The simulator does not model storage transport, so the
			// local datastore must degrade to unknown rather than
			// inventing a fabric.
			if ds.Type != TransportUnknown {
				t.Errorf("datastore %s: type = %q, want unknown from the simulator", ds.Name, ds.Type)
			}
		}

		// Refresh populates real capacity in the simulator, which makes
		// the used/available math checkable against non-zero values.
		ref, err := datastoreRefByName(ctx, c, dss[0].Name)
		if err != nil {
			t.Fatalf("find datastore ref: %v", err)
		}
		if _, err := methods.RefreshDatastore(ctx, c, &types.RefreshDatastore{This: ref}); err != nil {
			t.Fatalf("RefreshDatastore(%s): %v", dss[0].Name, err)
		}

		refreshed, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores after refresh: %v", err)
		}
		if len(refreshed) != 1 {
			t.Fatalf("got %d datastores after refresh, want 1", len(refreshed))
		}
		ds := refreshed[0]
		if ds.Capacity <= 0 {
			t.Errorf("datastore %s: capacity = %d after refresh, want > 0", ds.Name, ds.Capacity)
		}
		if ds.Available != ds.Capacity {
			t.Errorf("datastore %s: available = %d, want full capacity %d", ds.Name, ds.Available, ds.Capacity)
		}
		if ds.Used != 0 {
			t.Errorf("datastore %s: used = %d, want 0 on an empty datastore", ds.Name, ds.Used)
		}
	}, smallModel())
}

func clampUsed(total, available int64) int64 {
	used := total - available
	if used < 0 {
		return 0
	}
	if used > total {
		return total
	}
	return used
}

func datastoreRefByName(ctx context.Context, c *vim25.Client, name string) (types.ManagedObjectReference, error) {
	refs, err := findRefs(ctx, c, "Datastore")
	if err != nil {
		return types.ManagedObjectReference{}, err
	}

	var content []mo.Datastore
	if err := retrieve(ctx, c, refs, []string{"name"}, &content); err != nil {
		return types.ManagedObjectReference{}, err
	}
	for i := range content {
		if content[i].Name == name {
			return content[i].Self, nil
		}
	}
	return types.ManagedObjectReference{}, &datastoreNotFound{name: name}
}

type datastoreNotFound struct{ name string }

func (e *datastoreNotFound) Error() string { return "datastore " + e.name + " not found" }
