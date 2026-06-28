package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vim25 "github.com/vmware/govmomi/vim25"
)

func TestListDatastores_Simulator(t *testing.T) {
	model := simulator.VPX()
	model.Datastore = 3 // three datastores per host; total >= 3.

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		dsList, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}

		if len(dsList) == 0 {
			t.Fatal("ListDatastores returned no datastores")
		}

		for _, ds := range dsList {
			if ds.Name == "" {
				t.Error("ListDatastores: empty datastore name")
			}

			switch ds.Type {
			case "FC", "iSCSI", "NVMe", "NFS", "unknown":
				// valid transport type
			default:
				t.Errorf("ListDatastores %q: TYPE = %q, want one of FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Type)
			}

			if ds.FreeB > ds.CapacityB {
				t.Errorf("ListDatastores %q: free (%d) > capacity (%d)", ds.Name, ds.FreeB, ds.CapacityB)
			}

			used := UsedFromCapacity(ds.CapacityB, ds.FreeB)
			if used+ds.FreeB != ds.CapacityB && ds.FreeB <= ds.CapacityB {
				// within rounding: used + available should approximately equal capacity.
				diff := used + ds.FreeB - ds.CapacityB
				if diff < 0 {
					diff = -diff
				}
				if diff > 1<<20 { // allow up to 1 MiB of rounding drift
					t.Errorf("ListDatastores %q: used(%d)+free(%d)=%d != capacity(%d)", ds.Name, used, ds.FreeB, used+ds.FreeB, ds.CapacityB)
				}
			}

			if ds.CapacityB < 0 {
				t.Errorf("ListDatastores %q: CapacityB = %d, want >= 0", ds.Name, ds.CapacityB)
			}
		}

		// Verify sort order.
		for i := 1; i < len(dsList); i++ {
			if dsList[i-1].Name > dsList[i].Name {
				t.Errorf("ListDatastores: not sorted by name at index %d (%q > %q)", i, dsList[i-1].Name, dsList[i].Name)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("simulator.Run: %v", err)
	}
}
