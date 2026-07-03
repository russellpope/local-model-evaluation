package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

type Config struct {
	URL      string        `mapstructure:"url"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	Insecure bool          `mapstructure:"insecure"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Config   string        `mapstructure:"config"`
}

var rootCmd = &cobra.Command{
	Use:   "govmomi-cli",
	Short: "vSphere Inventory CLI",
	Long:  "A command-line application that connects to a VMware vCenter Server and reports virtualization inventory.",
}

func initConfig() error {
	viper.SetDefault("url", "")
	viper.SetDefault("username", "")
	viper.SetDefault("password", "")
	viper.SetDefault("insecure", false)
	viper.SetDefault("timeout", 60*time.Second)

	viper.SetEnvPrefix("VSPHERE")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	viper.SetConfigType("yaml")

	rootCmd.PersistentFlags().String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk")
	viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))

	rootCmd.PersistentFlags().String("username", "", "vCenter username")
	viper.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))

	rootCmd.PersistentFlags().String("password", "", "vCenter password")
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))

	rootCmd.PersistentFlags().Bool("insecure", false, "skip TLS verification")
	viper.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))

	rootCmd.PersistentFlags().Duration("timeout", 60*time.Second, "overall operation timeout")
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))

	rootCmd.PersistentFlags().String("config", "", "optional path to a YAML config file")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		configFlag := viper.GetString("config")
		if configFlag != "" {
			viper.SetConfigFile(configFlag)
			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config file: %w", err)
			}
		}
		return nil
	}

	return nil
}

func connect(ctx context.Context, cfg Config) (*govmomi.Client, error) {
	urlStr := cfg.URL
	if urlStr == "" {
		return nil, fmt.Errorf("url is required")
	}

	u, err := soap.ParseURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	u.User = url.UserPassword(cfg.Username, cfg.Password)

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vCenter: %w", err)
	}

	return client, nil
}

func main() {
	if err := initConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)

	rootCmd.Execute()
}
