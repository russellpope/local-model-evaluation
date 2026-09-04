// Package inventory retrieves typed vSphere inventory data.
//
// Every fetch function takes a context and a connected *vim25.Client and
// returns typed results, independent of Cobra wiring and table rendering.
package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
)

// retrieve collects properties for every object of the given managed-object
// kind into dst, which must be a pointer to a slice of the matching mo type.
func retrieve(ctx context.Context, c *vim25.Client, kind string, props []string, dst any) error {
	m := view.NewManager(c)
	cv, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{kind}, true)
	if err != nil {
		return fmt.Errorf("create %s container view: %w", kind, err)
	}
	defer func() { _ = cv.Destroy(ctx) }()

	if err := cv.Retrieve(ctx, []string{kind}, props, dst); err != nil {
		return fmt.Errorf("retrieve %s properties: %w", kind, err)
	}
	return nil
}
