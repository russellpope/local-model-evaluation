package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"vint/transport"
)

func TestListDatastoresAgainstSimulator(t *testing.T) {
	const wantCount = 3

	model := simulator.VPX()
	model.Datastore = wantCount

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dss, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(dss) != wantCount {
			t.Fatalf("ListDatastores returned %d datastores, want %d", len(dss), wantCount)
		}

		validTypes := map[string]bool{
			transport.FC: true, transport.ISCSI: true, transport.NVMe: true,
			transport.NFS: true, transport.Unknown: true,
		}

		prev := ""
		for i, ds := range dss {
			if ds.Name == "" {
				t.Errorf("datastore[%d] has empty name", i)
			}
			if !validTypes[ds.Type] {
				t.Errorf("datastore %q has invalid type %q", ds.Name, ds.Type)
			}
			if ds.Free > ds.Capacity {
				t.Errorf("datastore %q free %d > capacity %d", ds.Name, ds.Free, ds.Capacity)
			}
			if got := UsedCapacity(ds.Capacity, ds.Free); got != ds.Used {
				t.Errorf("datastore %q used %d, want UsedCapacity(capacity,free) = %d", ds.Name, ds.Used, got)
			}
			if delta := (ds.Used + ds.Free) - ds.Capacity; delta < -1 || delta > 1 {
				t.Errorf("datastore %q: used(%d) + free(%d) not consistent with capacity(%d), delta %d",
					ds.Name, ds.Used, ds.Free, ds.Capacity, delta)
			}
			if i > 0 && ds.Name < prev {
				t.Errorf("datastores not sorted by name: %q after %q", ds.Name, prev)
			}
			prev = ds.Name
		}
	}, model)
}

func TestUsedCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int64
		free     int64
		want     int64
	}{
		{"partially used", 1000, 400, 600},
		{"empty", 1000, 1000, 0},
		{"free exceeds capacity clamped", 1000, 1500, 0},
		{"zeros", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsedCapacity(tt.capacity, tt.free); got != tt.want {
				t.Errorf("UsedCapacity(%d, %d) = %d, want %d", tt.capacity, tt.free, got, tt.want)
			}
		})
	}
}
