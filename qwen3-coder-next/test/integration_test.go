package test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/client"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/config"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/ui"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/datastores"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/portgroup"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/vms"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/vswitches"
)

func TestMain(m *testing.M) {
	config.Init()
	os.Exit(m.Run())
}

func TestVMsIntegration(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := vms.NewFinder(c)
		
		vmsList, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			t.Fatalf("ListVMs failed: %v", err)
		}
		
		if len(vmsList) == 0 {
			t.Error("expected at least one VM")
		}
		
		for _, vm := range vmsList {
			var props struct {
				Name      string
				Config    types.VirtualMachineConfigInfo
				Storage   types.VirtualMachineStorageInfo
			}
			
			pc := property.DefaultCollector(c)
			if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "storage"}, &props); err != nil {
				t.Errorf("failed to get VM properties: %v", err)
				continue
			}
			
			if props.Name == "" {
				t.Error("VM name should not be empty")
			}
			if props.Config.Hardware.NumCPU <= 0 {
				t.Errorf("VM %s should have vCPU > 0, got %d", props.Name, props.Config.Hardware.NumCPU)
			}
			if props.Config.Hardware.MemoryMB <= 0 {
				t.Errorf("VM %s should have RAM > 0, got %d MB", props.Name, props.Config.Hardware.MemoryMB)
			}
			
			var totalStorage int64
			for _, usage := range props.Storage.PerDatastoreUsage {
				totalStorage += usage.Committed
			}
			
			if totalStorage < 0 {
				t.Errorf("VM %s should have storage >= 0, got %d", props.Name, totalStorage)
			}
			
			_ = formatBytes(totalStorage)
		}
		
		var buf strings.Builder
		if err := ui.PrintVMs(&buf, vmsList); err != nil {
			t.Errorf("PrintVMs failed: %v", err)
		}
		
		output := buf.String()
		if len(output) == 0 {
			t.Error("expected non-empty output from PrintVMs")
		}
	})
}

func TestDatastoresIntegration(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := datastores.NewFinder(c)
		
		dss, err := finder.DatastoreList(ctx, "*")
		if err != nil {
			t.Fatalf("ListDatastores failed: %v", err)
		}
		
		if len(dss) == 0 {
			t.Error("expected at least one datastore")
		}
		
		for _, ds := range dss {
			var props struct {
				Name       string
				Summary    types.DatastoreSummary
				Info       types.DatastoreInfo
			}
			
			pc := property.DefaultCollector(c)
			if err := pc.RetrieveOne(ctx, ds.Reference(), []string{"name", "summary.capacity", "summary.freeSpace", "info"}, &props); err != nil {
				t.Errorf("failed to get datastore properties: %v", err)
				continue
			}
			
			if props.Name == "" {
				t.Error("datastore name should not be empty")
			}
			
			if props.Summary.Capacity < 0 {
				t.Errorf("datastore %s should have capacity >= 0, got %d", props.Name, props.Summary.Capacity)
			}
			
			if props.Summary.FreeSpace < 0 {
				t.Errorf("datastore %s should have freeSpace >= 0, got %d", props.Name, props.Summary.FreeSpace)
			}
			
			if props.Summary.FreeSpace > props.Summary.Capacity {
				t.Errorf("datastore %s: freeSpace(%d) > capacity(%d)", props.Name, props.Summary.FreeSpace, props.Summary.Capacity)
			}
			
			if ds.Type != "FC" && ds.Type != "iSCSI" && ds.Type != "NVMe" && ds.Type != "NFS" && ds.Type != "unknown" {
				t.Errorf("datastore %s: invalid type %q", props.Name, ds.Type)
			}
			
			_ = formatBytes(props.Summary.Capacity - props.Summary.FreeSpace)
			_ = formatBytes(props.Summary.FreeSpace)
		}
		
		var buf strings.Builder
		if err := ui.PrintDatastores(&buf, dss); err != nil {
			t.Errorf("PrintDatastores failed: %v", err)
		}
		
		output := buf.String()
		if len(output) == 0 {
			t.Error("expected non-empty output from PrintDatastores")
		}
	})
}

