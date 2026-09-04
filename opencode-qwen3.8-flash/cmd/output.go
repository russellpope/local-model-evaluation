package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/local-model-evaluation/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

const (
	tabMinWidth = 2
	tabPadding  = 2
)

func newTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, tabMinWidth, 0, tabPadding, ' ', 0)
}

// renderVMs writes the VM table: NAME, VCPU, RAM, STORAGE (consumed).
func renderVMs(w io.Writer, vms []inventory.VMInfo) error {
	tw := newTable(w)
	if _, err := fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE"); err != nil {
		return fmt.Errorf("write VM table: %w", err)
	}
	for _, vm := range vms {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", vm.Name, vm.NumCPU, format.MemoryMB(vm.MemoryMB), format.Bytes(vm.Committed)); err != nil {
			return fmt.Errorf("write VM table: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush VM table: %w", err)
	}
	return nil
}

// renderDatastores writes the datastore table: NAME, TYPE (transport), USED,
// AVAILABLE.
func renderDatastores(w io.Writer, dss []inventory.DatastoreInfo) error {
	tw := newTable(w)
	if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE"); err != nil {
		return fmt.Errorf("write datastore table: %w", err)
	}
	for _, ds := range dss {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, format.Bytes(ds.Used), format.Bytes(ds.Available)); err != nil {
			return fmt.Errorf("write datastore table: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush datastore table: %w", err)
	}
	return nil
}

// renderSwitches writes the vswitch table, one row per port group.
func renderSwitches(w io.Writer, switches []inventory.SwitchInfo) error {
	tw := newTable(w)
	if _, err := fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED"); err != nil {
		return fmt.Errorf("write switch table: %w", err)
	}
	for _, sw := range switches {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			sw.Switch, sw.SwitchType, sw.Portgroup, sw.VLAN, sw.Uplinks, sw.LACP, sw.Ports, sw.Used); err != nil {
			return fmt.Errorf("write switch table: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush switch table: %w", err)
	}
	return nil
}
