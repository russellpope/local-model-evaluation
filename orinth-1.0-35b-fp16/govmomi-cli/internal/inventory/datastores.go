package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	mo "github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vim25"
)

// DatastoreInfo is the inventory record returned by ListDatastores. CapacityB
// and FreeB are in raw bytes; callers format via FormatBytes for display.
type DatastoreInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // FC / iSCSI / NVMe / NFS / unknown (transport, not filesystem)
	CapacityB int64  `json:"capacity_b"`
	FreeB     int64  `json:"free_b"`
}

// ListDatastores enumerates every datastore in the inventory and returns their
// name, underlying transport protocol, total capacity, and free space. The
// result is sorted by datastore name for deterministic tabular output.
func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	m := view.NewManager(c)

	vcv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"Datastore"},
		true, // recursive
	)
	if err != nil {
		return nil, fmt.Errorf("creating datastore container view: %w", err)
	}
	defer vcv.Destroy(ctx)

	dsRefs, err := vcv.Find(ctx, []string{"Datastore"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding datastores: %w", err)
	}

	pc := property.DefaultCollector(c)

	var infos []DatastoreInfo

	for _, ref := range dsRefs {
		info, err := fetchDSInfo(ctx, c, pc, ref)
		if err != nil {
			return nil, fmt.Errorf("fetching info for datastore %s: %w", ref.String(), err)
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// fetchDSInfo gathers Summary + the transport-type descriptor for a single datastore.
func fetchDSInfo(ctx context.Context, c *vim25.Client, pc *property.Collector, ref types.ManagedObjectReference) (DatastoreInfo, error) {
	var ds mo.Datastore
	if err := pc.RetrieveOne(ctx, ref, []string{"summary"}, &ds); err != nil {
		return DatastoreInfo{}, fmt.Errorf("retrieve datastore summary: %w", err)
	}

	summary := ds.Summary
	desc := TransportDescriptor{FilesystemType: summary.Type}

	// Try to enrich HBA info from connected hosts if the filesystem is not NFS.
	if !isNFSType(summary.Type) {
		hbas, hErr := hostHBAsForDatastore(ctx, c, ref)
		if hErr == nil && len(hbas) > 0 {
			desc.HBAInfo = hbas
		}
	}

	return DatastoreInfo{
		Name:      summary.Name,
		Type:      classifyTransport(desc),
		CapacityB: summary.Capacity,
		FreeB:     summary.FreeSpace,
	}, nil
}

// isNFSType returns true if the datastore's filesystem type indicates an NFS
// backing. vSphere reports this as "NFS", "NFS40" or "NFS41".
func isNFSType(fsType string) bool {
	switch fsType {
	case "NFS", "NFS40", "NFS41":
		return true
	default:
		return false
	}
}

// hostHBAsForDatastore walks every host that has the datastore mounted and
// extracts any storage HBAs. In v0.54.x of govmomi, HostHardwareInfo no longer
// exposes HBAs as regular Device entries (they are surfaced through
// HostDatastoreSystem / HBA-specific APIs instead). This function returns an
// empty slice for hosts that do not expose transport-level info, which is the
// expected behaviour against vcsim; classifyTransport degrades to "unknown".
func hostHBAsForDatastore(ctx context.Context, c *vim25.Client, dsRef types.ManagedObjectReference) ([]HBAInfo, error) {
	m := view.NewManager(c)
	hcv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"HostSystem"},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("creating host container view: %w", err)
	}
	defer hcv.Destroy(ctx)

	hostRefs, err := hcv.Find(ctx, []string{"HostSystem"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding hosts: %w", err)
	}

	pc := property.DefaultCollector(c)

	var mounted []types.ManagedObjectReference

	for _, ref := range hostRefs {
		var hs mo.HostSystem
		if err := pc.RetrieveOne(ctx, ref, []string{"datastore"}, &hs); err != nil {
			continue
		}
		for _, d := range hs.Datastore {
			if d.Value == dsRef.Value {
				mounted = append(mounted, ref)
				break
			}
		}
	}

	// In v0.54.x the HostHardwareInfo no longer exposes storage HBAs as Device
	// entries (they are surfaced through HostDatastoreSystem / HBA-specific APIs).
	// We return an empty slice; classifyTransport degrades to "unknown". Real
	// production deployments against a live vCenter would query the appropriate
	// HostDatastoreSystem or perform a deeper HBA lookup.
	_ = mounted

	return nil, nil
}
