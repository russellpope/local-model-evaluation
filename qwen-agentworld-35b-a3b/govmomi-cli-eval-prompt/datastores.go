package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type DatastoreInfo struct {
	Name      string
	Type      string
	Used      string
	Available string
}

func getDatastoreTransportType(ds mo.Datastore) string {
	// Check for NFS datastores
	if ds.Summary.Type == "NFS" || ds.Summary.Type == "NFS41" {
		return "NFS"
	}

	// For VMFS datastores, check the backing storage
	if ds.Summary.Type == "VMFS" {
		// Check for NVMe over FC (i.e. NVMe/FC)
		if ds.Info != nil {
			if vmfs, ok := ds.Info.(*types.VmfsDatastoreInfo); ok {
				if vmfs.Vmfs != nil && vmfs.Vmfs.Extent != nil {
					for _, extent := range vmfs.Vmfs.Extent {
						if extent.DiskName != "" {
							// Check for NVMe devices (nvme0n1, nvmexxx, etc.)
							if isNVMeDevice(extent.DiskName) {
								return "NVMe"
							}
							// Check for iSCSI devices (naa.xxxx, iqn.xxxx, tpgt, etc.)
							if isISCSIDevice(extent.DiskName) {
								return "iSCSI"
							}
							// Check for FC devices (mpx.vmhba..., t10., etc.)
							if isFCDevice(extent.DiskName) {
								return "FC"
							}
						}
					}
				}
			}
		}
		// Fallback: check host storage adapters
		return classifyStorageFromDevice("")
	}

	return "unknown"
}

func isNVMeDevice(device string) bool {
	deviceLower := strings.ToLower(device)
	return strings.Contains(deviceLower, "nvme") ||
		strings.Contains(deviceLower, "nvmex") ||
		strings.Contains(deviceLower, "ns0") ||
		strings.Contains(deviceLower, "ns1") ||
		strings.Contains(deviceLower, "ns2") ||
		strings.Contains(deviceLower, "ns3")
}

func isISCSIDevice(device string) bool {
	deviceLower := strings.ToLower(device)
	return strings.Contains(deviceLower, "naa.") ||
		strings.Contains(deviceLower, "iqn.") ||
		strings.Contains(deviceLower, "eui.") ||
		strings.Contains(deviceLower, "tpgt") ||
		strings.Contains(deviceLower, "iscsi") ||
		strings.Contains(deviceLower, "vmhba33") ||
		strings.Contains(deviceLower, "vmhba34") ||
		strings.Contains(deviceLower, "vmhba35") ||
		strings.Contains(deviceLower, "vmhba36") ||
		strings.Contains(deviceLower, "vmhba37") ||
		strings.Contains(deviceLower, "vmhba38") ||
		strings.Contains(deviceLower, "vmhba39") ||
		strings.Contains(deviceLower, "vmhba40") ||
		strings.Contains(deviceLower, "vmhba41") ||
		strings.Contains(deviceLower, "vmhba42") ||
		strings.Contains(deviceLower, "vmhba43") ||
		strings.Contains(deviceLower, "vmhba44") ||
		strings.Contains(deviceLower, "vmhba45") ||
		strings.Contains(deviceLower, "vmhba46") ||
		strings.Contains(deviceLower, "vmhba47") ||
		strings.Contains(deviceLower, "vmhba48")
}

