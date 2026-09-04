package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
)

func containerRetrieve(ctx context.Context, c *vim25.Client, kinds []string, props []string, dst interface{}) error {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, kinds, true)
	if err != nil {
		return fmt.Errorf("create container view for %v: %w", kinds, err)
	}
	defer v.Destroy(ctx)
	if err := v.Retrieve(ctx, kinds, props, dst); err != nil {
		return fmt.Errorf("retrieve %v properties: %w", kinds, err)
	}
	return nil
}
