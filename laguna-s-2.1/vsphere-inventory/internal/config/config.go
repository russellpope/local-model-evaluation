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
}

func New() *Config {
	return &Config{
		Timeout: 60 * time.Second,
	}
}

func (c *Config) Load() error {
	cfgFile := viper.GetString("config")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config file: %w", err)
		}
	}

	c.URL = viper.GetString("url")
	c.Username = viper.GetString("username")
	c.Password = viper.GetString("password")
	c.Insecure = viper.GetBool("insecure")
	c.ConfigFile = cfgFile

	timeoutStr := viper.GetString("timeout")
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
