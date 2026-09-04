// Package config wires vSphere connection settings through Viper with
// precedence: command-line flag > environment variable > config file > default.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is the prefix for every environment variable. Viper appends the
// underscore itself, so VSPHERE_URL et al. are the effective names.
const EnvPrefix = "VSPHERE"

// Config keys.
const (
	KeyURL      = "url"
	KeyUsername = "username"
	KeyPassword = "password"
	KeyInsecure = "insecure"
	KeyTimeout  = "timeout"
)

// Config bundles a Viper instance with the shared connection settings.
type Config struct {
	viper *viper.Viper
}

// New returns a Config with built-in defaults registered.
func New() *Config {
	v := viper.New()
	v.SetDefault(KeyURL, "")
	v.SetDefault(KeyUsername, "")
	v.SetDefault(KeyPassword, "")
	v.SetDefault(KeyInsecure, false)
	v.SetDefault(KeyTimeout, 60*time.Second)
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	return &Config{viper: v}
}

// RegisterFlags defines the persistent flags on the given FlagSet and binds
// them to Viper. Binding happens against the live flag values, so a flag that
// was explicitly set wins over environment and config-file sources.
func (c *Config) RegisterFlags(fs *pflag.FlagSet) error {
	fs.String(KeyURL, "", "vCenter URL or host, e.g. https://vc.lab/sdk (env VSPHERE_URL)")
	fs.String(KeyUsername, "", "vCenter username (env VSPHERE_USERNAME)")
	fs.String(KeyPassword, "", "vCenter password (env VSPHERE_PASSWORD)")
	fs.Bool(KeyInsecure, false, "skip TLS certificate verification (env VSPHERE_INSECURE)")
	fs.Duration(KeyTimeout, 60*time.Second, "overall operation timeout, e.g. 30s or 2m (env VSPHERE_TIMEOUT)")
	fs.String("config", "", "path to a YAML config file")
	if err := c.viper.BindPFlags(fs); err != nil {
		return fmt.Errorf("bind flags to configuration: %w", err)
	}
	return nil
}

// LoadConfigFile makes Viper read the YAML file at path. It is optional: an
// empty path is a no-op. A non-empty path that fails to parse is returned as
// an error so the user gets an actionable message.
func (c *Config) LoadConfigFile(path string) error {
	if path == "" {
		return nil
	}
	c.viper.SetConfigFile(path)
	c.viper.SetConfigType("yaml")
	if err := c.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}
	return nil
}

// Values is a resolved snapshot of the connection configuration.
type Values struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// Resolve returns the effective configuration for the given key set, applying
// Viper's precedence: flag > env > config file > default.
func (c *Config) Resolve() (Values, error) {
	var out Values
	if key := c.viper.GetString(KeyURL); key == "" {
		return out, fmt.Errorf("no vSphere URL configured: set --url, VSPHERE_URL, or a config file")
	} else {
		out.URL = key
	}
	out.Username = c.viper.GetString(KeyUsername)
	out.Password = c.viper.GetString(KeyPassword)
	out.Insecure = c.viper.GetBool(KeyInsecure)
	timeout := c.viper.GetDuration(KeyTimeout)
	if timeout <= 0 {
		return out, fmt.Errorf("invalid timeout %q: must be positive", c.viper.Get(KeyTimeout))
	}
	out.Timeout = timeout
	return out, nil
}
