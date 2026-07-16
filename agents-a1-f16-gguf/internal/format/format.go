package format

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"vsphere-inventory/internal/storage"
)

func HumanSize(bytes int64) string {
	size := float64(bytes)
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}

	unitIndex := 0
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d B", bytes)
	}

	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}

func FormatVMs(w io.Writer, vms []storage.VMInfo) {
	wr := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(wr, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(wr, "%s\t%d\t%.1f GB\t%s\n",
			vm.Name,
			vm.VCPU,
			vm.RAMGB,
			HumanSize(int64(vm.StorageGB*1024*1024*1024)),
		)
	}
	wr.Flush()
}

func FormatDatastores(w io.Writer, ds []storage.DatastoreInfo) {
	wr := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(wr, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, d := range ds {
		fmt.Fprintf(wr, "%s\t%s\t%s\t%s\n",
			d.Name,
			d.Type,
			HumanSize(int64(d.UsedGB*1024*1024*1024)),
			HumanSize(int64(d.AvailableGB*1024*1024*1024)),
		)
	}
	wr.Flush()
}

func FormatSwitches(w io.Writer, switches []storage.SwitchInfo) {
	wr := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(wr, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, sw := range switches {
		for _, pg := range sw.PortGroups {
			uplinks := strings.Join(sw.Uplinks, ",")
			fmt.Fprintf(wr, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				sw.Name,
				sw.SwitchType,
				pg.Name,
				pg.VLAN,
				uplinks,
				sw.LACP,
				sw.TotalPorts,
				sw.UsedPorts,
			)
		}
	}
	wr.Flush()
}

func FormatVMsByPortGroup(w io.Writer, vms []storage.VMInfoForPortGroup) {
	wr := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(wr, "VM NAME\tPORT GROUP")
	for _, vm := range vms {
		fmt.Fprintf(wr, "%s\t%s\n", vm.Name, vm.PortGroup)
	}
	wr.Flush()
}
