package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
)

// TestDatastores verifies capacity math and the transport vocabulary
// against a model with a known datastore count.
func TestDatastores(t *testing.T) {
	const wantDatastores = 3

	runSimulator(t, func(m *simulator.Model) {
		m.Datastore = wantDatastores
	}, func(ctx context.Context, c *govmomi.Client) {
		dss, err := ListDatastores(ctx, c)
		if err != nil {
			t.Fatalf("ListDatastores: %v", err)
		}
		if len(dss) != wantDatastores {
			t.Fatalf("got %d datastores, want %d", len(dss), wantDatastores)
		}

		valid := map[string]bool{
			TransportFC:      true,
			TransportISCSI:   true,
			TransportNVMe:    true,
			TransportNFS:     true,
			TransportUnknown: true,
		}

		for i, ds := range dss {
			if ds.Name == "" {
				t.Errorf("datastore %d: empty name", i)
			}
			if !valid[ds.Transport] {
				t.Errorf("%s: transport %q not in FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Transport)
			}
			if ds.AvailableBytes > ds.CapacityBytes {
				t.Errorf("%s: available (%d) exceeds capacity (%d)", ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}
			if got := ds.UsedBytes + ds.AvailableBytes; got != ds.CapacityBytes {
				t.Errorf("%s: used (%d) + available (%d) = %d, want capacity %d",
					ds.Name, ds.UsedBytes, ds.AvailableBytes, got, ds.CapacityBytes)
			}
			if ds.CapacityBytes <= 0 {
				t.Errorf("%s: capacity = %d, want > 0", ds.Name, ds.CapacityBytes)
			}
		}

		// Simulator datastores are local volumes with no fabric topology,
		// so the only truthful transport is unknown.
		for _, ds := range dss {
			if ds.Transport != TransportUnknown {
				t.Errorf("%s: simulator transport = %q, want unknown", ds.Name, ds.Transport)
			}
		}

		// Sorted by name.
		for i := 1; i < len(dss); i++ {
			if dss[i-1].Name > dss[i].Name {
				t.Fatalf("datastores not sorted: %q before %q", dss[i-1].Name, dss[i].Name)
			}
		}
	})
}
