package inventory

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

func TestListSwitches(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 1
	model.ClusterHost = 1
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

		if len(switches) == 0 {
			t.Fatal("expected at least one switch")
		}

		for _, sw := range switches {
			if sw.Name == "" {
				t.Error("switch has empty name")
			}
			if sw.SwitchType != "standard" && sw.SwitchType != "distributed" {
				t.Errorf("switch %s has invalid type: %q", sw.Name, sw.SwitchType)
			}

			for _, pg := range sw.PortGroups {
				if pg.Name == "" {
					t.Errorf("switch %s has port group with empty name", sw.Name)
				}
			}

			if sw.UsedPorts > sw.TotalPorts && sw.TotalPorts > 0 {
				t.Errorf("switch %s: used ports (%d) > total ports (%d)",
					sw.Name, sw.UsedPorts, sw.TotalPorts)
			}

			validLACP := false
			for _, lacp := range []string{"enabled", "disabled", "N/A"} {
				if sw.LACP == lacp {
					validLACP = true
					break
				}
			}
			if !validLACP {
				t.Errorf("switch %s has invalid LACP value: %q", sw.Name, sw.LACP)
			}
		}
	}, model)
}
