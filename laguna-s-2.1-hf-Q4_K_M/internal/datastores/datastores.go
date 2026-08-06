package datastores

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/transport"
)

type DatastoreInfo struct {
	Name     string
	Type     transport.Transport
	Capacity int64
	Free     int64
	Used     int64
}

func GetDatastores(ctx context.Context, client *vim25.Client) ([]DatastoreInfo, error) {
	finder := find.NewFinder(client)

	datastores, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}

	var results []DatastoreInfo

	for _, ds := range datastores {
		info, err := getDatastoreInfo(ctx, client, ds)
		if err != nil {
			return nil, fmt.Errorf("get datastore info for %q: %w", ds.Name(), err)
		}
		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func getDatastoreInfo(ctx context.Context, client *vim25.Client, ds *object.Datastore) (DatastoreInfo, error) {
	var dsMo mo.Datastore

	props := []string{"name", "summary", "info", "host"}

	if err := ds.Properties(ctx, ds.Reference(), props, &dsMo); err != nil {
		return DatastoreInfo{}, fmt.Errorf("retrieve datastore properties: %w", err)
	}

	info := DatastoreInfo{
		Name:     dsMo.Name,
		Capacity: dsMo.Summary.Capacity,
		Free:     dsMo.Summary.FreeSpace,
	}

	info.Used = info.Capacity - info.Free

	if dsMo.Summary.Type == "NFS" {
		info.Type = transport.ClassifyNFSTransport()
	} else {
		info.Type = classifyVMFSTransport(ctx, client, &dsMo)
	}

	return info, nil
}

func classifyVMFSTransport(ctx context.Context, client *vim25.Client, dsMo *mo.Datastore) transport.Transport {
	if len(dsMo.Host) == 0 {
		return transport.TransportUnknown
	}

	hostRef := dsMo.Host[0].Key

	host := object.NewHostSystem(client, hostRef)

	var hostMo mo.HostSystem

	props := []string{"name", "config.storageDevice"}

	if err := host.Properties(ctx, hostRef, props, &hostMo); err != nil {
		return transport.TransportUnknown
	}

	if hostMo.Config == nil || hostMo.Config.StorageDevice == nil {
		return transport.TransportUnknown
	}

	sd := hostMo.Config.StorageDevice

	if len(sd.HostBusAdapter) > 0 {
		for _, hba := range sd.HostBusAdapter {
			t := transport.ClassifyHBAFromInterface(hba)
			if t != transport.TransportUnknown {
				return t
			}
		}
	}

	if sd.ScsiTopology != nil {
		for _, iface := range sd.ScsiTopology.Adapter {
			for _, target := range iface.Target {
				if target.Transport != nil {
					t := classifyTargetTransport(target.Transport)
					if t != transport.TransportUnknown {
						return t
					}
				}
			}
		}
	}

	return transport.TransportUnknown
}

func classifyTargetTransport(t types.BaseHostTargetTransport) transport.Transport {
	switch t.(type) {
	case *types.HostFibreChannelTargetTransport:
		return transport.TransportFC
	case *types.HostInternetScsiTargetTransport:
		return transport.TransportISCSI
	case *types.HostPcieTargetTransport:
		return transport.TransportNVMe
	case *types.HostBlockAdapterTargetTransport:
		return transport.TransportUnknown
	default:
		return transport.TransportUnknown
	}
}
