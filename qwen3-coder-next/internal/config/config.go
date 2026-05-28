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

const (
	defaultURL      = "https://localhost/sdk"
	defaultUsername = ""
	defaultPassword = ""
	defaultInsecure = false
	defaultTimeout  = 60 * time.Second
)

var v *viper.Viper

func Init() error {
	v = viper.New()

	v.SetDefault("url", defaultURL)
	v.SetDefault("username", defaultUsername)
	v.SetDefault("password", defaultPassword)
	v.SetDefault("insecure", defaultInsecure)
	v.SetDefault("timeout", defaultTimeout)

	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()

	return nil
}

func BindFlags(cmd *cobra.Command) error {
	cmd.Flags().String("url", "", "vCenter URL or host")
	cmd.Flags().String("username", "", "vCenter username")
	cmd.Flags().String("password", "", "vCenter password")
	cmd.Flags().Bool("insecure", false, "skip TLS verification")
	cmd.Flags().Duration("timeout", defaultTimeout, "operation timeout")
	cmd.Flags().String("config", "", "path to config file")

	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return nil
}

func LoadConfig() (*Config, error) {
	if v == nil {
		if err := Init(); err != nil {
			return nil, fmt.Errorf("failed to init viper: %w", err)
		}
	}

	if configPath := v.GetString("config"); configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
	}

	cfg := &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
		Config:   v.GetString("config"),
	}

	if cfg.URL == "" {
		cfg.URL = defaultURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	return cfg, nil
}

func GetString(key string) string {
	if v == nil {
		return ""
	}
	return v.GetString(key)
}

func GetBool(key string) bool {
	if v == nil {
		return false
	}
	return v.GetBool(key)
}

func GetDuration(key string) time.Duration {
	if v == nil {
		return 0
	}
	return v.GetDuration(key)
}

func init() {
	_ = fmt.Errorf
}
