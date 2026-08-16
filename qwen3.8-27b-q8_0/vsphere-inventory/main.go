package main

import (
	"os"

	"github.com/local-model-evaluation/vsphere-inventory/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