func isFCDevice(device string) bool {
	deviceLower := strings.ToLower(device)
	return strings.Contains(deviceLower, "mpx.vmhba") ||
		strings.Contains(deviceLower, "t10.") ||
		strings.Contains(deviceLower, "fc") ||
		strings.HasPrefix(deviceLower, "vmhba0") ||
		strings.HasPrefix(deviceLower, "vmhba1") ||
		strings.HasPrefix(deviceLower, "vmhba2") ||
		strings.HasPrefix(deviceLower, "vmhba3") ||
		strings.HasPrefix(deviceLower, "vmhba4") ||
		strings.HasPrefix(deviceLower, "vmhba5") ||
		strings.HasPrefix(deviceLower, "vmhba6") ||
		strings.HasPrefix(deviceLower, "vmhba7") ||
		strings.HasPrefix(deviceLower, "vmhba8") ||
		strings.HasPrefix(deviceLower, "vmhba9") ||
		strings.HasPrefix(deviceLower, "vmhba10") ||
		strings.HasPrefix(deviceLower, "vmhba11") ||
		strings.HasPrefix(deviceLower, "vmhba12") ||
		strings.HasPrefix(deviceLower, "vmhba13") ||
		strings.HasPrefix(deviceLower, "vmhba14") ||
		strings.HasPrefix(deviceLower, "vmhba15") ||
		strings.HasPrefix(deviceLower, "vmhba16") ||
		strings.HasPrefix(deviceLower, "vmhba17") ||
		strings.HasPrefix(deviceLower, "vmhba18") ||
		strings.HasPrefix(deviceLower, "vmhba19") ||
		strings.HasPrefix(deviceLower, "vmhba20") ||
		strings.HasPrefix(deviceLower, "vmhba21") ||
		strings.HasPrefix(deviceLower, "vmhba22") ||
		strings.HasPrefix(deviceLower, "vmhba23") ||
		strings.HasPrefix(deviceLower, "vmhba24") ||
		strings.HasPrefix(deviceLower, "vmhba25") ||
		strings.HasPrefix(deviceLower, "vmhba26") ||
		strings.HasPrefix(deviceLower, "vmhba27") ||
		strings.HasPrefix(deviceLower, "vmhba28") ||
		strings.HasPrefix(deviceLower, "vmhba29") ||
		strings.HasPrefix(deviceLower, "vmhba30") ||
		strings.HasPrefix(deviceLower, "vmhba31") ||
		strings.HasPrefix(deviceLower, "vmhba32")
}

func classifyStorageFromDevice(device string) string {
	if device == "" {
		return "unknown"
	}
	if isNVMeDevice(device) {
		return "NVMe"
	}
	if isISCSIDevice(device) {
		return "iSCSI"
	}
	if isFCDevice(device) {
		return "FC"
	}
	return "unknown"
}

func getDatastores(ctx context.Context, client *govmomi.Client) ([]DatastoreInfo, error) {
	folder := object.NewRootFolder(client.Client)

	dsViewManager := view.NewManager(client.Client)
	v, err := dsViewManager.CreateContainerView(ctx, folder.Reference(), []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	defer v.Destroy(ctx)

	var dsMoList []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{"name", "summary", "info"}, &dsMoList)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve datastores: %w", err)
	}

	var datastoreInfos []DatastoreInfo
	for _, dsMo := range dsMoList {
		dsType := getDatastoreTransportType(dsMo)

		capacity := int64(0)
		free := int64(0)

		if dsMo.Summary.Type != "" {
			if dsMo.Summary.Capacity != 0 {
				capacity = dsMo.Summary.Capacity
			}
			if dsMo.Summary.FreeSpace != 0 {
				free = dsMo.Summary.FreeSpace
			}
		}

		usedBytes := capacity - free
		if usedBytes < 0 {
			usedBytes = 0
		}

		used := formatBytes(usedBytes)
		available := formatBytes(free)

		datastoreInfos = append(datastoreInfos, DatastoreInfo{
			Name:      dsMo.Name,
			Type:      dsType,
			Used:      used,
			Available: available,
		})
	}

	sort.Slice(datastoreInfos, func(i, j int) bool {
		return datastoreInfos[i].Name < datastoreInfos[j].Name
	})

	return datastoreInfos, nil
}

func datastoresPrint(datastores []DatastoreInfo) {
	fmt.Printf("%-30s %-10s %-10s %-10s\n", "NAME", "TYPE", "USED", "AVAILABLE")
	fmt.Printf("%-30s %-10s %-10s %-10s\n", "----", "----", "----", "---------")
	for _, ds := range datastores {
		fmt.Printf("%-30s %-10s %-10s %-10s\n", ds.Name, ds.Type, ds.Used, ds.Available)
	}
}

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "Print a table of all datastores",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := Config{
			URL:      viper.GetString("url"),
			Username: viper.GetString("username"),
			Password: viper.GetString("password"),
			Insecure: viper.GetBool("insecure"),
			Timeout:  viper.GetDuration("timeout"),
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
		defer cancel()

		client, err := connect(ctx, cfg)
		if err != nil {
			return err
		}
		defer client.Logout(ctx)

		datastores, err := getDatastores(ctx, client)
		if err != nil {
			return err
		}

		datastoresPrint(datastores)
		return nil
	},
}
