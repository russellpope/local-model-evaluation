package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
	Config   string
}

type ViperConfig struct {
	v     *viper.Viper
	Cfg   Config
	bound bool
}

func NewConfig() *ViperConfig {
	return &ViperConfig{
		v: viper.New(),
	}
}

func (c *ViperConfig) BindFlags(cmd *cobra.Command) {
	if c.bound {
		return
	}

	c.v.SetEnvPrefix("VSPHERE")
	c.v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	c.v.AutomaticEnv()

	c.v.SetDefault("url", "https://localhost/sdk")
	c.v.SetDefault("username", "")
	c.v.SetDefault("password", "")
	c.v.SetDefault("insecure", false)
	c.v.SetDefault("timeout", "60s")

	cmd.Flags().StringVar(&c.Cfg.Config, "config", "", "Path to config file")
	cmd.Flags().String("url", "", "vCenter URL (e.g. https://vc.lab/sdk)")
	cmd.Flags().String("username", "", "vCenter username")
	cmd.Flags().String("password", "", "vCenter password")
	cmd.Flags().Bool("insecure", false, "Skip TLS verification")
	cmd.Flags().Duration("timeout", 60*time.Second, "Operation timeout")

	c.v.BindPFlag("url", cmd.Flags().Lookup("url"))
	c.v.BindPFlag("username", cmd.Flags().Lookup("username"))
	c.v.BindPFlag("password", cmd.Flags().Lookup("password"))
	c.v.BindPFlag("insecure", cmd.Flags().Lookup("insecure"))
	c.v.BindPFlag("timeout", cmd.Flags().Lookup("timeout"))

	c.bound = true
}

func (c *ViperConfig) Load() (*Config, error) {
	if c.Cfg.Config != "" {
		c.v.SetConfigFile(c.Cfg.Config)
		if err := c.v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := c.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if cfg.URL == "" {
		cfg.URL = c.v.GetString("url")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = c.v.GetDuration("timeout")
	}

	return &cfg, nil
}
