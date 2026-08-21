package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// distributedSwitches returns every distributed switch in the default datacenter
// by enumerating the datacenter network folder children.
//
// The simulator does not implement view.ContainerView.Retrieve, so the objects
// are resolved directly rather than through a container view. Port groups are
// resolved from each switch's Portgroup references.
func distributedSwitches(ctx context.Context, client *vim25.Client) ([]mo.DistributedVirtualSwitch, error) {
	finder, err := newFinder(ctx, client)
	if err != nil {
		return nil, err
	}
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolving default datacenter: %w", err)
	}
	folders, err := dc.Folders(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading datacenter folders: %w", err)
	}

	var folder mo.Folder
	if err := property.DefaultCollector(client).RetrieveOne(ctx, folders.NetworkFolder.Reference(), nil, &folder); err != nil {
		return nil, fmt.Errorf("reading network folder: %w", err)
	}

	pc := property.DefaultCollector(client)
	var dvsList []mo.DistributedVirtualSwitch
	for _, ref := range folder.ChildEntity {
		if ref.Type != "DistributedVirtualSwitch" {
			continue
		}
		var dvs mo.DistributedVirtualSwitch
		if err := pc.RetrieveOne(ctx, ref, nil, &dvs); err != nil {
			return nil, fmt.Errorf("reading distributed switch %q: %w", ref.Value, err)
		}
		dvsList = append(dvsList, dvs)
	}
	return dvsList, nil
}
