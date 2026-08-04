package cmd

import (
	"bufio"
	"context"
	"os"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:           "vsphere-inventory",
	Short:         "vSphere inventory CLI",
	Long:          "A CLI tool for reporting VMware vSphere virtualization inventory.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if viper.GetBool("password-stdin") {
			reader := bufio.NewReader(os.Stdin)
			pass, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			viper.Set("password", pass)
		}
		return nil
	},
}

func init() {
	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("VSPHERE")
		viper.AutomaticEnv()
		viper.SetDefault("timeout", "60s")
		viper.SetDefault("insecure", false)
	})

	rootCmd.PersistentFlags().StringP("url", "u", "", "vCenter URL or host (e.g. https://vc.lab/sdk)")
	rootCmd.PersistentFlags().StringP("username", "U", "", "vCenter username")
	rootCmd.PersistentFlags().StringP("password", "P", "", "vCenter password")
	rootCmd.PersistentFlags().Bool("password-stdin", false, "read password from stdin")
	rootCmd.PersistentFlags().BoolP("insecure", "k", false, "skip TLS verification")
	rootCmd.PersistentFlags().StringP("timeout", "t", "60s", "overall operation timeout")
	rootCmd.PersistentFlags().StringP("config", "c", "", "path to config file")

	bindFlag := func(key string, flagName string) {
		flag := rootCmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			panic("flag " + flagName + " not found")
		}
		if err := viper.BindPFlag(key, flag); err != nil {
			panic("binding flag " + flagName + ": " + err.Error())
		}
	}
	bindFlag("url", "url")
	bindFlag("username", "username")
	bindFlag("password", "password")
	bindFlag("password-stdin", "password-stdin")
	bindFlag("insecure", "insecure")
	bindFlag("timeout", "timeout")
	bindFlag("config", "config")

	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)
}

func Execute() error {
	return ExecuteContext(context.Background())
}

func ExecuteContext(ctx context.Context) error {
	cfg = config.New()
	rootCmd.SetContext(ctx)
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
