package ui

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/model"
)

func PrintVSwitches(w io.Writer, switches []model.SwitchInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	defer tw.Flush()

	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")

	for _, sw := range switches {
		uplinks := strings.Join(sw.Uplinks, ",")
		if uplinks == "" {
			uplinks = "N/A"
		}

		totalPorts := sw.TotalPorts
		if totalPorts == 0 {
			totalPorts = int32(len(sw.PortgroupName))
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			sw.SwitchName,
			sw.SwitchType,
			sw.PortgroupName,
			sw.VLAN,
			uplinks,
			sw.LACP,
			totalPorts,
			sw.UsedPorts,
		)
	}

	return nil
}

func PrintVMsByPortgroup(w io.Writer, vms []model.VMInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	defer tw.Flush()

	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")

	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%.1f GB\t%s\n", vm.Name, vm.VCPU, vm.RAMGB(), vm.StorageHuman())
	}

	return nil
}

func init() {
	_ = fmt.Fprintln
}
