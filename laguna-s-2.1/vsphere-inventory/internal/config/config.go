package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	URL        string
	Username   string
	Password   string
	Insecure   bool
	Timeout    time.Duration
	ConfigFile string
	v          *viper.Viper
}

func New() *Config {
	return &Config{
		Timeout: 60 * time.Second,
		v:       viper.GetViper(),
	}
}

func NewWithViper(v *viper.Viper) *Config {
	return &Config{
		Timeout: 60 * time.Second,
		v:       v,
	}
}

func (c *Config) Load() error {
	cfgFile := c.v.GetString("config")
	if cfgFile != "" {
		c.v.SetConfigFile(cfgFile)
		if err := c.v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config file: %w", err)
		}
	}

	c.URL = c.v.GetString("url")
	c.Username = c.v.GetString("username")
	c.Password = c.v.GetString("password")
	c.Insecure = c.v.GetBool("insecure")
	c.ConfigFile = cfgFile

	timeoutStr := c.v.GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout value %q: %w", timeoutStr, err)
	}
	c.Timeout = timeout

	return nil
}

func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("url is required (set via --url, VSPHERE_URL env var, or config file)")
	}
	if c.Username == "" {
		return fmt.Errorf("username is required (set via --username, VSPHERE_USERNAME env var, or config file)")
	}
	if c.Password == "" {
		return fmt.Errorf("password is required (set via --password, VSPHERE_PASSWORD env var, or config file)")
	}
	return nil
}
