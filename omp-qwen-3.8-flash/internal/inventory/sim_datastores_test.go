package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25"
)

// TestGetDatastores validates the datastore table contract against the
// simulator: every row has a name, used+available reconstructs capacity,
// available never exceeds capacity, and TYPE is one of the five legal values.
// vcsim models local file-backed datastores only, so TYPE degrades to
// "unknown" — the graceful-degradation path required by the spec. Real
// FC/iSCSI/NVMe resolution is proven by TestClassifyTransport and the live
// derivation in hostDiskProtocols.
func TestGetDatastores(t *testing.T) {
	mustRun(t, vpxModel(2, 3, 1), func(ctx context.Context, c *vim25.Client) error {
		dss, err := Datastores(ctx, c)
		if err != nil {
			t.Fatalf("Datastores: %v", err)
		}
		if len(dss) != 3 {
			t.Fatalf("datastore count = %d, want 3", len(dss))
		}
		legal := map[string]bool{
			TransportFC: true, TransportiSCSI: true, TransportNVMe: true,
			TransportNFS: true, TransportUnknown: true,
		}
		for _, d := range dss {
			if d.Name == "" {
				t.Errorf("empty datastore name in %+v", d)
			}
			if !legal[d.Transport] {
				t.Errorf("datastore %s: transport %q not in %v", d.Name, d.Transport,
					[]string{"FC", "iSCSI", "NVMe", "NFS", "unknown"})
			}
			if d.Available > d.Capacity {
				t.Errorf("datastore %s: available %d > capacity %d", d.Name, d.Available, d.Capacity)
			}
			// used = capacity - available must hold exactly in int64.
			if got := d.Capacity - d.Available; got != d.Used {
				t.Errorf("datastore %s: used %d, want capacity-available = %d", d.Name, d.Used, got)
			}
		}
		for i := 1; i < len(dss); i++ {
			if dss[i-1].Name > dss[i].Name {
				t.Errorf("rows not sorted by name: %q before %q", dss[i-1].Name, dss[i].Name)
			}
		}
		return nil
	})
}

// TestDatastoreTransportNFS proves the filesystem-type branch: any datastore
// whose summary type is NFS/NFS4/NFS41 reports transport NFS. vcsim cannot
// create NFS datastores, so the derivation is exercised through the same
// helper the live path uses.
func TestDatastoreTransportNFS(t *testing.T) {
	for _, fs := range []string{"NFS", "nfs", "NFS4", "NFS41"} {
		if !isNfsFsType(fs) {
			t.Errorf("isNfsFsType(%q) = false, want true", fs)
		}
	}
	for _, fs := range []string{"VMFS", "vsan", "OTHER", ""} {
		if isNfsFsType(fs) {
			t.Errorf("isNfsFsType(%q) = true, want false", fs)
		}
	}
}
