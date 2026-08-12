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
}

func Init(cfgPath string) error {
	viper.SetEnvPrefix("VSPHERE")
	viper.AutomaticEnv()
	viper.SetDefault("insecure", false)
	viper.SetDefault("timeout", "60s")
	if cfgPath != "" {
		viper.SetConfigFile(cfgPath)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME")
	}
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	return nil
}

func Load() (*Config, error) {
	url := viper.GetString("url")
	username := viper.GetString("username")
	password := viper.GetString("password")
	insecure := viper.GetBool("insecure")
	timeoutStr := viper.GetString("timeout")
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}
	return &Config{
		URL:      url,
		Username: username,
		Password: password,
		Insecure: insecure,
		Timeout:  timeout,
	}, nil
}
