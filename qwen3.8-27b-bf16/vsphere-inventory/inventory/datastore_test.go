package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

func TestListDatastores(t *testing.T) {
	withClient(t, func(ctx context.Context, c *vim25.Client) {
		dss, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(dss) != 2 {
			t.Fatalf("expected 2 datastores, got %d: %+v", len(dss), dss)
		}

		knownTypes := map[string]bool{
			TransportFC:      true,
			TransportISCSI:   true,
			TransportNVMe:    true,
			TransportNFS:     true,
			TransportUnknown: true,
		}

		for _, ds := range dss {
			if ds.Name == "" {
				t.Fatalf("datastore with empty name: %+v", ds)
			}
			if !knownTypes[ds.Type] {
				t.Fatalf("datastore %s: type %q, want one of FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Type)
			}
			if ds.AvailableBytes > ds.CapacityBytes {
				t.Fatalf("datastore %s: available %d > capacity %d", ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}
			if ds.UsedBytes < 0 {
				t.Fatalf("datastore %s: used %d < 0", ds.Name, ds.UsedBytes)
			}
			if got := ds.UsedBytes + ds.AvailableBytes; got != ds.CapacityBytes {
				t.Fatalf("datastore %s: used+available = %d, want capacity %d", ds.Name, got, ds.CapacityBytes)
			}
		}

		for i := 1; i < len(dss); i++ {
			if dss[i-1].Name >= dss[i].Name {
				t.Fatalf("datastores not sorted by name: %q before %q", dss[i-1].Name, dss[i].Name)
			}
		}
	})
}
