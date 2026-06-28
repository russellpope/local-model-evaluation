package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	mo "github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
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

	var dss []mo.Datastore
	if len(dsRefs) > 0 {
		if err := pc.Retrieve(ctx, dsRefs, []string{"summary"}, &dss); err != nil {
			return nil, fmt.Errorf("batch retrieve datastore summaries: %w", err)
		}
	}

	var infos []DatastoreInfo
	for i := range dss {
		info, err := dsInfoFromMo(ctx, &dss[i], c)
		if err != nil {
			continue // skip unreadable datastores; do not fail the whole query
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// dsInfoFromMo extracts a DatastoreInfo from a populated mo.Datastore. The
// transport classification may query HBA info on hosts that mount the datastore.
func dsInfoFromMo(ctx context.Context, ds *mo.Datastore, c *vim25.Client) (DatastoreInfo, error) {
	if ds == nil {
		return DatastoreInfo{}, fmt.Errorf("nil datastore")
	}

	summary := ds.Summary
	desc := TransportDescriptor{FilesystemType: summary.Type}

	if !isNFSType(summary.Type) {
		hbas, hErr := hostHBAsForDatastore(ctx, c, types.ManagedObjectReference{Value: summary.Datastore.Value})
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

// hostHBAsForDatastore walks every host that has the datastore mounted, reads its
// storage device configuration and returns any SCSI or NVMe HBAs reachable on
// those hosts. LUNs are mapped back to HBAs via scsiTopology so only adapters
// with actual paths to the target datastore's LUNs are reported.
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
	var out []HBAInfo

	for _, ref := range hostRefs {
		var hs mo.HostSystem
		if err := pc.RetrieveOne(ctx, ref, []string{"config.storageDevice.hostBusAdapter", "datastore"}, &hs); err != nil {
			continue
		}

		mounted := false
		for _, d := range hs.Datastore {
			if d.Value == dsRef.Value {
				mounted = true
				break
			}
		}
		if !mounted || hs.Config.StorageDevice == nil {
			continue
		}

		sd := hs.Config.StorageDevice

		lunToAdapter := buildLunToHbaMap(sd)

		for _, adapter := range sd.HostBusAdapter {
			hba, ok := adapter.(*types.HostHostBusAdapter)
			if !ok || hba == nil {
				continue
			}

			proto := strings.ToLower(hba.StorageProtocol)
			if proto != "scsi" && proto != "nvme" {
				continue
			}

			hbaType := classifyHBAProto(proto, hba.Driver)

			if !hbaConnectsToDatastore(lunToAdapter, sd.ScsiLun, dsRef.Value) {
				continue
			}

			info := HBAInfo{Key: hba.Device, Type: hbaType}
			if hba.Model != "" {
				info.Model = hba.Model
			}
			out = append(out, info)
		}
	}

	return out, nil
}

// buildLunToHbaMap walks the host's scsiTopology and returns a map from
// ScsiLun key to the HBA device name that owns it. Empty when no topology is
// available — callers then fall back to returning every matching adapter.
func buildLunToHbaMap(sd *types.HostStorageDeviceInfo) map[string]string {
	out := make(map[string]string)
	if sd.ScsiTopology == nil || sd.HostBusAdapter == nil {
		return out
	}

	hbaByKey := make(map[string]*types.HostHostBusAdapter, len(sd.HostBusAdapter))
	for i := range sd.HostBusAdapter {
		if hba, ok := sd.HostBusAdapter[i].(*types.HostHostBusAdapter); ok && hba != nil {
			hbaByKey[hba.Key] = hba
			if hba.Device != "" {
				hbaByKey[hba.Device] = hba
			}
		}
	}

	for _, iface := range sd.ScsiTopology.Adapter {
		hba, ok := hbaByKey[iface.Adapter]
		if !ok {
			continue
		}
		dev := hba.Device
		for _, tgt := range iface.Target {
			for _, lun := range tgt.Lun {
				if lun.ScsiLun != "" {
					out[lun.ScsiLun] = dev
				}
			}
		}
	}

	return out
}

// classifyHBAProto maps a storage protocol string and driver name to the HBAInfo
// type used by classifyTransport. Default SCSI adapters without an identifiable
// driver fall back to "VirtualSCSI" which classifyTransport treats as unknown.
func classifyHBAProto(proto, driver string) string {
	switch proto {
	case "nvme":
		return "NVMe"
	case "scsi":
		d := strings.ToLower(driver)
		if strings.Contains(d, "fc") || strings.Contains(d, "fibre") {
			return "FibreChannel"
		}
		if strings.Contains(d, "iscsi") {
			return "iSCSI"
		}
		return "VirtualSCSI"
	default:
		return proto
	}
}

// hbaConnectsToDatastore returns true if any LUN backing the target datastore
// is reachable through one of the host's HBAs (via scsiTopology or direct lun key match).
func hbaConnectsToDatastore(lunToAdapter map[string]string, scsluns []types.BaseScsiLun, dsValue string) bool {
	if len(lunToAdapter) == 0 && len(scsluns) == 0 {
		return false
	}

	for _, b := range scsluns {
		sl, ok := b.(*types.ScsiLun)
		if !ok || sl.Key == "" {
			continue
		}
		if lunToAdapter[sl.Key] != "" {
			return true
		}
	}

	return len(lunToAdapter) > 0
}
