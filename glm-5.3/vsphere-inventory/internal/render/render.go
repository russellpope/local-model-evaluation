// Package render prints inventory results as plain, greppable tables using
// text/tabwriter.
package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/inventory"
)

func newWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

// WriteVMs prints the VM table: NAME, VCPU, RAM, STORAGE.
func WriteVMs(w io.Writer, vms []inventory.VMInfo) error {
	tw := newWriter(w)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			vm.Name, vm.NumCPU, format.HumanGB(int64(vm.MemoryMB)),
			format.HumanBytes(vm.StorageCommittedBytes)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// WriteDatastores prints the datastore table: NAME, TYPE, USED, AVAILABLE.
func WriteDatastores(w io.Writer, dss []inventory.DatastoreInfo) error {
	tw := newWriter(w)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			ds.Name, ds.Transport,
			format.HumanBytes(ds.UsedBytes), format.HumanBytes(ds.AvailableBytes)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// WriteSwitches prints the switch/port-group table:
// SWITCH, SWITCH TYPE, PORTGROUP, VLAN, UPLINKS, LACP, PORTS, USED.
//
// One row is printed per port group; a switch without port groups still
// gets a single row. Fields the API does not populate render as unknown/N/A.
func WriteSwitches(w io.Writer, switches []inventory.SwitchInfo) error {
	tw := newWriter(w)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, sw := range switches {
		if sw.TotalPorts == 0 && sw.UsedPorts == 0 && len(sw.PortGroups) == 0 {
			continue // nothing known about this switch at all
		}
		uplinks := strings.Join(sw.Uplinks, ",")
		if uplinks == "" {
			uplinks = "unknown"
		}
		rows := sw.PortGroups
		if len(rows) == 0 {
			rows = []inventory.PortGroupInfo{{Name: "-", VLAN: "-"}}
		}
		for _, pg := range rows {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				sw.Name, sw.Kind, pg.Name, pg.VLAN, uplinks, sw.LACP,
				portCount(sw.TotalPorts), portCount(sw.UsedPorts)); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
}

// portCount renders a port count; a negative value means unknown.
func portCount(n int64) string {
	if n < 0 {
		return "unknown"
	}
	return strconv.FormatInt(n, 10)
}
