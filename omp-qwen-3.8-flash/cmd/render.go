package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/ldh/vsphere-inventory/internal/format"
	"github.com/ldh/vsphere-inventory/internal/inventory"
)

// stdout is overridable in tests.
var stdout io.Writer = os.Stdout

// newTabWriter returns an aligned, space-padded table writer writing to w.
func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
}

// printTable renders header + rows via tabwriter and flushes once.
func printTable(w io.Writer, header string, rows [][]string) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, header)
	for _, r := range rows {
		fmt.Fprintln(tw, joinTab(r))
	}
	return tw.Flush()
}

func joinTab(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += "\t"
		}
		out += c
	}
	return out
}

func vmRows(vms []inventory.VMInfo) [][]string {
	rows := make([][]string, 0, len(vms))
	for _, v := range vms {
		rows = append(rows, []string{
			v.Name,
			fmt.Sprintf("%d", v.NumCPU),
			format.MemoryGB(v.MemoryMB),
			format.Bytes(v.Committed),
		})
	}
	return rows
}

func datastoreRows(dss []inventory.DatastoreInfo) [][]string {
	rows := make([][]string, 0, len(dss))
	for _, d := range dss {
		rows = append(rows, []string{
			d.Name,
			d.Transport,
			format.Bytes(d.Used),
			format.Bytes(d.Available),
		})
	}
	return rows
}

func portgroupRows(pgs []inventory.PortgroupInfo) [][]string {
	rows := make([][]string, 0, len(pgs))
	for _, p := range pgs {
		rows = append(rows, []string{
			p.Switch,
			p.SwitchType,
			p.Portgroup,
			p.VLAN,
			p.Uplinks,
			p.LACP,
			fmt.Sprintf("%d", p.Ports),
			fmt.Sprintf("%d", p.Used),
		})
	}
	return rows
}
