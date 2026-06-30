package config

import (
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

func Resolve(v *viper.Viper) (*Config, error) {
	timeoutStr := v.GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, err
	}

	return &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  timeout,
	}, nil
}
