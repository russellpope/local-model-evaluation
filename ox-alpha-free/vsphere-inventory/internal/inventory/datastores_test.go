package inventory

import (
	"context"
	"testing"
)

func TestListDatastores(t *testing.T) {
	ctx := context.Background()
	c, done := newTestClient(t)
	defer done()

	dss, err := ListDatastores(ctx, c)
	if err != nil {
		t.Fatalf("ListDatastores() error = %v", err)
	}

	const wantCount = 2 // Model.Datastore = 2
	if len(dss) != wantCount {
		t.Fatalf("ListDatastores() returned %d datastores, want %d: %v", len(dss), wantCount, dss)
	}

	validTypes := map[string]bool{"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true}

	for i, ds := range dss {
		if i > 0 && dss[i-1].Name >= ds.Name {
			t.Errorf("ListDatastores() not sorted by name at index %d: %q after %q", i, ds.Name, dss[i-1].Name)
		}
		if ds.Name == "" {
			t.Errorf("datastore %d has an empty name", i)
		}
		if !validTypes[ds.Type] {
			t.Errorf("datastore %q TYPE = %q, want one of FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Type)
		}
		// The simulator reports capacity but no consumption; used + available
		// must still reconcile with capacity and available must never exceed it.
		if ds.UsedBytes+ds.AvailableBytes != ds.CapacityBytes {
			t.Errorf("datastore %q: used(%d) + available(%d) != capacity(%d)",
				ds.Name, ds.UsedBytes, ds.AvailableBytes, ds.CapacityBytes)
		}
		if ds.AvailableBytes > ds.CapacityBytes {
			t.Errorf("datastore %q: available(%d) > capacity(%d)", ds.Name, ds.AvailableBytes, ds.CapacityBytes)
		}
		if ds.CapacityBytes <= 0 {
			t.Errorf("datastore %q has non-positive capacity %d", ds.Name, ds.CapacityBytes)
		}
	}

	// vcsim datastores are local VMFS-style disks with no fabric HBA paths,
	// so the transport must degrade to "unknown" rather than being guessed.
	for _, ds := range dss {
		if ds.Type != "unknown" {
			t.Errorf("datastore %q TYPE = %q; simulator LUNs have no FC/iSCSI/NVMe paths so want \"unknown\"", ds.Name, ds.Type)
		}
	}
}
