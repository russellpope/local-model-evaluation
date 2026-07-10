package main

import (
	"fmt"
	"io"
	"text/tabwriter"
)

func WriteVMs(w io.Writer, rows []VMInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", row.Name, row.VCPU, FormatBytes(row.RAMBytes), FormatBytes(row.StorageBytes)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func WriteDatastores(w io.Writer, rows []DatastoreInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.Name, row.Type, FormatBytes(row.UsedBytes), FormatBytes(row.AvailableBytes)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func WriteSwitches(w io.Writer, rows []SwitchInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			row.Switch,
			row.SwitchType,
			row.PortGroup,
			row.VLAN,
			row.Uplinks,
			row.LACP,
			row.TotalPorts,
			row.UsedPorts,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
