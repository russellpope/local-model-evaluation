package config

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

const EnvPrefix = "VSPHERE"

func BindFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.StringP("url", "u", "", fmt.Sprintf("vCenter URL (e.g. https://vc.lab/sdk). Env: %s_URL", EnvPrefix))
	flags.StringP("username", "U", "", fmt.Sprintf("vCenter username. Env: %s_USERNAME", EnvPrefix))
	flags.StringP("password", "P", "", fmt.Sprintf("vCenter password. Env: %s_PASSWORD", EnvPrefix))
	flags.BoolP("insecure", "k", false, fmt.Sprintf("Skip TLS verification. Env: %s_INSECURE", EnvPrefix))
	flags.DurationP("timeout", "t", 60*time.Second, fmt.Sprintf("Operation timeout. Env: %s_TIMEOUT", EnvPrefix))
	flags.StringP("config", "c", "", "Path to YAML config file")

	if err := v.BindPFlag("url", flags.Lookup("url")); err != nil {
		return fmt.Errorf("bind url flag: %w", err)
	}
	if err := v.BindPFlag("username", flags.Lookup("username")); err != nil {
		return fmt.Errorf("bind username flag: %w", err)
	}
	if err := v.BindPFlag("password", flags.Lookup("password")); err != nil {
		return fmt.Errorf("bind password flag: %w", err)
	}
	if err := v.BindPFlag("insecure", flags.Lookup("insecure")); err != nil {
		return fmt.Errorf("bind insecure flag: %w", err)
	}
	if err := v.BindPFlag("timeout", flags.Lookup("timeout")); err != nil {
		return fmt.Errorf("bind timeout flag: %w", err)
	}

	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()

	return nil
}

func Load(v *viper.Viper) (*Config, error) {
	configPath := v.GetString("config")

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file %s: %w", configPath, err)
		}
	}

	cfg := &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
	}

	return cfg, nil
}

func Validate(cfg *Config) error {
	if cfg.URL == "" {
		return fmt.Errorf("vCenter URL is required (use --url or %s_URL)", EnvPrefix)
	}
	if cfg.Username == "" {
		return fmt.Errorf("username is required (use --username or %s_USERNAME)", EnvPrefix)
	}
	if cfg.Password == "" {
		return fmt.Errorf("password is required (use --password or %s_PASSWORD)", EnvPrefix)
	}
	return nil
}
