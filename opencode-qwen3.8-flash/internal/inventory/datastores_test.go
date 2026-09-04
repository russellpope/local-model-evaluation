package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestFetchDatastores(t *testing.T) {
	m := simulator.VPX()
	m.Cluster = 0
	m.Host = 1
	m.Machine = 0
	m.Portgroup = 0
	m.Datastore = 3

	runModel(t, m, func(ctx context.Context, c *vim25.Client) error {
		dss, err := FetchDatastores(ctx, c)
		if err != nil {
			return fmt.Errorf("FetchDatastores: %w", err)
		}
		if len(dss) != 3 {
			return fmt.Errorf("FetchDatastores returned %d datastores, want 3", len(dss))
		}
		known := map[string]bool{
			TransportFC: true, TransportISCSI: true, TransportNVMe: true,
			TransportNFS: true, TransportUnknown: true,
		}
		for i, ds := range dss {
			if ds.Name == "" {
				return fmt.Errorf("datastore %d has empty name", i)
			}
			if !known[ds.Type] {
				return fmt.Errorf("datastore %q: Type = %q, want one of FC/iSCSI/NVMe/NFS/unknown", ds.Name, ds.Type)
			}
			if ds.Available < 0 || ds.Available > ds.Capacity {
				return fmt.Errorf("datastore %q: Available = %d out of [0, Capacity=%d]", ds.Name, ds.Available, ds.Capacity)
			}
			if ds.Used < 0 {
				return fmt.Errorf("datastore %q: Used = %d, want >= 0", ds.Name, ds.Used)
			}
			sum := ds.Used + ds.Available
			if diff := sum - ds.Capacity; diff > 1 || diff < -1 {
				return fmt.Errorf("datastore %q: Used(%d)+Available(%d) != Capacity(%d)", ds.Name, ds.Used, ds.Available, ds.Capacity)
			}
		}
		for i := 1; i < len(dss); i++ {
			if dss[i-1].Name > dss[i].Name {
				return fmt.Errorf("datastores not sorted: %q after %q", dss[i].Name, dss[i-1].Name)
			}
		}
		return nil
	})
}
