package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/transport"
)

type DatastoreInfo struct {
	Name           string
	Transport      string
	CapacityBytes  int64
	UsedBytes      int64
	AvailableBytes int64
}

func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	var stores []mo.Datastore
	if err := containerRetrieve(ctx, c, []string{"Datastore"}, []string{"summary", "info", "host"}, &stores); err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}

	devices, err := hostStorageDevices(ctx, c)
	if err != nil {
		return nil, err
	}

	infos := make([]DatastoreInfo, 0, len(stores))
	for _, ds := range stores {
		info := DatastoreInfo{
			Name:           ds.Summary.Name,
			CapacityBytes:  ds.Summary.Capacity,
			AvailableBytes: ds.Summary.FreeSpace,
		}
		info.UsedBytes = format.Used(info.CapacityBytes, info.AvailableBytes)
		info.Transport = classifyDatastoreTransport(ds, devices)
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

func hostStorageDevices(ctx context.Context, c *vim25.Client) (map[types.ManagedObjectReference]*types.HostStorageDeviceInfo, error) {
	var hosts []mo.HostSystem
	if err := containerRetrieve(ctx, c, []string{"HostSystem"}, []string{"config.storageDevice"}, &hosts); err != nil {
		return nil, fmt.Errorf("list host storage devices: %w", err)
	}
	devices := make(map[types.ManagedObjectReference]*types.HostStorageDeviceInfo, len(hosts))
	for _, host := range hosts {
		if host.Config == nil {
			continue
		}
		devices[host.Self] = host.Config.StorageDevice
	}
	return devices, nil
}

func classifyDatastoreTransport(ds mo.Datastore, devices map[types.ManagedObjectReference]*types.HostStorageDeviceInfo) string {
	extents := extentDevices(ds.Info)
	for _, mount := range ds.Host {
		device := devices[mount.Key]
		if device == nil {
			continue
		}
		if t := transport.ClassifyDatastore(ds.Summary.Type, extents, device); t != transport.Unknown {
			return t
		}
	}
	return transport.ClassifyDatastore(ds.Summary.Type, extents, nil)
}

func extentDevices(info types.BaseDatastoreInfo) []string {
	vmfs, ok := info.(*types.VmfsDatastoreInfo)
	if !ok || vmfs.Vmfs == nil {
		return nil
	}
	devices := make([]string, 0, len(vmfs.Vmfs.Extent))
	for _, extent := range vmfs.Vmfs.Extent {
		if extent.DiskName != "" {
			devices = append(devices, extent.DiskName)
		}
	}
	return devices
}
