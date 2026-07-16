package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
	Config   string
}

func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	if configPath != "" {
		v.SetConfigFile(configPath)
		// Only return error if config file exists but can't be read
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	config := &Config{}

	v.SetDefault("insecure", false)
	v.SetDefault("timeout", 60*time.Second)

	v.BindEnv("url", "VSPHERE_URL")
	v.BindEnv("username", "VSPHERE_USERNAME")
	v.BindEnv("password", "VSPHERE_PASSWORD")
	v.BindEnv("insecure", "VSPHERE_INSECURE")
	v.BindEnv("timeout", "VSPHERE_TIMEOUT")

	config.URL = v.GetString("url")
	config.Username = v.GetString("username")
	config.Password = v.GetString("password")
	config.Insecure = v.GetBool("insecure")
	config.Timeout = v.GetDuration("timeout")
	config.Config = configPath

	if config.URL == "" {
		return nil, fmt.Errorf("VSPHERE_URL is required")
	}
	if config.Username == "" {
		return nil, fmt.Errorf("VSPHERE_USERNAME is required")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("VSPHERE_PASSWORD is required")
	}

	return config, nil
}
