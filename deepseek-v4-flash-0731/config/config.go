// Package config resolves vCenter connection settings with strict
// flag > environment > config file > default precedence (via Viper).
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	// EnvPrefix is the environment-variable prefix for all vCenter settings.
	EnvPrefix = "VSPHERE"

	// DefaultTimeout applies when no source provides a value.
	DefaultTimeout = 60 * time.Second
)

// Config is the resolved, fully-populated set of connection options.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// Load resolves configuration. flags may be nil; cfgFile may be empty.
// Precedence, highest first:
//
//	command-line flag > environment variable > config file > default
func Load(flags *pflag.FlagSet, cfgFile string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	v.SetDefault("insecure", false)
	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("timeout", DefaultTimeout.String())

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", cfgFile, err)
		}
	}

	cfg := Config{
		URL:      stringFrom(flags, "url", v.GetString("url")),
		Username: stringFrom(flags, "username", v.GetString("username")),
		Password: stringFrom(flags, "password", v.GetString("password")),
		Insecure: boolFrom(flags, "insecure", v.GetBool("insecure")),
	}

	t, err := timeoutFrom(flags, "timeout", v.Get("timeout"))
	if err != nil {
		return nil, fmt.Errorf("resolve timeout: %w", err)
	}
	if t <= 0 {
		t = DefaultTimeout
	}
	cfg.Timeout = t

	return &cfg, nil
}

// stringFrom returns the flag value when the flag is present on the command
// line, otherwise fallback (which itself comes from env > file > default).
func stringFrom(flags *pflag.FlagSet, name, fallback string) string {
	if flags != nil {
		if f := flags.Lookup(name); f != nil && f.Changed {
			if s, err := flags.GetString(name); err == nil {
				return s
			}
		}
	}
	return fallback
}

func boolFrom(flags *pflag.FlagSet, name string, fallback bool) bool {
	if flags != nil {
		if f := flags.Lookup(name); f != nil && f.Changed {
			if b, err := flags.GetBool(name); err == nil {
				return b
			}
		}
	}
	return fallback
}

// timeoutFrom prefers an explicitly-set flag, otherwise parses the lower
// precedence layers (environment / config file / default).
func timeoutFrom(flags *pflag.FlagSet, name string, fallback any) (time.Duration, error) {
	if flags != nil {
		if f := flags.Lookup(name); f != nil && f.Changed {
			if d, err := flags.GetDuration(name); err == nil && d > 0 {
				return d, nil
			}
		}
	}
	return parseTimeout(fallback)
}

// parseTimeout accepts duration strings ("90s"), bare second counts (90, from
// a YAML scalar or env var), or a time.Duration already provided by Viper.
func parseTimeout(raw any) (time.Duration, error) {
	switch v := raw.(type) {
	case time.Duration:
		return v, nil
	case int:
		return time.Duration(v) * time.Second, nil
	case int64:
		return time.Duration(v) * time.Second, nil
	case float64:
		return time.Duration(v) * time.Second, nil
	case string:
		s := strings.TrimSpace(v)
		if d, err := time.ParseDuration(s); err == nil {
			return d, nil
		}
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second, nil
		}
		return 0, fmt.Errorf("invalid timeout %q (use e.g. %q or a bare second count)", s, DefaultTimeout)
	default:
		return 0, fmt.Errorf("invalid timeout value %v", raw)
	}
}
