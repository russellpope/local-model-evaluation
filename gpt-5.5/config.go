package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type appConfig struct {
	cfg Config
}

func NewRootCommand() *cobra.Command {
	state := &appConfig{}

	cmd := &cobra.Command{
		Use:          "vsphere-inventory",
		Short:        "Report vSphere inventory",
		SilenceUsage: true,
	}

	flags := cmd.PersistentFlags()
	flags.String("url", "", "vCenter URL or host")
	flags.String("username", "", "vCenter username")
	flags.String("password", "", "vCenter password")
	flags.Bool("insecure", false, "skip TLS verification")
	flags.Duration("timeout", 60*time.Second, "overall operation timeout")
	flags.String("config", "", "optional YAML config file")

	cmd.AddCommand(newVMsCommand(state), newDatastoresCommand(state), newSwitchesCommand(state))
	return cmd
}

func LoadConfigForCommand(cmd *cobra.Command) (Config, *viper.Viper, error) {
	root := cmd.Root()
	if !root.PersistentFlags().Parsed() {
		if err := root.ParseFlags(root.Flags().Args()); err != nil {
			return Config{}, nil, fmt.Errorf("parse flags: %w", err)
		}
	}

	vp := viper.New()
	vp.SetEnvPrefix("VSPHERE")
	vp.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	vp.AutomaticEnv()

	vp.SetDefault("url", "")
	vp.SetDefault("username", "")
	vp.SetDefault("password", "")
	vp.SetDefault("insecure", false)
	vp.SetDefault("timeout", 60*time.Second)

	for _, key := range []string{"url", "username", "password", "insecure", "timeout"} {
		if err := vp.BindPFlag(key, root.PersistentFlags().Lookup(key)); err != nil {
			return Config{}, nil, fmt.Errorf("bind flag %q: %w", key, err)
		}
	}

	configFile, err := root.PersistentFlags().GetString("config")
	if err != nil {
		return Config{}, nil, fmt.Errorf("read config flag: %w", err)
	}
	if configFile != "" {
		vp.SetConfigFile(configFile)
		if err := vp.ReadInConfig(); err != nil {
			return Config{}, nil, fmt.Errorf("read config file %q: %w", configFile, err)
		}
	}

	cfg := Config{
		URL:        vp.GetString("url"),
		Username:   vp.GetString("username"),
		Password:   vp.GetString("password"),
		Insecure:   vp.GetBool("insecure"),
		Timeout:    vp.GetDuration("timeout"),
		ConfigFile: configFile,
	}

	return cfg, vp, nil
}

func (a *appConfig) load(cmd *cobra.Command) error {
	cfg, _, err := LoadConfigForCommand(cmd)
	if err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}
