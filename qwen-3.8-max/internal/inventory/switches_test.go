package inventory_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/internal/inventory"
)

var (
	vlanNumber = regexp.MustCompile(`^[0-9]+$`)
	vlanTrunk  = regexp.MustCompile(`^([0-9]+(-[0-9]+)?)(,([0-9]+(-[0-9]+)?))*$`)
	vlanPvlan  = regexp.MustCompile(`^pvlan [0-9]+$`)
)

func validVLAN(v string) bool {
	switch {
	case v == "trunk", v == "unknown":
		return true
	case vlanNumber.MatchString(v), vlanTrunk.MatchString(v), vlanPvlan.MatchString(v):
		return true
	}
	return false
}

func TestListSwitches(t *testing.T) {
	model := simulator.VPX()
	model.ClusterHost = 2
	model.Portgroup = 2

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		rows, err := inventory.ListSwitches(ctx, c)
		if err != nil {
			t.Fatalf("ListSwitches: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("ListSwitches returned no rows")
		}

		var standard, distributed int
		for _, row := range rows {
			if row.Switch == "" {
				t.Errorf("row with empty switch name: %+v", row)
			}
			if row.Portgroup == "" {
				t.Errorf("row with empty port group name: %+v", row)
			}
			switch row.SwitchType {
			case "standard":
				standard++
				if row.LACP != "N/A" {
					t.Errorf("standard switch %q: LACP = %q, want N/A", row.Switch, row.LACP)
				}
			case "distributed":
				distributed++
				if row.LACP != "enabled" && row.LACP != "disabled" {
					t.Errorf("distributed switch %q: LACP = %q, want enabled or disabled", row.Switch, row.LACP)
				}
			default:
				t.Errorf("row with invalid switch type %q: %+v", row.SwitchType, row)
			}
			if !validVLAN(row.VLAN) {
				t.Errorf("row %s/%s: VLAN %q does not parse", row.Switch, row.Portgroup, row.VLAN)
			}
			if row.Ports < 0 {
				t.Errorf("row %s/%s: ports %d < 0", row.Switch, row.Portgroup, row.Ports)
			}
			if row.Used < 0 || row.Used > row.Ports {
				t.Errorf("row %s/%s: used ports %d outside [0, %d]", row.Switch, row.Portgroup, row.Used, row.Ports)
			}
		}
		if standard == 0 {
			t.Error("no standard switch rows returned")
		}
		if distributed == 0 {
			t.Error("no distributed switch rows returned")
		}
	}, model)
}
