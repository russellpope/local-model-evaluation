package format

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/model"
)

func Bytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	if exp >= len(units) {
		return fmt.Sprintf("%.1f EiB", float64(b)/float64(div))
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

type DatastoreInfo struct {
	Name           string
	Type           string
	UsedBytes      int64
	AvailableBytes int64
	CapacityBytes  int64
}

type SwitchInfo struct {
	Name       string
	Type       string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	TotalPorts int
	UsedPorts  int
}

func RenderVMs(w io.Writer, vms []model.VMInfo) {
	sort.Slice(vms, func(i, j int) bool {
		return vms[i].Name < vms[j].Name
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", vm.Name, vm.VCPU, RAMBytes(vm.RAMMB), Bytes(vm.StorageBytes))
	}
	tw.Flush()
}

func RenderDatastores(w io.Writer, datastores []DatastoreInfo) {
	sort.Slice(datastores, func(i, j int) bool {
		return datastores[i].Name < datastores[j].Name
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range datastores {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, Bytes(ds.UsedBytes), Bytes(ds.AvailableBytes))
	}
	tw.Flush()
}

func RenderVSwitches(w io.Writer, switches []SwitchInfo) {
	sort.Slice(switches, func(i, j int) bool {
		if switches[i].Name != switches[j].Name {
			return switches[i].Name < switches[j].Name
		}
		return switches[i].Portgroup < switches[j].Portgroup
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, sw := range switches {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			sw.Name, sw.Type, sw.Portgroup, sw.VLAN, sw.Uplinks, sw.LACP, sw.TotalPorts, sw.UsedPorts)
	}
	tw.Flush()
}

func RAMBytes(ramMB int) string {
	return Bytes(int64(ramMB) * 1024 * 1024)
}
