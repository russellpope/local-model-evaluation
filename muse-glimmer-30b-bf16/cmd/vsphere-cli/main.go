package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/config"
)

var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "vsphere-cli",
		Short: "vSphere inventory CLI",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Init(cfgFile); err != nil {
				return err
			}
			return nil
		},
	}
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().String("url", "", "vCenter URL")
	rootCmd.PersistentFlags().String("username", "", "username")
	rootCmd.PersistentFlags().String("password", "", "password")
	rootCmd.PersistentFlags().Bool("insecure", false, "skip TLS verification")
	rootCmd.PersistentFlags().String("timeout", "60s", "timeout")
	_ = viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	_ = viper.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))
	_ = viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	_ = viper.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
