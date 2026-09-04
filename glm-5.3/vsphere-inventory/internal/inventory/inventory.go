// Package inventory retrieves virtualization inventory from a vCenter
// Server. All entry points take a context and an authenticated govmomi
// client and return typed results, keeping them separate from both the CLI
// wiring and the presentation layer so they can be unit-tested directly
// against govmomi's embedded simulator.
package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/view"
)

// newContainerView creates a recursive container view rooted at the
// inventory root folder for the given managed entity types.
func newContainerView(ctx context.Context, c *govmomi.Client, kinds []string) (*view.ContainerView, error) {
	mgr := view.NewManager(c.Client)
	v, err := mgr.CreateContainerView(ctx, c.ServiceContent.RootFolder, kinds, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for %v: %w", kinds, err)
	}
	return v, nil
}
