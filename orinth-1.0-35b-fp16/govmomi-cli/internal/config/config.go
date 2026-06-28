package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config holds the resolved connection parameters for a vCenter session.
type Config struct {
	URL      string // e.g. https://vc.lab/sdk
	Username string
	Password string
	Insecure bool          // skip TLS verification
	Timeout  time.Duration // overall operation timeout, default 60s
}

// New returns a viper instance configured with defaults, env prefix VSPHERE_,
// and an optional YAML config file at cfgPath. Defaults are set first so that
// the precedence order (flag > env > file > default) is preserved by viper's
// resolution rules when flags are later bound via BindPFlag.
func New(cfgPath string) (*viper.Viper, error) {
	v := viper.New()

	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", 60*time.Second)

	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()

	if cfgPath != "" {
		if _, err := os.Stat(cfgPath); err != nil {
			return v, fmt.Errorf("config file %q: %w", cfgPath, err)
		}
		v.SetConfigFile(cfgPath)
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			v.AddConfigPath(home)
		}
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// ReadInConfig returns an error only when a configured file exists but is unreadable.
	// An absent config file is fine — we just use defaults + env.
	_ = v.ReadInConfig()

	return v, nil
}

// ToStruct extracts the typed Config from a populated viper instance.
func ToStruct(v *viper.Viper) Config {
	return Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
	}
}
