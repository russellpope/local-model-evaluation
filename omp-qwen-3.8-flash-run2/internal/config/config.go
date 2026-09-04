// Package config resolves shared vSphere connection settings with the
// precedence: command-line flag > environment variable > config file > default.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is prepended to upper-cased keys to form environment variables
// (e.g. url -> VSPHERE_URL).
const EnvPrefix = "VSPHERE"

// Keys of the shared connection configuration.
const (
	KeyURL      = "url"
	KeyUsername = "username"
	KeyPassword = "password"
	KeyInsecure = "insecure"
	KeyTimeout  = "timeout"
	KeyConfig   = "config"
)

// Settings holds the resolved connection configuration.
type Settings struct {
	URL        string
	Username   string
	Password   string
	Insecure   bool
	Timeout    time.Duration
	ConfigFile string
}

// NewViper builds the viper instance for a command's flag set: registers
// environment overrides (VSPHERE_ prefix), binds the given flags, and applies
// built-in defaults. Bind order matters for precedence: flag > env > file >
// default (viper consults changed flags first, then env, then config, then
// defaults).
func NewViper(fs *pflag.FlagSet) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := v.BindPFlags(fs); err != nil {
		return nil, fmt.Errorf("bind flags: %w", err)
	}
	v.SetDefault(KeyURL, "")
	v.SetDefault(KeyUsername, "")
	v.SetDefault(KeyPassword, "")
	v.SetDefault(KeyInsecure, false)
	v.SetDefault(KeyTimeout, "60s")
	return v, nil
}

// ReadFile merges a YAML config file into the instance, if a path is given.
// File values sit below env and flags in precedence.
func ReadFile(v *viper.Viper, path string) error {
	if path == "" {
		return nil
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}
	return nil
}

// Load resolves Settings from a configured viper instance.
func Load(v *viper.Viper) (Settings, error) {
	s := Settings{
		URL:        v.GetString(KeyURL),
		Username:   v.GetString(KeyUsername),
		Password:   v.GetString(KeyPassword),
		Insecure:   v.GetBool(KeyInsecure),
		Timeout:    v.GetDuration(KeyTimeout),
		ConfigFile: v.GetString(KeyConfig),
	}
	if s.Timeout <= 0 {
		return Settings{}, fmt.Errorf("timeout must be positive, got %s", s.Timeout)
	}
	return s, nil
}
