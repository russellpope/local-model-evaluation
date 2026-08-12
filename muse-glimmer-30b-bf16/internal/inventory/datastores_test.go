package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestGetDatastores(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()
	simulator.Run(func(ctx context.Context, c *vim25.Client) error {
		ds, err := GetDatastores(ctx, c)
		if err != nil {
			return err
		}
		if len(ds) == 0 {
			t.Fatalf("expected datastores")
		}
		allowed := map[string]bool{"FC": true, "iSCSI": true, "NVMe": true, "NFS": true, "unknown": true}
		for _, d := range ds {
			if d.Name == "" {
				t.Errorf("empty name")
			}
			if !allowed[d.Type] {
				t.Errorf("unexpected type %s", d.Type)
			}
		}
		return nil
	}, model)
}
