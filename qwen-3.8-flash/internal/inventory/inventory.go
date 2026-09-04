// Package inventory implements the vSphere inventory queries behind each
// subcommand. All entry points take a context and a vim25 client and return
// typed results, separate from Cobra wiring and tabwriter presentation.
package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// listKind returns references to every managed object of the given MO kind
// ("VirtualMachine", "Datastore", "HostSystem", ...) anywhere in the
// inventory.
func listKind(ctx context.Context, c *vim25.Client, kind string) ([]types.ManagedObjectReference, error) {
	m := view.NewManager(c)
	cv, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{kind}, true)
	if err != nil {
		return nil, fmt.Errorf("create inventory view for %s: %w", kind, err)
	}
	defer func() { _ = cv.Destroy(ctx) }()
	refs, err := cv.Find(ctx, []string{kind}, nil)
	if err != nil {
		return nil, fmt.Errorf("enumerate %s objects: %w", kind, err)
	}
	return refs, nil
}

// collect retrieves the given properties of refs into dst, which must be a
// pointer to a slice of the matching mo.* struct.
func collect(ctx context.Context, c *vim25.Client, refs []types.ManagedObjectReference, ps []string, dst any) error {
	pc := property.DefaultCollector(c)
	if err := pc.Retrieve(ctx, refs, ps, dst); err != nil {
		return fmt.Errorf("retrieve %v: %w", ps, err)
	}
	return nil
}
