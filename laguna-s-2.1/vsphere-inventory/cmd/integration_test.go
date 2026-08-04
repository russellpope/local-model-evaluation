package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestConfigPrecedenceFlagOverEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `url: https://file.lab/sdk
username: fileuser
password: filepass
insecure: false
timeout: 10s
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	v := viper.New()
	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()

	os.Setenv("VSPHERE_URL", "https://env.lab/sdk")
	os.Setenv("VSPHERE_USERNAME", "envuser")
	os.Setenv("VSPHERE_PASSWORD", "envpass")
	defer os.Unsetenv("VSPHERE_URL")
	defer os.Unsetenv("VSPHERE_USERNAME")
	defer os.Unsetenv("VSPHERE_PASSWORD")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("url", "", "vCenter URL")
	fs.String("username", "", "vCenter username")
	fs.String("password", "", "vCenter password")
	v.BindPFlag("url", fs.Lookup("url"))
	v.BindPFlag("username", fs.Lookup("username"))
	v.BindPFlag("password", fs.Lookup("password"))

	if err := fs.Parse([]string{
		"--url", "https://flag.lab/sdk",
		"--username", "flaguser",
		"--password", "flagpass",
	}); err != nil {
		t.Fatalf("parsing flags: %v", err)
	}

	v.SetConfigFile(configFile)
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("reading config file: %v", err)
	}

	c := config.NewWithViper(v)
	if err := c.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.URL != "https://flag.lab/sdk" {
		t.Errorf("URL = %q, want %q (flag should win over env and file)", c.URL, "https://flag.lab/sdk")
	}
	if c.Username != "flaguser" {
		t.Errorf("Username = %q, want %q (flag should win over env and file)", c.Username, "flaguser")
	}
	if c.Password != "flagpass" {
		t.Errorf("Password = %q, want %q (flag should win over env and file)", c.Password, "flagpass")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
