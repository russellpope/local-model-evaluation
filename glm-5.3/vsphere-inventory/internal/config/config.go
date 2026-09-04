// Package config resolves the shared vCenter connection configuration.
//
// Resolution follows a fixed precedence (highest first):
//
//  1. command-line flag
//  2. environment variable (prefix VSPHERE_)
//  3. YAML config file
//  4. built-in default
//
// The CLI wiring that connects cobra flags and viper lives in the cmd
// package; this package owns the keys, defaults, resolution and validation.
package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// EnvPrefix is the prefix for environment variables (e.g. VSPHERE_URL).
const EnvPrefix = "VSPHERE"

// Keys are the supported configuration field names.
const (
	KeyURL      = "url"
	KeyUsername = "username"
	KeyPassword = "password"
	KeyInsecure = "insecure"
	KeyTimeout  = "timeout"
)

// Keys lists all configuration keys in canonical order.
var Keys = []string{KeyURL, KeyUsername, KeyPassword, KeyInsecure, KeyTimeout}

// Defaults returns the built-in defaults for every configuration key.
func Defaults() map[string]any {
	return map[string]any{
		KeyURL:      "",
		KeyUsername: "",
		KeyPassword: "",
		KeyInsecure: false,
		KeyTimeout:  "60s",
	}
}

// Config is the resolved connection configuration.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// Load resolves the final configuration from a bound viper instance, which
// must already have an optional config file loaded (ReadInConfig).
func Load(v *viper.Viper) (*Config, error) {
	cfg := &Config{
		URL:      v.GetString(KeyURL),
		Username: v.GetString(KeyUsername),
		Password: v.GetString(KeyPassword),
		Insecure: v.GetBool(KeyInsecure),
	}

	if cfg.URL == "" {
		return nil, errors.New("no vCenter URL configured: set --url, VSPHERE_URL, or the url key in the config file")
	}
	if cfg.Username == "" {
		return nil, errors.New("no vCenter username configured: set --username, VSPHERE_USERNAME, or the username key in the config file")
	}

	timeout := v.GetString(KeyTimeout)
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout %q: use a Go duration such as \"30s\" or \"2m\"", timeout)
	}
	if d <= 0 {
		return nil, fmt.Errorf("timeout must be positive, got %s", d)
	}
	cfg.Timeout = d

	return cfg, nil
}
