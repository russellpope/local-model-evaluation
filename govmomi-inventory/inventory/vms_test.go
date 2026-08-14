package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

func TestListVMs(t *testing.T) {
	model := simulator.ESX()
	model.Machine = 5
	model.Datastore = 2
	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		vmgr := view.NewManager(c)
		v, err := vmgr.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
		if err != nil {
			return err
		}
		defer v.Destroy(ctx)

		var vmList []mo.VirtualMachine
		if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "storage.perDatastoreUsage"}, &vmList); err != nil {
			return err
		}

		if len(vmList) != 5 {
			t.Errorf("expected 5 VMs, got %d", len(vmList))
		}
		for _, vm := range vmList {
			if vm.Name == "" {
				t.Error("VM name is empty")
			}
			if vm.Config.Hardware.NumCPU <= 0 {
				t.Errorf("VM %s has vCPU <= 0: %d", vm.Name, vm.Config.Hardware.NumCPU)
			}
			if vm.Config.Hardware.MemoryMB <= 0 {
				t.Errorf("VM %s has RAM <= 0: %d", vm.Name, vm.Config.Hardware.MemoryMB)
			}
			if vm.Storage != nil {
				var storageGiB float64
				for _, usage := range vm.Storage.PerDatastoreUsage {
					storageGiB += float64(usage.Committed) / (1024 * 1024 * 1024)
				}
				if storageGiB < 0 {
					t.Errorf("VM %s has negative storage: %.1f", vm.Name, storageGiB)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}

func TestListDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Datastore = 3
	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		vmgr := view.NewManager(c)
		v, err := vmgr.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)
		if err != nil {
			return err
		}
		defer v.Destroy(ctx)

		var dsList []mo.Datastore
		if err := v.Retrieve(ctx, []string{"Datastore"}, []string{"name", "summary.capacity", "summary.freeSpace", "summary.type"}, &dsList); err != nil {
			return err
		}

		if len(dsList) != 3 {
			t.Errorf("expected 3 datastores, got %d", len(dsList))
		}
		for _, ds := range dsList {
			if ds.Name == "" {
				t.Error("datastore name is empty")
			}
			if ds.Summary.Type == "NFS" {
				if classifyDatastoreType(ds.Summary) != "NFS" {
					t.Errorf("NFS datastore classified as %s", classifyDatastoreType(ds.Summary))
				}
			} else {
				cls := classifyDatastoreType(ds.Summary)
				if cls != "unknown" {
					t.Errorf("non-NFS datastore %s classified as %s", ds.Name, cls)
				}
			}
			capacityGiB := float64(ds.Summary.Capacity) / (1024 * 1024 * 1024)
			freeGiB := float64(ds.Summary.FreeSpace) / (1024 * 1024 * 1024)
			usedGiB := capacityGiB - freeGiB
			if usedGiB < 0 {
				t.Errorf("datastore %s has negative used: %.1f", ds.Name, usedGiB)
			}
			if freeGiB < 0 {
				t.Errorf("datastore %s has negative free: %.1f", ds.Name, freeGiB)
			}
			if freeGiB > capacityGiB {
				t.Errorf("datastore %s free %.1f > capacity %.1f", ds.Name, freeGiB, capacityGiB)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}

func TestListSwitches(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Portgroup = 2
	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		dc := object.NewDatacenter(c, c.ServiceContent.RootFolder)
		_ = dc
		sw, err := ListSwitches(ctx, dc)
		if err != nil {
			return err
		}
		if len(sw) == 0 {
			t.Fatal("expected at least one switch, got 0")
		}
		for _, s := range sw {
			if s.Name == "" {
				t.Error("switch name is empty")
			}
			if s.SwitchType != "standard" && s.SwitchType != "distributed" {
				t.Errorf("switch %s has invalid type: %s", s.Name, s.SwitchType)
			}
			if s.LACP != "enabled" && s.LACP != "disabled" && s.LACP != "N/A" {
				t.Errorf("switch %s has invalid LACP: %s", s.Name, s.LACP)
			}
			if s.TotalPorts < 0 {
				t.Errorf("switch %s has negative total ports: %d", s.Name, s.TotalPorts)
			}
			if s.UsedPorts < 0 {
				t.Errorf("switch %s has negative used ports: %d", s.Name, s.UsedPorts)
			}
			if s.UsedPorts > s.TotalPorts {
				t.Errorf("switch %s used ports %d > total ports %d", s.Name, s.UsedPorts, s.TotalPorts)
			}
		}
		for i := 1; i < len(sw); i++ {
			if switchLess(sw[i], sw[i-1]) {
				t.Errorf("switches not sorted: %s %s < %s %s",
					sw[i].SwitchType, sw[i].Name, sw[i-1].SwitchType, sw[i-1].Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}

func TestFindVMsByPortGroup(t *testing.T) {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 3
	model.Portgroup = 2
	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		dc := object.NewDatacenter(c, c.ServiceContent.RootFolder)
		sw, err := ListSwitches(ctx, dc)
		if err != nil {
			return err
		}
		var pgName string
		for _, s := range sw {
			if s.SwitchType == "standard" && s.PortGroup != "" {
				pgName = s.PortGroup
				break
			}
		}
		if pgName == "" {
			t.Skip("no standard port groups found")
		}
		vms, err := FindVMsByPortGroup(ctx, dc, pgName)
		if err != nil {
			return err
		}
		_ = vms
		return nil
	})
	if err != nil {
		t.Fatalf("simulator run: %v", err)
	}
}
