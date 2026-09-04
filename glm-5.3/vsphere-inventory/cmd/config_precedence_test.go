package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"vsphere-inventory/internal/config"
)

const testConfigYAML = `
url: https://file-vc.lab/sdk
username: file-user
password: file-pass
insecure: true
timeout: 45s
`

// runProbeCommand builds a real cobra command with the production config
// wiring (registerConfigFlags + readConfigFile + bindConfig + config.Load),
// executes it with args, and returns the resolved configuration.
func runProbeCommand(t *testing.T, configYAML string, args []string) (*config.Config, error) {
	t.Helper()

	var cfgFile string
	if configYAML != "" {
		dir := t.TempDir()
		cfgFile = filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(cfgFile, []byte(configYAML), 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--config", cfgFile)
	}

	var cfg *config.Config
	probe := &cobra.Command{
		Use: "probe",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.New()
			bindConfig(cmd, v)
			path, _ := cmd.Flags().GetString("config")
			if err := readConfigFile(v, path); err != nil {
				return err
			}
			resolved, err := config.Load(v)
			cfg = resolved
			return err
		},
	}
	registerConfigFlags(probe)
	probe.SetArgs(args)
	err := probe.Execute()
	return cfg, err
}

// TestConfigPrecedence walks the full resolution chain and asserts
// flag > env > config file > default for every field.
func TestConfigPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		configYAML string
		env        map[string]string
		args       []string

		wantURL      string
		wantUsername string
		wantInsecure bool
		wantTimeout  time.Duration
	}{
		{
			name:         "defaults only",
			args:         []string{},
			wantURL:      "",
			wantUsername: "",
			wantInsecure: false,
			wantTimeout:  60 * time.Second,
		},
		{
			name:         "config file beats default",
			configYAML:   testConfigYAML,
			args:         []string{},
			wantURL:      "https://file-vc.lab/sdk",
			wantUsername: "file-user",
			wantInsecure: true,
			wantTimeout:  45 * time.Second,
		},
		{
			name:       "env beats config file and default",
			configYAML: testConfigYAML,
			env: map[string]string{
				"VSPHERE_URL":      "https://env-vc.lab/sdk",
				"VSPHERE_USERNAME": "env-user",
				"VSPHERE_INSECURE": "false",
				"VSPHERE_TIMEOUT":  "30s",
			},
			args:         []string{},
			wantURL:      "https://env-vc.lab/sdk",
			wantUsername: "env-user",
			wantInsecure: false,
			wantTimeout:  30 * time.Second,
		},
		{
			name:       "flag beats env, config file and default",
			configYAML: testConfigYAML,
			env: map[string]string{
				"VSPHERE_URL":      "https://env-vc.lab/sdk",
				"VSPHERE_USERNAME": "env-user",
				"VSPHERE_INSECURE": "false",
				"VSPHERE_TIMEOUT":  "30s",
			},
			args: []string{
				"--url", "https://flag-vc.lab/sdk",
				"--username", "flag-user",
				"--insecure",
				"--timeout", "15s",
			},
			wantURL:      "https://flag-vc.lab/sdk",
			wantUsername: "flag-user",
			wantInsecure: true,
			wantTimeout:  15 * time.Second,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, val := range tc.env {
				t.Setenv(k, val)
			}

			cfg, err := runProbeCommand(t, tc.configYAML, tc.args)
			if err != nil {
				if tc.wantURL == "" {
					return // expected: no URL configured
				}
				t.Fatalf("resolve config: %v", err)
			}
			if tc.wantURL == "" {
				t.Fatal("expected error for missing URL")
			}

			if cfg.URL != tc.wantURL {
				t.Errorf("url = %q, want %q", cfg.URL, tc.wantURL)
			}
			if cfg.Username != tc.wantUsername {
				t.Errorf("username = %q, want %q", cfg.Username, tc.wantUsername)
			}
			if cfg.Insecure != tc.wantInsecure {
				t.Errorf("insecure = %v, want %v", cfg.Insecure, tc.wantInsecure)
			}
			if cfg.Timeout != tc.wantTimeout {
				t.Errorf("timeout = %s, want %s", cfg.Timeout, tc.wantTimeout)
			}
		})
	}
}
