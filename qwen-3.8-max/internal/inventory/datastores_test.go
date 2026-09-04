package inventory_test

import (
	"context"
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/internal/inventory"
)

func TestDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Datastore = 3

	validTransport := map[string]bool{
		"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true,
	}

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		want := model.Count().Datastore
		if want == 0 {
			t.Fatal("simulator model has no datastores")
		}

		infos, err := inventory.ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(infos) != want {
			t.Fatalf("ListDatastores returned %d datastores, want %d", len(infos), want)
		}
		for _, ds := range infos {
			if ds.Name == "" {
				t.Error("datastore with empty name")
			}
			if !validTransport[ds.Transport] {
				t.Errorf("datastore %q: transport %q not in FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Transport)
			}
			if ds.AvailableBytes > ds.CapacityBytes {
				t.Errorf("datastore %q: available %d > capacity %d", ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}
			if ds.AvailableBytes < 0 {
				t.Errorf("datastore %q: available %d < 0", ds.Name, ds.AvailableBytes)
			}
			if got := ds.UsedBytes + ds.AvailableBytes; got != ds.CapacityBytes {
				t.Errorf("datastore %q: used %d + available %d = %d, want capacity %d",
					ds.Name, ds.UsedBytes, ds.AvailableBytes, got, ds.CapacityBytes)
			}
		}
		if !sort.SliceIsSorted(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name }) {
			t.Error("datastores are not sorted by name")
		}
	}, model)
}
