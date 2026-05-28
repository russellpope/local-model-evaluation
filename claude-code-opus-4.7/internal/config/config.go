// Package config resolves the shared vCenter connection configuration with the
// precedence flag > environment variable > config file > built-in default,
// using Viper. The environment-variable prefix is VSPHERE_.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// EnvPrefix is the prefix for all environment variables (e.g. VSPHERE_URL).
const EnvPrefix = "VSPHERE"

// Configuration keys, shared by flags, env bindings, and the YAML file.
const (
	KeyURL      = "url"
	KeyUsername = "username"
	KeyPassword = "password"
	KeyInsecure = "insecure"
	KeyTimeout  = "timeout"
	KeyConfig   = "config"
)

// DefaultTimeout is the built-in overall operation timeout.
const DefaultTimeout = 60 * time.Second

// resolvableKeys are the keys that participate in the flag/env/file/default
// precedence chain. The "config" key only names the file to read and is not
// itself part of the resolved configuration.
var resolvableKeys = []string{KeyURL, KeyUsername, KeyPassword, KeyInsecure, KeyTimeout}

// Config is the resolved connection configuration.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// New returns a Viper instance primed with built-in defaults and environment
// variable binding (VSPHERE_ prefix). Bind command flags afterward with
// BindFlags and optionally read a file with LoadConfigFile; the resulting
// precedence is flag > env > file > default.
func New() *viper.Viper {
	v := viper.New()
	v.SetDefault(KeyInsecure, false)
	v.SetDefault(KeyTimeout, DefaultTimeout)

	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	for _, key := range resolvableKeys {
		// Bind explicitly so env resolution does not depend on a key having
		// first been seen in a config file.
		_ = v.BindEnv(key)
	}
	return v
}

// RegisterFlags defines the shared persistent flags on the command. Defining
// them in one place keeps the production command wiring and the precedence
// tests in agreement about flag names and defaults.
func RegisterFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.String(KeyURL, "", "vCenter URL or host, e.g. https://vc.lab/sdk (env VSPHERE_URL)")
	flags.String(KeyUsername, "", "vCenter username (env VSPHERE_USERNAME)")
	flags.String(KeyPassword, "", "vCenter password (env VSPHERE_PASSWORD)")
	flags.Bool(KeyInsecure, false, "skip TLS certificate verification (env VSPHERE_INSECURE)")
	flags.Duration(KeyTimeout, DefaultTimeout, "overall operation timeout, e.g. 60s (env VSPHERE_TIMEOUT)")
	flags.String(KeyConfig, "", "path to a YAML config file")
}

// BindFlags binds the resolvable command flags to the Viper instance so a flag
// that the user actually set takes precedence over env and file values. An
// unset flag falls through to env/file/default, which is what gives the
// documented precedence order.
func BindFlags(v *viper.Viper, cmd *cobra.Command) error {
	for _, key := range resolvableKeys {
		f := cmd.Flags().Lookup(key)
		if f == nil {
			continue
		}
		if err := v.BindPFlag(key, f); err != nil {
			return fmt.Errorf("binding flag %q: %w", key, err)
		}
	}
	return nil
}

// LoadConfigFile reads the YAML config file at path into the Viper instance.
func LoadConfigFile(v *viper.Viper, path string) error {
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("reading config file %q: %w", path, err)
	}
	return nil
}

// Resolve reads the effective configuration out of the Viper instance.
func Resolve(v *viper.Viper) Config {
	return Config{
		URL:      v.GetString(KeyURL),
		Username: v.GetString(KeyUsername),
		Password: v.GetString(KeyPassword),
		Insecure: v.GetBool(KeyInsecure),
		Timeout:  v.GetDuration(KeyTimeout),
	}
}
