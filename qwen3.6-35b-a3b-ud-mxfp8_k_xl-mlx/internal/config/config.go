package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds the vSphere connection configuration.
type Config struct {
	URL      string        `mapstructure:"url"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	Insecure bool          `mapstructure:"insecure"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Config   string        // path to config file (not in viper map)
}

// DefaultConfig returns a Config with built-in defaults.
func DefaultConfig() *Config {
	return &Config{
		URL:      "",
		Username: "",
		Password: "",
		Insecure: false,
		Timeout:  60 * time.Second,
		Config:   "",
	}
}

// BindFlags binds command-line flags to viper keys.
func BindFlags(flags *pflag.FlagSet, v *viper.Viper) error {
	flags.String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk")
	flags.String("username", "", "vCenter username")
	flags.String("password", "", "vCenter password")
	flags.Bool("insecure", false, "skip TLS verification")
	flags.Duration("timeout", 60*time.Second, "overall operation timeout")
	flags.String("config", "", "path to a YAML config file")

	if v != nil {
		if err := v.BindPFlags(flags); err != nil {
			return fmt.Errorf("bind flags: %w", err)
		}

		// Bind environment variables with prefix VSPHERE_
		v.AutomaticEnv()
		v.SetEnvPrefix("VSPHERE")
		v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

		keys := []string{"url", "username", "password", "insecure", "timeout"}
		for _, k := range keys {
			if err := v.BindEnv(k, "VSPHERE_"+strings.ToUpper(strings.ReplaceAll(k, "-", "_"))); err != nil {
				return fmt.Errorf("bind env %s: %w", k, err)
			}
		}
	}

	return nil
}

// Load reads configuration from the config file (if specified), applies env var and flag overrides.
func Load(flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if flags == nil {
		flags = pflag.NewFlagSet("default", pflag.ContinueOnError)
	}

	if err := BindFlags(flags, v); err != nil {
		return nil, err
	}

	// Read config file if specified via flag
	configFile, _ := flags.GetString("config")
	if configFile != "" {
		v.SetConfigFile(configFile)
		if _, err := os.Stat(configFile); err != nil {
			return nil, fmt.Errorf("config file %s does not exist: %w", configFile, err)
		}
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	c := DefaultConfig()

	// Viper precedence: flag > env > file > default
	// We need to read from viper which already has the precedence resolved
	if flags != nil {
		if f := flags.Lookup("url"); f != nil && f.Changed {
			c.URL = v.GetString("url")
		} else if envURL := os.Getenv("VSPHERE_URL"); envURL != "" {
			c.URL = envURL
		} else {
			c.URL = v.GetString("url")
		}

		if f := flags.Lookup("username"); f != nil && f.Changed {
			c.Username = v.GetString("username")
		} else if envUser := os.Getenv("VSPHERE_USERNAME"); envUser != "" {
			c.Username = envUser
		} else {
			c.Username = v.GetString("username")
		}

		if f := flags.Lookup("password"); f != nil && f.Changed {
			c.Password = v.GetString("password")
		} else if envPass := os.Getenv("VSPHERE_PASSWORD"); envPass != "" {
			c.Password = envPass
		} else {
			c.Password = v.GetString("password")
		}

		if f := flags.Lookup("insecure"); f != nil && f.Changed {
			c.Insecure = v.GetBool("insecure")
		} else if envInsec := os.Getenv("VSPHERE_INSECURE"); envInsec != "" {
			c.Insecure = envInsec == "true"
		} else {
			c.Insecure = v.GetBool("insecure")
		}

		if f := flags.Lookup("timeout"); f != nil && f.Changed {
			c.Timeout = v.GetDuration("timeout")
		} else if envTimeout := os.Getenv("VSPHERE_TIMEOUT"); envTimeout != "" {
			if d, err := time.ParseDuration(envTimeout); err == nil {
				c.Timeout = d
			}
		} else {
			c.Timeout = v.GetDuration("timeout")
		}

		if f := flags.Lookup("config"); f != nil && f.Changed {
			c.Config, _ = flags.GetString("config")
		}
	} else {
		c.URL = v.GetString("url")
		c.Username = v.GetString("username")
		c.Password = v.GetString("password")
		c.Insecure = v.GetBool("insecure")
		c.Timeout = v.GetDuration("timeout")
	}

	return c, nil
}
