package transport

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type ClassifyResult struct {
	Type   string
	Reason string
}

type HostCache struct {
	client *vim25.Client
	mu     sync.RWMutex
	hosts  map[types.ManagedObjectReference]*hostEntry
}

type hostEntry struct {
	hostMo *mo.HostSystem
}

func NewHostCache(client *vim25.Client) *HostCache {
	return &HostCache{
		client: client,
		hosts:  make(map[types.ManagedObjectReference]*hostEntry),
	}
}

func (c *HostCache) prefetch(ctx context.Context, refs []types.ManagedObjectReference) error {
	if len(refs) == 0 {
		return nil
	}

	var missing []types.ManagedObjectReference
	c.mu.RLock()
	for _, ref := range refs {
		if _, ok := c.hosts[ref]; !ok {
			missing = append(missing, ref)
		}
	}
	c.mu.RUnlock()

	if len(missing) == 0 {
		return nil
	}

	pc := property.DefaultCollector(c.client)
	var hostMos []mo.HostSystem
	if err := pc.Retrieve(ctx, missing, []string{"name", "config.storageDevice"}, &hostMos); err != nil {
		return fmt.Errorf("batch retrieving host properties: %w", err)
	}

	c.mu.Lock()
	for i := range hostMos {
		ref := hostMos[i].Reference()
		c.hosts[ref] = &hostEntry{hostMo: &hostMos[i]}
	}
	c.mu.Unlock()

	return nil
}

func (c *HostCache) PrefetchAll(ctx context.Context) error {
	vm := view.NewManager(c.client)
	v, err := vm.CreateContainerView(ctx, c.client.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return fmt.Errorf("creating container view: %w", err)
	}
	defer v.Destroy(ctx)

	hostRefs, err := v.Find(ctx, []string{"HostSystem"}, property.Match{"name": "*"})
	if err != nil {
		return fmt.Errorf("finding host systems: %w", err)
	}

	return c.prefetch(ctx, hostRefs)
}

func (c *HostCache) get(ref types.ManagedObjectReference) (*mo.HostSystem, bool) {
	c.mu.RLock()
	entry, ok := c.hosts[ref]
	c.mu.RUnlock()
	if ok && entry != nil {
		return entry.hostMo, true
	}
	return nil, false
}

func (c *HostCache) prefetchMissing(ctx context.Context, refs []types.ManagedObjectReference) error {
	return c.prefetch(ctx, refs)
}

func ClassifyDatastore(ctx context.Context, client *vim25.Client, dsMo mo.Datastore) (ClassifyResult, error) {
	switch info := dsMo.Info.(type) {
	case *types.NasDatastoreInfo:
		return ClassifyResult{Type: "NFS"}, nil
	case *types.VmfsDatastoreInfo:
		return classifyVMFS(ctx, client, dsMo, info)
	case *types.LocalDatastoreInfo:
		return ClassifyResult{Type: "unknown"}, nil
	case *types.VsanDatastoreInfo:
		return ClassifyResult{Type: "unknown"}, nil
	default:
		return ClassifyResult{Type: "unknown"}, nil
	}
}

func ClassifyDatastoreWithCache(ctx context.Context, client *vim25.Client, dsMo mo.Datastore, cache *HostCache) (ClassifyResult, error) {
	switch info := dsMo.Info.(type) {
	case *types.NasDatastoreInfo:
		return ClassifyResult{Type: "NFS"}, nil
	case *types.VmfsDatastoreInfo:
		return classifyVMFSWithCache(ctx, client, dsMo, info, cache)
	case *types.LocalDatastoreInfo:
		return ClassifyResult{Type: "unknown"}, nil
	case *types.VsanDatastoreInfo:
		return ClassifyResult{Type: "unknown"}, nil
	default:
		return ClassifyResult{Type: "unknown"}, nil
	}
}

func classifyVMFS(ctx context.Context, client *vim25.Client, dsMo mo.Datastore, info *types.VmfsDatastoreInfo) (ClassifyResult, error) {
	cache := NewHostCache(client)
	return classifyVMFSWithCache(ctx, client, dsMo, info, cache)
}

