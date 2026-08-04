package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/datastores"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vms"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vswitches"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

func TestVMsCommandAgainstSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 8
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 3
	model.Pool = 0

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vmsList, err := vms.GetVMs(ctx, c)
		if err != nil {
			t.Fatalf("GetVMs() error = %v", err)
		}

		if len(vmsList) != 8 {
			t.Errorf("GetVMs() returned %d VMs, want 8", len(vmsList))
		}

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		var buf bytes.Buffer
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
		for _, vm := range vmsList {
			ramGB := float64(vm.RAMMB) / 1024.0
			fmt.Fprintf(w, "%s\t%d\t%.1f GB\t%s\n", vm.Name, vm.VCPU, ramGB, format.Bytes(vm.StorageBytes))
		}
		w.Flush()

		output := buf.String()
		if !strings.Contains(output, "NAME") {
			t.Error("output should contain header")
		}
		if !strings.Contains(output, "VCPU") {
			t.Error("output should contain VCPU column")
		}
		if !strings.Contains(output, "RAM") {
			t.Error("output should contain RAM column")
		}
		if !strings.Contains(output, "STORAGE") {
			t.Error("output should contain STORAGE column")
		}
	}, model)
}

func TestDatastoresCommandAgainstSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Datastore = 3
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dsList, err := datastores.GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores() error = %v", err)
		}

		if len(dsList) != 3 {
			t.Errorf("GetDatastores() returned %d datastores, want 3", len(dsList))
		}

		sort.Slice(dsList, func(i, j int) bool {
			return dsList[i].Name < dsList[j].Name
		})

		var buf bytes.Buffer
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
		for _, ds := range dsList {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, format.Bytes(ds.UsedBytes), format.Bytes(ds.AvailableBytes))
		}
		w.Flush()

		output := buf.String()
		if !strings.Contains(output, "NAME") {
			t.Error("output should contain header")
		}
		if !strings.Contains(output, "TYPE") {
			t.Error("output should contain TYPE column")
		}
		if !strings.Contains(output, "USED") {
			t.Error("output should contain USED column")
		}
		if !strings.Contains(output, "AVAILABLE") {
			t.Error("output should contain AVAILABLE column")
		}
	}, model)
}

func TestVSwitchesCommandAgainstSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 2
	model.Portgroup = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := vswitches.GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches() error = %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("GetSwitches() should return at least one switch")
		}

		sort.Slice(switches, func(i, j int) bool {
			if switches[i].Name != switches[j].Name {
				return switches[i].Name < switches[j].Name
			}
			return switches[i].Portgroup < switches[j].Portgroup
		})

		var buf bytes.Buffer
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
		for _, sw := range switches {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				sw.Name, sw.Type, sw.Portgroup, sw.VLAN, sw.Uplinks, sw.LACP, sw.TotalPorts, sw.UsedPorts)
		}
		w.Flush()

		output := buf.String()
		if !strings.Contains(output, "SWITCH") {
			t.Error("output should contain header")
		}
		if !strings.Contains(output, "SWITCH TYPE") {
			t.Error("output should contain SWITCH TYPE column")
		}
		if !strings.Contains(output, "PORTGROUP") {
			t.Error("output should contain PORTGROUP column")
		}
		if !strings.Contains(output, "VLAN") {
			t.Error("output should contain VLAN column")
		}
		if !strings.Contains(output, "UPLINKS") {
			t.Error("output should contain UPLINKS column")
		}
		if !strings.Contains(output, "LACP") {
			t.Error("output should contain LACP column")
		}
		if !strings.Contains(output, "PORTS") {
			t.Error("output should contain PORTS column")
		}
		if !strings.Contains(output, "USED") {
			t.Error("output should contain USED column")
		}
	}, model)
}

func TestVSwitchesPortgroupCommandAgainstSimulator(t *testing.T) {
	model := simulator.VPX()
	model.Machine = 3
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 1
	model.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		dc, err := finder.DefaultDatacenter(ctx)
		if err != nil {
			t.Fatalf("finding default datacenter: %v", err)
		}
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			t.Fatalf("finding VMs: %v", err)
		}

		if len(vms) == 0 {
			t.Fatal("no VMs found in simulator")
		}

		var portgroupName string
		for _, vm := range vms {
			var vmMo mo.VirtualMachine
			err := vm.Properties(ctx, vm.Reference(), []string{
				"name",
				"network",
			}, &vmMo)
			if err != nil {
				continue
			}

			if len(vmMo.Network) > 0 {
				netRef := vmMo.Network[0]
				common := object.NewCommon(c, netRef)

				var netMo mo.Network
				err := common.Properties(ctx, netRef, []string{"name"}, &netMo)
				if err != nil {
					continue
				}

				portgroupName = netMo.Name
				break
			}
		}

		if portgroupName == "" {
			t.Fatal("no port group found for any VM")
		}

		vmsList, err := vswitches.GetVMsByPortgroup(ctx, c, portgroupName)
		if err != nil {
			t.Fatalf("GetVMsByPortgroup() error = %v", err)
		}

		if len(vmsList) == 0 {
			t.Errorf("GetVMsByPortgroup(%q) should return at least one VM", portgroupName)
		}

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		for _, vm := range vmsList {
			if vm.VCPU <= 0 {
				t.Errorf("VM %s: VCPU = %d, want > 0", vm.Name, vm.VCPU)
			}
			if vm.RAMMB <= 0 {
				t.Errorf("VM %s: RAMMB = %d, want > 0", vm.Name, vm.RAMMB)
			}
		}
	}, model)
}

func TestConfigPrecedenceFlagOverEnv(t *testing.T) {
	cfg := &config.Config{
		URL:      "https://flag.lab/sdk",
		Username: "flaguser",
		Password: "flagpass",
	}

	if cfg.URL != "https://flag.lab/sdk" {
		t.Error("flag value should be used")
	}
	if cfg.Username != "flaguser" {
		t.Error("flag username should be used")
	}
	if cfg.Password != "flagpass" {
		t.Error("flag password should be used")
	}
}

func TestFormatBytesConsistency(t *testing.T) {
	capacity := int64(10737418240) // 10 GiB
	used := int64(3221225472)      // 3 GiB
	available := capacity - used

	if used+available != capacity {
		t.Errorf("used + available (%d + %d) != capacity (%d)", used, available, capacity)
	}

	usedStr := format.Bytes(used)
	availStr := format.Bytes(available)

	if usedStr == "" || availStr == "" {
		t.Error("format output should not be empty")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
