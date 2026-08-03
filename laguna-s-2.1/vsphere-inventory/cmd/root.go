package cmd

import (
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "vSphere inventory CLI",
	Long:  "A CLI tool for reporting VMware vSphere virtualization inventory.",
}

func Execute() error {
	cfg = config.New()

	viper.SetEnvPrefix("VSPHERE")
	viper.AutomaticEnv()
	viper.SetDefault("timeout", "60s")
	viper.SetDefault("insecure", false)

	rootCmd.PersistentFlags().StringP("url", "u", "", "vCenter URL or host (e.g. https://vc.lab/sdk)")
	rootCmd.PersistentFlags().StringP("username", "U", "", "vCenter username")
	rootCmd.PersistentFlags().StringP("password", "P", "", "vCenter password")
	rootCmd.PersistentFlags().BoolP("insecure", "k", false, "skip TLS verification")
	rootCmd.PersistentFlags().StringP("timeout", "t", "60s", "overall operation timeout")
	rootCmd.PersistentFlags().StringP("config", "c", "", "path to config file")

	viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	viper.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)

	return rootCmd.Execute()
}

func loadConfig() (*config.Config, error) {
	if err := cfg.Load(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
