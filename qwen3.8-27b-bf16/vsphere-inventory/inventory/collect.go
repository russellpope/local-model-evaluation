package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// rootView creates a recursive container view over the entire inventory,
// scoped to the given managed object types. Types must be listed explicitly
// (including every subtype of interest), because a container view does not
// expand subtypes on its own.
func rootView(ctx context.Context, c *vim25.Client, viewTypes []string) (*view.ContainerView, error) {
	manager := view.NewManager(c)
	v, err := manager.CreateContainerView(ctx, c.ServiceContent.RootFolder, viewTypes, true)
	if err != nil {
		return nil, fmt.Errorf("create container view: %w", err)
	}
	return v, nil
}

// viewRefs returns the references of all entities in a root view.
func viewRefs(ctx context.Context, c *vim25.Client, viewTypes []string) ([]types.ManagedObjectReference, error) {
	v, err := rootView(ctx, c, viewTypes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = v.Destroy(ctx) }()
	refs, err := v.Find(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	return refs, nil
}