func classifyVMFSWithCache(ctx context.Context, client *vim25.Client, dsMo mo.Datastore, info *types.VmfsDatastoreInfo, cache *HostCache) (ClassifyResult, error) {
	if info.Vmfs == nil || len(info.Vmfs.Extent) == 0 {
		return ClassifyResult{Type: "unknown", Reason: "no VMFS extents found"}, nil
	}

	canonicalName := info.Vmfs.Extent[0].DiskName
	if canonicalName == "" {
		return ClassifyResult{Type: "unknown", Reason: "no canonical name in VMFS extent"}, nil
	}

	var hostRefs []types.ManagedObjectReference
	for _, hostMount := range dsMo.Host {
		hostRefs = append(hostRefs, hostMount.Key)
	}

	if err := cache.prefetchMissing(ctx, hostRefs); err != nil {
		fmt.Fprintf(os.Stderr, "warning: batch retrieving host properties: %v\n", err)
	}

	for _, hostMount := range dsMo.Host {
		hostRef := hostMount.Key
		hostMo, ok := cache.get(hostRef)
		if !ok {
			var fallback mo.HostSystem
			hostObj := object.NewHostSystem(client, hostRef)
			err := hostObj.Properties(ctx, hostRef, []string{
				"name",
				"config.storageDevice",
			}, &fallback)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: retrieving properties for host %s: %v\n", hostRef, err)
				continue
			}
			hostMo = &fallback
		}

		if hostMo.Config == nil || hostMo.Config.StorageDevice == nil {
			continue
		}

		storageDevice := hostMo.Config.StorageDevice

		if storageDevice.ScsiLun != nil {
			for _, baseLun := range storageDevice.ScsiLun {
				lun := baseLun.GetScsiLun()
				if lun.CanonicalName == canonicalName {
					result, err := classifyByScsiTopology(*hostMo, lun)
					if err != nil {
						return ClassifyResult{Type: "unknown", Reason: err.Error()}, nil
					}
					return result, nil
				}
			}
		}

		if storageDevice.NvmeTopology != nil {
			for _, iface := range storageDevice.NvmeTopology.Adapter {
				if iface.Adapter == "" {
					continue
				}
				hba := findHBAByKey(storageDevice, iface.Adapter)
				if hba == nil {
					continue
				}
				if classifyHBA(hba) == "NVMe" && len(iface.ConnectedController) > 0 {
					for _, controller := range iface.ConnectedController {
						for _, ns := range controller.AttachedNamespace {
							if ns.Name == canonicalName {
								return ClassifyResult{Type: "NVMe"}, nil
							}
						}
					}
				}
			}
		}
	}

	return ClassifyResult{Type: "unknown", Reason: fmt.Sprintf("could not determine HBA for datastore (canonical name: %s)", canonicalName)}, nil
}

func classifyByScsiTopology(hostMo mo.HostSystem, lun *types.ScsiLun) (ClassifyResult, error) {
	storageDevice := hostMo.Config.StorageDevice
	if storageDevice.ScsiTopology == nil {
		return ClassifyResult{Type: "unknown", Reason: "no SCSI topology on host " + hostMo.Name}, nil
	}

	for _, adapter := range storageDevice.ScsiTopology.Adapter {
		for _, target := range adapter.Target {
			for _, topoLun := range target.Lun {
				if topoLun.ScsiLun == lun.Key {
					hba := findHBAByKey(storageDevice, adapter.Adapter)
					if hba != nil {
						return ClassifyResult{Type: classifyHBA(hba)}, nil
					}
				}
			}
		}
	}

	return ClassifyResult{Type: "unknown", Reason: "could not find HBA for LUN " + lun.Key}, nil
}

func findHBAByKey(storageDevice *types.HostStorageDeviceInfo, adapterKey string) types.BaseHostHostBusAdapter {
	if storageDevice.HostBusAdapter == nil {
		return nil
	}
	for _, hba := range storageDevice.HostBusAdapter {
		if hba.GetHostHostBusAdapter().Key == adapterKey {
			return hba
		}
	}
	return nil
}

func classifyHBA(hba types.BaseHostHostBusAdapter) string {
	switch h := hba.(type) {
	case *types.HostFibreChannelOverEthernetHba:
		return "FC"
	case *types.HostFibreChannelHba:
		return "FC"
	case *types.HostInternetScsiHba:
		return "iSCSI"
	default:
		switch h.GetHostHostBusAdapter().StorageProtocol {
		case "nvme":
			return "NVMe"
		}
		return "unknown"
	}
}
