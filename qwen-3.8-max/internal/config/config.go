package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

var keys = []string{"url", "username", "password", "insecure", "timeout"}

func NewViper() *viper.Viper {
	v := viper.New()
	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", "60s")
	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()
	return v
}

func BindFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	for _, key := range keys {
		f := flags.Lookup(key)
		if f == nil {
			continue
		}
		if err := v.BindPFlag(key, f); err != nil {
			return fmt.Errorf("bind flag --%s: %w", key, err)
		}
	}
	return nil
}

func Resolve(v *viper.Viper) (*Config, error) {
	rawTimeout := strings.TrimSpace(v.GetString("timeout"))
	timeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse timeout %q: %w (use a Go duration such as 30s or 2m)", rawTimeout, err)
	}
	if timeout <= 0 {
		return nil, errors.New("timeout must be a positive duration")
	}
	return &Config{
		URL:      strings.TrimSpace(v.GetString("url")),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  timeout,
	}, nil
}
