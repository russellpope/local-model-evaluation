// Package config resolves shared vSphere connection settings.
//
// Precedence (highest first): command-line flag, environment variable
// (VSPHERE_*), YAML config file, built-in default.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// DefaultTimeout is the built-in overall operation timeout.
const DefaultTimeout = 60 * time.Second

// EnvPrefix is the environment variable prefix for every key.
const EnvPrefix = "VSPHERE"

// Keys are the configuration keys shared by all subcommands.
var Keys = []string{"url", "username", "password", "insecure", "timeout"}

// Settings holds fully resolved connection values.
type Settings struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// Config wraps a private viper instance so callers control precedence
// explicitly: defaults and env first, config file second, changed flags last.
type Config struct {
	v *viper.Viper
}

// New returns a Config seeded with built-in defaults and VSPHERE_* env binding.
func New() *Config {
	c := &Config{v: viper.New()}
	c.v.SetEnvPrefix(EnvPrefix)
	c.v.AutomaticEnv()
	for _, k := range Keys {
		c.v.BindEnv(k)
	}
	c.v.SetDefault("timeout", DefaultTimeout.String())
	c.v.SetDefault("insecure", false)
	return c
}

// ReadConfig loads an optional YAML config file. An empty path is a no-op.
func (c *Config) ReadConfig(path string) error {
	if path == "" {
		return nil
	}
	c.v.SetConfigFile(path)
	c.v.SetConfigType("yaml")
	if err := c.v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}
	return nil
}

// SetOverride records a value at the highest precedence tier. Callers must
// invoke it only for flags the user explicitly set.
func (c *Config) SetOverride(key, value string) {
	c.v.Set(key, value)
}

// Settings resolves every key through the precedence chain.
func (c *Config) Settings() (Settings, error) {
	s := Settings{
		URL:      c.v.GetString("url"),
		Username: c.v.GetString("username"),
		Password: c.v.GetString("password"),
		Insecure: c.v.GetBool("insecure"),
		Timeout:  c.v.GetDuration("timeout"),
	}
	if s.Timeout <= 0 {
		return Settings{}, fmt.Errorf("timeout must be greater than zero, got %q", c.v.GetString("timeout"))
	}
	return s, nil
}
