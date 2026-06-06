package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	URL       string        `yaml:"url"`
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
	Insecure  bool          `yaml:"insecure"`
	Timeout   int64         `yaml:"timeout"`
	Config    string        `yaml:"config"`
	VMs       []VMInfo      `yaml:"-"`
}

type VMInfo struct {
	Name     string `yaml:"name"`
	VCPU     int    `yaml:"vcpu"`
	RAM      int64  `yaml:"ram"`
	Storage  int64  `yaml:"storage"`
}

func LoadConfig(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{
		Timeout: 60,
	}

	if cfg.Config != "" {
		data, err := os.ReadFile(cfg.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		err = yaml.Unmarshal(data, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else if cmd.Flags().Changed("url") {
		var err error
		cfg.URL, err = cmd.Flags().GetString("url")
		if err != nil {
			return nil, err
		}
		cfg.Username, err = cmd.Flags().GetString("username")
		if err != nil {
			return nil, err
		}
		cfg.Password, err = cmd.Flags().GetString("password")
		if err != nil {
			return nil, err
		}
		cfg.Insecure, err = cmd.Flags().GetBool("insecure")
		if err != nil {
			return nil, err
		}
		t, err := cmd.Flags().GetInt("timeout")
		if err != nil {
			return nil, err
		}
		cfg.Timeout = int64(t)
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	return cfg, nil
}
