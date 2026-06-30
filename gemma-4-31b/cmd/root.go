package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"govmomi-cli/pkg/config"
)

var (
	cfgFile  string
	cfgViper *viper.Viper
)

var RootCmd = &cobra.Command{
	Use:   "govmomi-cli",
	Short: "vSphere Inventory CLI",
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	RootCmd.PersistentFlags().String("url", "", "vCenter URL or host")
	RootCmd.PersistentFlags().String("username", "", "vCenter username")
	RootCmd.PersistentFlags().String("password", "", "vCenter password")
	RootCmd.PersistentFlags().Bool("insecure", false, "skip TLS verification")
	RootCmd.PersistentFlags().String("timeout", "60s", "overall operation timeout")

	RootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return initConfig(cmd)
	}
}

func initConfig(cmd *cobra.Command) error {
	cfgViper = viper.New()
	cfgViper.SetEnvPrefix("VSPHERE")
	cfgViper.AutomaticEnv()

	// Bind flags from the command
	cfgViper.BindPFlag("url", cmd.Flags().Lookup("url"))
	cfgViper.BindPFlag("username", cmd.Flags().Lookup("username"))
	cfgViper.BindPFlag("password", cmd.Flags().Lookup("password"))
	cfgViper.BindPFlag("insecure", cmd.Flags().Lookup("insecure"))
	cfgViper.BindPFlag("timeout", cmd.Flags().Lookup("timeout"))

	if cfgFile != "" {
		cfgViper.SetConfigFile(cfgFile)
		if err := cfgViper.ReadInConfig(); err != nil {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	return nil
}

func GetConfig() (*config.Config, error) {
	if cfgViper == nil {
		return nil, fmt.Errorf("config not initialized")
	}
	return config.Resolve(cfgViper)
}
