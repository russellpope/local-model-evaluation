package transport

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func ClassifyDatastore(ctx context.Context, client *vim25.Client, dsMo mo.Datastore) (string, error) {
	switch info := dsMo.Info.(type) {
	case *types.NasDatastoreInfo:
		return "NFS", nil
	case *types.VmfsDatastoreInfo:
		return classifyVMFS(ctx, client, dsMo, info)
	case *types.LocalDatastoreInfo:
		return "unknown", nil
	case *types.VsanDatastoreInfo:
		return "unknown", nil
	default:
		return "unknown", nil
	}
}

func classifyVMFS(ctx context.Context, client *vim25.Client, dsMo mo.Datastore, info *types.VmfsDatastoreInfo) (string, error) {
	if info.Vmfs == nil || len(info.Vmfs.Extent) == 0 {
		return "unknown", fmt.Errorf("no VMFS extents found for datastore %s", dsMo.Name)
	}

	canonicalName := info.Vmfs.Extent[0].DiskName
	if canonicalName == "" {
		return "unknown", fmt.Errorf("no canonical name in VMFS extent for datastore %s", dsMo.Name)
	}

	for _, hostMount := range dsMo.Host {
		hostRef := hostMount.Key
		var hostMo mo.HostSystem
		hostObj := object.NewHostSystem(client, hostRef)
		err := hostObj.Properties(ctx, hostRef, []string{
			"name",
			"config.storageDevice",
		}, &hostMo)
		if err != nil {
			return "unknown", fmt.Errorf("retrieving properties for host %s: %w", hostRef, err)
		}

		if hostMo.Config == nil || hostMo.Config.StorageDevice == nil {
			continue
		}

		storageDevice := hostMo.Config.StorageDevice

		if storageDevice.ScsiLun != nil {
			for _, baseLun := range storageDevice.ScsiLun {
				lun := baseLun.GetScsiLun()
				if lun.CanonicalName == canonicalName {
					return classifyByScsiTopology(hostMo, lun)
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
					return "NVMe", nil
				}
			}
		}
	}

	return "unknown", fmt.Errorf("could not determine HBA for datastore %s (canonical name: %s)", dsMo.Name, canonicalName)
}

func classifyByScsiTopology(hostMo mo.HostSystem, lun *types.ScsiLun) (string, error) {
	storageDevice := hostMo.Config.StorageDevice
	if storageDevice.ScsiTopology == nil {
		return "unknown", fmt.Errorf("no SCSI topology on host %s", hostMo.Name)
	}

	for _, adapter := range storageDevice.ScsiTopology.Adapter {
		for _, target := range adapter.Target {
			for _, topoLun := range target.Lun {
				if topoLun.ScsiLun == lun.Key {
					hba := findHBAByKey(storageDevice, adapter.Adapter)
					if hba != nil {
						return classifyHBA(hba), nil
					}
				}
			}
		}
	}

	return "unknown", fmt.Errorf("could not find HBA for LUN %s", lun.Key)
}

func findHBAByKey(storageDevice *types.HostStorageDeviceInfo, adapterKey string) types.BaseHostHostBusAdapter {
	if storageDevice.HostBusAdapter == nil {
		return nil
	}
	for _, hba := range storageDevice.HostBusAdapter {
		switch h := hba.(type) {
		case *types.HostFibreChannelHba:
			if h.Key == adapterKey {
				return h
			}
		case *types.HostInternetScsiHba:
			if h.Key == adapterKey {
				return h
			}
		}
		if hba.GetHostHostBusAdapter().Key == adapterKey {
			return hba
		}
	}
	return nil
}

func classifyHBA(hba types.BaseHostHostBusAdapter) string {
	switch h := hba.(type) {
	case *types.HostFibreChannelHba:
		return "FC"
	case *types.HostInternetScsiHba:
		return "iSCSI"
	default:
		switch h.GetHostHostBusAdapter().StorageProtocol {
		case "nvme":
			return "NVMe"
		case "fcoe":
			return "FCoE"
		}
		return "unknown"
	}
}
