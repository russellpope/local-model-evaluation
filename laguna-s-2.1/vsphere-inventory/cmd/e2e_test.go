package cmd

import (
	"bytes"
	"context"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/datastores"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vms"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vswitches"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestEndToEndVMLoop(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Machine = 8
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 3
	simModel.Pool = 0

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vmsList, err := vms.GetVMs(ctx, c)
		if err != nil {
			t.Fatalf("GetVMs() error = %v", err)
		}

		if len(vmsList) != 8 {
			t.Fatalf("GetVMs() returned %d VMs, want 8", len(vmsList))
		}

		for _, vm := range vmsList {
			if vm.VCPU != 1 {
				t.Errorf("VM %s: VCPU = %d, want 1", vm.Name, vm.VCPU)
			}
			if vm.RAMMB != 32 {
				t.Errorf("VM %s: RAMMB = %d, want 32", vm.Name, vm.RAMMB)
			}
			if vm.StorageBytes != 234 {
				t.Errorf("VM %s: StorageBytes = %d, want 234", vm.Name, vm.StorageBytes)
			}
		}

		var buf bytes.Buffer
		format.RenderVMs(&buf, vmsList)

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
		if !strings.Contains(output, "32.0 MiB") {
			t.Error("output should contain 32.0 MiB for RAM")
		}
	}, simModel)
}

func TestEndToEndDatastoresLoop(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Datastore = 3
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dsList, err := datastores.GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores() error = %v", err)
		}

		if len(dsList) != 3 {
			t.Fatalf("GetDatastores() returned %d datastores, want 3", len(dsList))
		}

		for _, ds := range dsList {
			if ds.Type != "unknown" {
				t.Errorf("Datastore %s: Type = %q, want %q", ds.Name, ds.Type, "unknown")
			}
			if ds.UsedBytes+ds.AvailableBytes != ds.CapacityBytes {
				t.Errorf("Datastore %s: used + available (%d + %d) != capacity (%d)",
					ds.Name, ds.UsedBytes, ds.AvailableBytes, ds.CapacityBytes)
			}
		}

		var buf bytes.Buffer
		format.RenderDatastores(&buf, dsList)

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
	}, simModel)
}

func TestEndToEndVSwitchesLoop(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 2
	simModel.Portgroup = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		switches, err := vswitches.GetSwitches(ctx, c)
		if err != nil {
			t.Fatalf("GetSwitches() error = %v", err)
		}

		if len(switches) == 0 {
			t.Fatal("GetSwitches() should return at least one switch")
		}

		hasStandard := false
		hasDistributed := false
		for _, sw := range switches {
			if sw.Portgroup == "" {
				t.Error("Portgroup should not be empty")
			}

			if sw.UsedPorts > sw.TotalPorts {
				t.Errorf("Switch %s/%s: used ports (%d) > total ports (%d)",
					sw.Name, sw.Portgroup, sw.UsedPorts, sw.TotalPorts)
			}

			if sw.Type == "standard" {
				hasStandard = true
				if sw.TotalPorts != 1536 {
					t.Errorf("Standard switch %s: TotalPorts = %d, want 1536", sw.Name, sw.TotalPorts)
				}
				if sw.UsedPorts != 6 {
					t.Errorf("Standard switch %s: UsedPorts = %d, want 6", sw.Name, sw.UsedPorts)
				}
				if sw.Uplinks != "vmnic0" {
					t.Errorf("Standard switch %s: Uplinks = %q, want %q", sw.Name, sw.Uplinks, "vmnic0")
				}
			}

			if sw.Type == "distributed" {
				hasDistributed = true
			}
		}

		if !hasStandard {
			t.Error("GetSwitches() should return at least one standard switch")
		}
		if !hasDistributed {
			t.Error("GetSwitches() should return at least one distributed switch")
		}

		var buf bytes.Buffer
		format.RenderVSwitches(&buf, switches)

		output := buf.String()
		if !strings.Contains(output, "SWITCH") {
			t.Error("output should contain header")
		}
		if !strings.Contains(output, "PORTGROUP") {
			t.Error("output should contain PORTGROUP column")
		}
		if !strings.Contains(output, "DC0_DVPG0") {
			t.Error("output should contain DC0_DVPG0 portgroup")
		}
	}, simModel)
}

