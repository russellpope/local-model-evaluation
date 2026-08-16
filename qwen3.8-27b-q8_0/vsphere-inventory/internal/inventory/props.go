package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

// findRefs returns the references of every managed object of the given types
// anywhere in the inventory. It creates a recursive container view over the
// root folder, so the result is not scoped to a single datacenter and covers
// the whole vCenter. The view is destroyed before returning.
func findRefs(ctx context.Context, c *vim25.Client, kinds ...string) ([]types.ManagedObjectReference, error) {
	mgr := view.NewManager(c)
	cv, err := mgr.CreateContainerView(ctx, c.ServiceContent.RootFolder, kinds, true)
	if err != nil {
		return nil, fmt.Errorf("create container view: %w", err)
	}
	defer func() { _ = cv.Destroy(ctx) }()

	refs, err := cv.Find(ctx, kinds, nil)
	if err != nil {
		return nil, fmt.Errorf("find %v: %w", kinds, err)
	}
	return refs, nil
}

// retrieveRaw fetches the given property paths for the given objects and
// returns a map keyed by the object reference value. It is used for object
// types that govmomi does not model as mo structs (for example
// HostVirtualSwitch and HostPortGroup), so the raw ObjectContent values are
// returned for the caller to type-switch.
func retrieveRaw(ctx context.Context, c *vim25.Client, refs []types.ManagedObjectReference, objType string, paths []string) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(refs))
	if len(refs) == 0 {
		return out, nil
	}

	objSpecs := make([]types.ObjectSpec, 0, len(refs))
	for _, r := range refs {
		objSpecs = append(objSpecs, types.ObjectSpec{
			Obj:  r,
			Skip: types.NewBool(true),
		})
	}

	req := types.RetrieveProperties{
		This: c.ServiceContent.PropertyCollector,
		SpecSet: []types.PropertyFilterSpec{
			{
				ObjectSet: objSpecs,
				PropSet: []types.PropertySpec{
					{
						Type:    objType,
						PathSet: paths,
					},
				},
			},
		},
	}

	res, err := methods.RetrieveProperties(ctx, c, &req)
	if err != nil {
		return nil, fmt.Errorf("retrieve %s properties: %w", objType, err)
	}

	for _, content := range res.Returnval {
		props := make(map[string]any, len(content.PropSet))
		for _, p := range content.PropSet {
			props[p.Name] = p.Val
		}
		out[content.Obj.Value] = props
	}

	return out, nil
}

// retrieve loads properties for the given objects into dst, which must be a
// pointer to a slice of mo types (for example *[]mo.VirtualMachine).
func retrieve(ctx context.Context, c *vim25.Client, refs []types.ManagedObjectReference, paths []string, dst any) error {
	if len(refs) == 0 {
		return nil
	}
	if err := property.DefaultCollector(c).Retrieve(ctx, refs, paths, dst); err != nil {
		return fmt.Errorf("retrieve properties: %w", err)
	}
	return nil
}
