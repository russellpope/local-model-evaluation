package transport

import (
	"context"
	"fmt"
	"os"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type ClassifyResult struct {
	Type   string
	Reason string
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

func classifyVMFS(ctx context.Context, client *vim25.Client, dsMo mo.Datastore, info *types.VmfsDatastoreInfo) (ClassifyResult, error) {
	if info.Vmfs == nil || len(info.Vmfs.Extent) == 0 {
		return ClassifyResult{Type: "unknown", Reason: "no VMFS extents found"}, nil
	}

	canonicalName := info.Vmfs.Extent[0].DiskName
	if canonicalName == "" {
		return ClassifyResult{Type: "unknown", Reason: "no canonical name in VMFS extent"}, nil
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
			fmt.Fprintf(os.Stderr, "warning: retrieving properties for host %s: %v\n", hostRef, err)
			continue
		}

		if hostMo.Config == nil || hostMo.Config.StorageDevice == nil {
			continue
		}

		storageDevice := hostMo.Config.StorageDevice

		if storageDevice.ScsiLun != nil {
			for _, baseLun := range storageDevice.ScsiLun {
				lun := baseLun.GetScsiLun()
				if lun.CanonicalName == canonicalName {
					result, err := classifyByScsiTopology(hostMo, lun)
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