func TestEndToEndPortgroupFilterLoop(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Machine = 3
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 1
	simModel.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vmsList, err := vswitches.GetVMsByPortgroup(ctx, c, "DC0_DVPG0")
		if err != nil {
			t.Fatalf("GetVMsByPortgroup() error = %v", err)
		}

		if len(vmsList) != 3 {
			t.Fatalf("GetVMsByPortgroup(%q) returned %d VMs, want 3", "DC0_DVPG0", len(vmsList))
		}

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		expectedNames := []string{
			"DC0_C0_RP0_VM0",
			"DC0_C0_RP0_VM1",
			"DC0_C0_RP0_VM2",
		}
		for i, vm := range vmsList {
			if vm.Name != expectedNames[i] {
				t.Errorf("VM at index %d: Name = %q, want %q", i, vm.Name, expectedNames[i])
			}
			if vm.VCPU != 1 {
				t.Errorf("VM %s: VCPU = %d, want 1", vm.Name, vm.VCPU)
			}
			if vm.RAMMB != 32 {
				t.Errorf("VM %s: RAMMB = %d, want 32", vm.Name, vm.RAMMB)
			}
			if vm.StorageBytes != 234 {
				t.Errorf("VM %s: StorageBytes = %d, want 234", vm.Name, vm.StorageBytes)
			}
		}
	}, simModel)
}

func TestEndToEndConfigPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := tmpDir + "/config.yaml"
	configContent := `url: https://file.lab/sdk
username: fileuser
password: filepass
insecure: false
timeout: 10s
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	v := viper.New()
	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()
	setEnv(t, "VSPHERE_URL", "https://env.lab/sdk")
	setEnv(t, "VSPHERE_USERNAME", "envuser")
	setEnv(t, "VSPHERE_PASSWORD", "envpass")

	v.Set("config", configFile)
	c := config.NewWithViper(v)
	if err := c.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.URL != "https://env.lab/sdk" {
		t.Errorf("URL = %q, want %q (env should override file)", c.URL, "https://env.lab/sdk")
	}
	if c.Username != "envuser" {
		t.Errorf("Username = %q, want %q (env should override file)", c.Username, "envuser")
	}
	if c.Password != "envpass" {
		t.Errorf("Password = %q, want %q (env should override file)", c.Password, "envpass")
	}
}

func TestProductionBindPFlagWired(t *testing.T) {
	urlFlag := rootCmd.PersistentFlags().Lookup("url")
	if urlFlag == nil {
		t.Fatal("rootCmd should have a --url persistent flag")
	}

	urlFlag.Value.Set("https://flag.lab/sdk")

	c := config.New()
	if err := c.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.URL != "https://flag.lab/sdk" {
		t.Errorf("cfg.URL = %q, want %q (BindPFlag for url is not wired)", c.URL, "https://flag.lab/sdk")
	}
}

func setEnv(t *testing.T, key, val string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	os.Setenv(key, val)
	t.Cleanup(func() {
		if ok {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestResolvePortgroupFromOutput(t *testing.T) {
	output := "SWITCH  SWITCH TYPE  PORTGROUP    VLAN  UPLINKS  LACP  PORTS  USED\n" +
		"DVS0    distributed    DC0_DVPG0    0     N/A      N/A   5      0\n" +
		"DVS0    distributed    DC0_DVPG1    0     N/A      N/A   5      0\n"

	pg, err := resolvePortgroupFromOutput(output)
	if err != nil {
		t.Fatalf("resolvePortgroupFromOutput() error = %v", err)
	}
	if pg != "DC0_DVPG0" {
		t.Errorf("resolvePortgroupFromOutput() = %q, want %q", pg, "DC0_DVPG0")
	}
}

func TestIsNotFoundError(t *testing.T) {
	if isNotFoundError(nil) {
		t.Error("isNotFoundError(nil) should be false")
	}
}
