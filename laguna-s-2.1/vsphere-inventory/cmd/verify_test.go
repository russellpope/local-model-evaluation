package cmd

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
)

func TestVerifyEndToEnd(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Machine = 5
	simModel.Host = 0
	simModel.Cluster = 1
	simModel.ClusterHost = 1
	simModel.Portgroup = 2

	if err := simModel.Create(); err != nil {
		t.Fatalf("creating simulator: %v", err)
	}
	defer simModel.Remove()

	simModel.Service.TLS = nil
	server := simModel.Service.NewServer()
	defer server.Close()

	binaryPath := "/tmp/vsphere-inventory-verify"
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./vsphere-inventory")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("building binary: %v\n%s", err, out)
	}
	defer os.Remove(binaryPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runCmd := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Env = append(os.Environ(),
			"VSPHERE_URL="+server.URL.String(),
			"VSPHERE_USERNAME=user",
			"VSPHERE_PASSWORD=pass",
			"VSPHERE_INSECURE=true",
		)
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	t.Run("vms", func(t *testing.T) {
		output, err := runCmd("vms")
		if err != nil {
			t.Fatalf("vms command failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "NAME") {
			t.Error("vms output should contain NAME header")
		}
		if !strings.Contains(output, "VCPU") {
			t.Error("vms output should contain VCPU column")
		}
		if !strings.Contains(output, "RAM") {
			t.Error("vms output should contain RAM column")
		}
		if !strings.Contains(output, "STORAGE") {
			t.Error("vms output should contain STORAGE column")
		}
		if !strings.Contains(output, "32.0 MiB") {
			t.Error("vms output should contain 32.0 MiB for RAM")
		}
	})

	t.Run("datastores", func(t *testing.T) {
		output, err := runCmd("datastores")
		if err != nil {
			t.Fatalf("datastores command failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "NAME") {
			t.Error("datastores output should contain NAME header")
		}
		if !strings.Contains(output, "TYPE") {
			t.Error("datastores output should contain TYPE column")
		}
		if !strings.Contains(output, "USED") {
			t.Error("datastores output should contain USED column")
		}
		if !strings.Contains(output, "AVAILABLE") {
			t.Error("datastores output should contain AVAILABLE column")
		}
		if !strings.Contains(output, "unknown") {
			t.Error("datastores output should contain unknown type for vcsim datastores")
		}
	})

	t.Run("vswitches", func(t *testing.T) {
		output, err := runCmd("vswitches")
		if err != nil {
			t.Fatalf("vswitches command failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "SWITCH") {
			t.Error("vswitches output should contain SWITCH header")
		}
		if !strings.Contains(output, "PORTGROUP") {
			t.Error("vswitches output should contain PORTGROUP column")
		}
		if !strings.Contains(output, "DC0_DVPG0") {
			t.Error("vswitches output should contain DC0_DVPG0 portgroup")
		}
		if !strings.Contains(output, "LACP") {
			t.Error("vswitches output should contain LACP column")
		}
		if !strings.Contains(output, "UPLINKS") {
			t.Error("vswitches output should contain UPLINKS column")
		}
	})

	t.Run("vswitches --portgroup", func(t *testing.T) {
		output, err := runCmd("vswitches", "--portgroup", "DC0_DVPG0")
		if err != nil {
			t.Fatalf("vswitches --portgroup command failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "NAME") {
			t.Error("vswitches --portgroup output should contain NAME header")
		}
		if !strings.Contains(output, "VCPU") {
			t.Error("vswitches --portgroup output should contain VCPU column")
		}
		if !strings.Contains(output, "DC0_C0_RP0_VM0") {
			t.Error("vswitches --portgroup output should contain DC0_C0_RP0_VM0")
		}
	})
}
