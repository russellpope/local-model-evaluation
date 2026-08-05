package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

const (
	EnvPrefix = "VSPHERE"

	DefaultTimeout = "60s"
)

type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

func FromViper(v *viper.Viper) (*Config, error) {
	timeoutStr := v.GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout %q: %w", timeoutStr, err)
	}

	return &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  timeout,
	}, nil
}

func Validate(cfg *Config) error {
	if cfg.URL == "" {
		return fmt.Errorf("vsphere URL is required (use --url, VSPHERE_URL env var, or config file)")
	}
	if cfg.Username == "" {
		return fmt.Errorf("username is required (use --username, VSPHERE_USERNAME env var, or config file)")
	}
	if cfg.Password == "" {
		return fmt.Errorf("password is required (use --password, VSPHERE_PASSWORD env var, or config file)")
	}
	return nil
}

func ResolveURL(cfg *Config) (string, error) {
	if cfg.URL == "" {
		return "", fmt.Errorf("vsphere URL is required (use --url, VSPHERE_URL env var, or config file)")
	}
	return cfg.URL, nil
}
