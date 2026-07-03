package main

import (
	"context"
	"fmt"
	"sort"

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
	device = lower(device)
	return containsAny(device, []string{"nvme", "nvmex", "ns0", "ns1", "ns2", "ns3"})
}

func isISCSIDevice(device string) bool {
	device = lower(device)
	return containsAny(device, []string{"naa.", "iqn.", "eui.", "tpgt", "iscsi", "vmhba33", "vmhba34", "vmhba35", "vmhba36", "vmhba37", "vmhba38", "vmhba39", "vmhba40", "vmhba41", "vmhba42", "vmhba43", "vmhba44", "vmhba45", "vmhba46", "vmhba47", "vmhba48"})
}

func isFCDevice(device string) bool {
	device = lower(device)
	return containsAny(device, []string{"mpx.vmhba", "t10.", "fc", "vmhba0", "vmhba1", "vmhba2", "vmhba3", "vmhba4", "vmhba5", "vmhba6", "vmhba7", "vmhba8", "vmhba9", "vmhba10", "vmhba11", "vmhba12", "vmhba13", "vmhba14", "vmhba15", "vmhba16", "vmhba17", "vmhba18", "vmhba19", "vmhba20", "vmhba21", "vmhba22", "vmhba23", "vmhba24", "vmhba25", "vmhba26", "vmhba27", "vmhba28", "vmhba29", "vmhba30", "vmhba31", "vmhba32"})
}

func classifyStorageFromDevice(device string) string {
	if device == "" {
		return "unknown"
	}
	device = lower(device)
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

func lower(s string) string {
	sLower := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			sLower += string(c + 32)
		} else {
			sLower += string(c)
		}
	}
	return sLower
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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

		ctx := cmd.Context()
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
