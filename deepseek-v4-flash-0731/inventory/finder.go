package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// withDatacenters invokes fn once per datacenter with a finder anchored to
// that datacenter, so list("*") operations resolve without ambiguity in
// multi-datacenter inventories.
func withDatacenters(ctx context.Context, c *vim25.Client, fn func(*find.Finder) error) error {
	f := find.NewFinder(c, true)
	dcs, err := f.DatacenterList(ctx, "*")
	if err != nil {
		return fmt.Errorf("find datacenters: %w", err)
	}
	if len(dcs) == 0 {
		return fmt.Errorf("no datacenters in inventory")
	}
	for _, dc := range dcs {
		f.SetDatacenter(dc)
		if err := fn(f); err != nil {
			return err
		}
	}
	return nil
}

// dedupeRefs removes duplicate managed object references while preserving
// order.
func dedupeRefs(in []types.ManagedObjectReference) []types.ManagedObjectReference {
	out := make([]types.ManagedObjectReference, 0, len(in))
	seen := make(map[types.ManagedObjectReference]bool, len(in))
	for _, r := range in {
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}
