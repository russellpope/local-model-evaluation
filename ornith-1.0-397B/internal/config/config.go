package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the resolved vSphere connection configuration.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// Load resolves configuration from flags, environment, config file, and defaults.
// Precedence (highest first): flags > env vars > config file > defaults.
// flagOverrides maps flag names to their values; empty string means "not set".
func Load(flagOverrides map[string]string, envPrefix string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("url", "https://localhost/sdk")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", "60s")

	// Environment variables
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	// Explicitly ignore BindEnv errors (L4): BindEnv only errors on empty key
	// list, which is a programmer error, not a runtime condition.
	_ = v.BindEnv("url", envPrefix+"_URL")
	_ = v.BindEnv("username", envPrefix+"_USERNAME")
	_ = v.BindEnv("password", envPrefix+"_PASSWORD")
	_ = v.BindEnv("insecure", envPrefix+"_INSECURE")
	_ = v.BindEnv("timeout", envPrefix+"_TIMEOUT")

	// Config file (optional)
	if cfgFile, ok := flagOverrides["config"]; ok && cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	// Flag overrides (highest precedence)
	for key, val := range flagOverrides {
		if val != "" {
			v.Set(key, val)
		}
	}

	cfg := &Config{}

	cfg.URL = v.GetString("url")
	cfg.Username = v.GetString("username")
	cfg.Password = v.GetString("password")
	cfg.Insecure = v.GetBool("insecure")

	timeoutStr := v.GetString("timeout")
	d, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("parse timeout %q: %w", timeoutStr, err)
	}
	cfg.Timeout = d

	return cfg, nil
}