func TestVSwitchesIntegration(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := vswitches.NewFinder(c)
		
		switches, err := finder.HostSystemList(ctx, "*")
		if err != nil {
			t.Fatalf("ListVSwitches failed: %v", err)
		}
		
		if len(switches) == 0 {
			t.Error("expected at least one host")
		}
		
		for _, host := range switches {
			var props struct {
				Name               string
				ConfigManager      types.HostConfigManager
			}
			
			pc := property.DefaultCollector(c)
			if err := pc.RetrieveOne(ctx, host.Reference(), []string{"name", "configManager.networkSystem"}, &props); err != nil {
				t.Errorf("failed to get host properties: %v", err)
				continue
			}
			
			if props.ConfigManager.NetworkSystem != nil {
				var netConfig types.HostNetworkConfig
				if err := pc.RetrieveOne(ctx, *props.ConfigManager.NetworkSystem, []string{"networkConfig"}, &netConfig); err != nil {
					continue
				}
				
				for _, vswitch := range netConfig.Vswitch {
					if vswitch.Name == "" {
						t.Error("vSwitch name should not be empty")
					}
					
					if vswitch.Spec != nil {
						if vswitch.Spec.NumPorts <= 0 {
							t.Errorf("vSwitch %s should have numPorts > 0, got %d", vswitch.Name, vswitch.Spec.NumPorts)
						}
					}
					
					if netConfig.Portgroup != nil {
						for _, pg := range netConfig.Portgroup {
							if pg.Spec.VlanId < 0 || (pg.Spec.VlanId > 4095 && pg.Spec.VlanId != 4095) {
								t.Errorf("vSwitch %s: invalid VLAN ID %d", vswitch.Name, pg.Spec.VlanId)
							}
						}
					}
				}
			}
		}
		
		var buf strings.Builder
		if err := ui.PrintVSwitches(&buf, switches); err != nil {
			t.Errorf("PrintVSwitches failed: %v", err)
		}
		
		output := buf.String()
		if len(output) == 0 {
			t.Error("expected non-empty output from PrintVSwitches")
		}
	})
}

func TestPortgroupVMsIntegration(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := vms.NewFinder(c)
		
		vmList, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			t.Fatalf("ListVMs failed: %v", err)
		}
		
		if len(vmList) == 0 {
			t.Skip("skipping: no VMs available")
		}
		
		switches, err := vswitches.NewFinder(c).HostSystemList(ctx, "*")
		if err != nil {
			t.Fatalf("ListVSwitches failed: %v", err)
		}
		
		if len(switches) == 0 {
			t.Skip("skipping: no switches available")
		}
		
		var testPortgroupName string
		for _, host := range switches {
			var props struct {
				ConfigManager types.HostConfigManager
			}
			
			pc := property.DefaultCollector(c)
			if err := pc.RetrieveOne(ctx, host.Reference(), []string{"configManager.networkSystem"}, &props); err != nil {
				continue
			}
			
			if props.ConfigManager.NetworkSystem == nil {
				continue
			}
			
			var netConfig types.HostNetworkConfig
			if err := pc.RetrieveOne(ctx, *props.ConfigManager.NetworkSystem, []string{"networkConfig"}, &netConfig); err != nil {
				continue
			}
			
			if netConfig.Portgroup != nil && len(netConfig.Portgroup) > 0 {
				testPortgroupName = netConfig.Portgroup[0].Spec.Name
				break
			}
		}
		
		if testPortgroupName == "" {
			t.Skip("skipping: no portgroups available")
		}
		
		var vmByPG []model.VMInfo
		for _, vm := range vmList {
			var props struct {
				Name      string
				Config    types.VirtualMachineConfigInfo
				Storage   types.VirtualMachineStorageInfo
				Network   []types.ManagedObjectReference
			}
			
			pc := property.DefaultCollector(c)
			if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "storage", "network"}, &props); err != nil {
				continue
			}
			
			for _, netRef := range props.Network {
				var netProps struct {
					Name string
				}
				
				if err := pc.RetrieveOne(ctx, netRef, []string{"name"}, &netProps); err != nil {
					continue
				}
				
				if netProps.Name == testPortgroupName {
					var totalStorage int64
					for _, usage := range props.Storage.PerDatastoreUsage {
						totalStorage += usage.Committed
					}
					
					vmByPG = append(vmByPG, model.VMInfo{
						Name:    props.Name,
						VCPU:    props.Config.Hardware.NumCPU,
						RAM:     int64(props.Config.Hardware.MemoryMB) * 1024 * 1024,
						Storage: totalStorage,
					})
				}
			}
		}
		
		for _, vm := range vmByPG {
			if vm.Name == "" {
				t.Error("VM name should not be empty")
			}
			
			if vm.VCPU <= 0 {
				t.Errorf("VM %s should have vCPU > 0, got %d", vm.Name, vm.VCPU)
			}
			
			if vm.RAM <= 0 {
				t.Errorf("VM %s should have RAM > 0, got %d", vm.Name, vm.RAM)
			}
			
			if vm.Storage < 0 {
				t.Errorf("VM %s should have storage >= 0, got %d", vm.Name, vm.Storage)
			}
			
			_ = vm.RAMGB()
			_ = vm.StorageHuman()
		}
		
		var buf strings.Builder
		if err := ui.PrintVMsByPortgroup(&buf, vmByPG); err != nil {
			t.Errorf("PrintVMsByPortgroup failed: %v", err)
		}
		
		output := buf.String()
		if len(output) == 0 {
			t.Error("expected non-empty output from PrintVMsByPortgroup")
		}
	})
}

func TestClientIntegration(t *testing.T) {
	cfg := &config.Config{
		URL:      "https://127.0.0.1:8989/sdk",
		Username: "test",
		Password: "test",
		Insecure: true,
	}
	
	ctx := context.Background()
	
	c, err := client.New(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Logout(ctx)
	
	if c.Client == nil {
		t.Error("client should not be nil")
	}
}

func init() {
	_ = os.RemoveAll
}
