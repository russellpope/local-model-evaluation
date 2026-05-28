package ui

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/model"
)

func PrintVMs(w io.Writer, vms []model.VMInfo) error {
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
