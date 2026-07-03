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
)

type VMInfo struct {
	Name    string
	VCPU    int32
	RAMGB   float64
	Storage string
}

func getVMs(ctx context.Context, client *govmomi.Client) ([]VMInfo, error) {
	folder := object.NewRootFolder(client.Client)

	vmViewManager := view.NewManager(client.Client)
	v, err := vmViewManager.CreateContainerView(ctx, folder.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	defer v.Destroy(ctx)

	var vmsMo []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage"}, &vmsMo)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve VMs: %w", err)
	}

	var vms []VMInfo
	for _, vmMo := range vmsMo {
		vcpu := int32(0)
		if vmMo.Config != nil {
			vcpu = vmMo.Config.Hardware.NumCPU
		}

		ramMB := int64(0)
		if vmMo.Config != nil {
			ramMB = int64(vmMo.Config.Hardware.MemoryMB)
		}
		ramGB := float64(ramMB) / 1024.0

		storage := "0.0 GiB"
		committed := int64(0)
		if vmMo.Summary.Storage != nil {
			committed = vmMo.Summary.Storage.Committed
		}
		if committed > 0 {
			storage = formatBytes(committed)
		}

		vms = append(vms, VMInfo{
			Name:    vmMo.Name,
			VCPU:    vcpu,
			RAMGB:   ramGB,
			Storage: storage,
		})
	}

	sort.Slice(vms, func(i, j int) bool {
		return vms[i].Name < vms[j].Name
	})

	return vms, nil
}

func vmsPrint(vms []VMInfo) {
	fmt.Printf("%-30s %-8s %-8s %-10s\n", "NAME", "VCPU", "RAM(GB)", "STORAGE")
	fmt.Printf("%-30s %-8s %-8s %-10s\n", "----", "----", "-------", "-------")
	for _, vm := range vms {
		fmt.Printf("%-30s %-8d %-8.1f %-10s\n", vm.Name, vm.VCPU, vm.RAMGB, vm.Storage)
	}
}

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "Print a table of all virtual machines",
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

		vms, err := getVMs(ctx, client)
		if err != nil {
			return err
		}

		vmsPrint(vms)
		return nil
	},
}
