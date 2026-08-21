// Package config wires connection settings through Viper with a fixed
// precedence: command-line flag > environment variable > config file > default.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is the environment-variable prefix for all settings.
const EnvPrefix = "VSPHERE"

// DefaultTimeout is the overall operation timeout used when none is configured.
const DefaultTimeout = 60 * time.Second

// Config holds the resolved connection settings for a command.
type Config struct {
	// URL is the vCenter URL or host, e.g. https://vc.lab/sdk.
	URL string
	// Username authenticates against the vCenter.
	Username string
	// Password authenticates against the vCenter.
	Password string
	// Insecure skips TLS certificate verification.
	Insecure bool
	// Timeout bounds the overall operation.
	Timeout time.Duration
}

// Flags returns a FlagSet with every connection flag defined. It is used both
// by the root command and by tests so the flag surface stays in one place.
func Flags() *pflag.FlagSet {
	f := pflag.NewFlagSet("vsphere-inventory", pflag.ContinueOnError)
	f.String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk")
	f.String("username", "", "vCenter username")
	f.String("password", "", "vCenter password")
	f.Bool("insecure", false, "skip TLS certificate verification")
	f.Duration("timeout", DefaultTimeout, "overall operation timeout")
	f.String("config", "", "path to a YAML config file")
	return f
}

// FromFlags resolves the configuration using the given flag set. Precedence is
// flag (when set) > environment variable > config file > built-in default.
func FromFlags(flags *pflag.FlagSet) (*Config, error) {
	if flags == nil {
		return nil, fmt.Errorf("no flag set provided")
	}

	v := viper.New()
	v.SetConfigType("yaml")

	// Built-in defaults (lowest precedence).
	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", DefaultTimeout)

	// Config file, when one is supplied.
	cfgPath, _ := flags.GetString("config")
	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file %q: %w", cfgPath, err)
		}
	}

	// Environment variables, highest after explicit flags.
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	bindEnv(v, "url", EnvPrefix+"_URL")
	bindEnv(v, "username", EnvPrefix+"_USERNAME")
	bindEnv(v, "password", EnvPrefix+"_PASSWORD")
	bindEnv(v, "insecure", EnvPrefix+"_INSECURE")
	bindEnv(v, "timeout", EnvPrefix+"_TIMEOUT")

	// Command-line flags. Viper only consults a flag when it was set on the
	// command line, so an unset flag falls through to env/file/default.
	if err := v.BindPFlags(flags); err != nil {
		return nil, fmt.Errorf("binding flags: %w", err)
	}

	return &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
	}, nil
}

func bindEnv(v *viper.Viper, key, env string) {
	_ = v.BindEnv(key, env)
}
