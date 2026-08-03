package main

import (
	"fmt"
	"os"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
