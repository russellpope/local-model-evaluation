package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestFindVMsInPortGroup(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 1
	model.ClusterHost = 1
	model.Machine = 2
	model.Portgroup = 1

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		client := &govmomi.Client{
			Client:         c,
			SessionManager: session.NewManager(c),
		}

		switches, err := ListSwitches(ctx, client)
		if err != nil {
			t.Fatalf("ListSwitches failed: %v", err)
		}

		var pgName string
		for _, sw := range switches {
			for _, pg := range sw.PortGroups {
				if pg.Name != "" {
					pgName = pg.Name
					break
				}
			}
			if pgName != "" {
				break
			}
		}

		if pgName == "" {
			t.Skip("no port groups found in simulator")
		}

		vms, err := FindVMsInPortGroup(ctx, client, pgName)
		if err != nil {
			t.Fatalf("FindVMsInPortGroup failed: %v", err)
		}

		for _, vm := range vms {
			if vm.Name == "" {
				t.Error("VM has empty name")
			}
		}
	}, model)
}
