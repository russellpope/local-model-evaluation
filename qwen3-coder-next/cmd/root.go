package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "vSphere Inventory CLI",
	Long:  "A CLI tool to query VMware vSphere inventory information",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	config.Init()
	config.BindFlags(rootCmd)
}
