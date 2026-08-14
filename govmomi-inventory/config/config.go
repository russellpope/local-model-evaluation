package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	Config   string
}

func NewViper() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()
	v.SetTypeByDefaultValue(true)
	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", 60*time.Second)
	v.SetDefault("config", "")
	return v
}

func BindFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	urlVal := v.GetString("url")
	usernameVal := v.GetString("username")
	passwordVal := v.GetString("password")
	insecureVal := v.GetBool("insecure")
	timeoutVal := v.GetDuration("timeout")
	configVal := v.GetString("config")

	flags.StringVarP(&urlVal, "url", "u", urlVal, "vCenter URL (e.g. https://vc/lab/sdk)")
	flags.StringVarP(&usernameVal, "username", "U", usernameVal, "vCenter username")
	flags.StringVarP(&passwordVal, "password", "P", passwordVal, "vCenter password")
	flags.BoolVarP(&insecureVal, "insecure", "k", insecureVal, "skip TLS verification")
	flags.DurationVarP(&timeoutVal, "timeout", "t", timeoutVal, "operation timeout")
	flags.StringVarP(&configVal, "config", "c", configVal, "config file path")

	if err := v.BindPFlags(flags); err != nil {
		return fmt.Errorf("bind pflags: %w", err)
	}

	v.Set("url", urlVal)
	v.Set("username", usernameVal)
	v.Set("password", passwordVal)
	v.Set("insecure", insecureVal)
	v.Set("timeout", timeoutVal)
	v.Set("config", configVal)

	return nil
}

func LoadConfig(v *viper.Viper) *Config {
	cfg := &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
		Config:   v.GetString("config"),
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return cfg
}

func ReadConfigFile(v *viper.Viper, path string) error {
	v.SetConfigType("yaml")
	v.SetConfigFile(path)
	return v.ReadInConfig()
}

func WriteTestConfig(content string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "govmomi-inventory-test")
	if err != nil {
		return "", err
	}
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
