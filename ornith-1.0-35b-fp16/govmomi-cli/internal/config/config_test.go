package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestConfigPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
url: https://from-config/file/sdk
username: configFileUser
password: configFilePass
insecure: true
timeout: 30s
`), 0644); err != nil {
		t.Fatalf("writing temp config file: %v", err)
	}

	v, err := New(cfgPath)
	if err != nil {
		t.Fatalf("New(%q): %v", cfgPath, err)
	}

	cfg := ToStruct(v)
	assertEqual(t, "url after file-read", "https://from-config/file/sdk", cfg.URL)
	assertEqual(t, "username after file-read", "configFileUser", cfg.Username)
	assertBool(t, "insecure after file-read", true, cfg.Insecure)

	t.Setenv("VSPHERE_URL", "https://from-env/sdk")
	cfg = ToStruct(v)
	assertEqual(t, "url after env override", "https://from-env/sdk", cfg.URL)

	v.Set("url", "https://from-flag/sdk")
	cfg = ToStruct(v)
	assertEqual(t, "url after explicit Set (simulating flag)", "https://from-flag/sdk", cfg.URL)

	t.Setenv("VSPHERE_URL", "")
	v2 := viper.New()
	v2.SetDefault("url", "default-url")
	v2.SetEnvPrefix("VSPHERE")
	v2.AutomaticEnv()
	_ = v2.ReadInConfig()

	cfg2 := ToStruct(v2)
	assertEqual(t, "url default fallback", "default-url", cfg2.URL)
}

// TestBindPFlagEndToEnd mirrors the production flow in cmd/bindFlags: it builds a
// viper from config.New(file), attaches a real *pflag.FlagSet with a known value
// via BindPFlag, and asserts that the bound flag overrides both env and file
// values. This is the only correct way to verify cobra-flag precedence end-to-end;
// v.Set("url", ...) bypasses pflag entirely and cannot catch regressions in
// bindFlags().
func TestBindPFlagEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
url: https://from-config/sdk
username: configFileUser
password: configFilePass
insecure: true
timeout: 30s
`), 0644); err != nil {
		t.Fatalf("writing temp config file: %v", err)
	}

	v, err := New(cfgPath)
	if err != nil {
		t.Fatalf("New(%q): %v", cfgPath, err)
	}

	t.Setenv("VSPHERE_URL", "https://from-env/sdk")

	flagSet := pflag.NewFlagSet("root", pflag.ContinueOnError)
	_ = flagSet.String("url", "", "vCenter URL or host")
	_ = v.BindPFlag("url", flagSet.Lookup("url"))

	if err := flagSet.Parse([]string{"--url", "https://from-flag/sdk"}); err != nil {
		t.Fatalf("flagSet.Parse: %v", err)
	}

	cfg := ToStruct(v)
	assertEqual(t, "url after BindPFlag override", "https://from-flag/sdk", cfg.URL)
	assertEqual(t, "username from file (unchanged by url flag)", "configFileUser", cfg.Username)
}

func TestConfigDefaultTimeout(t *testing.T) {
	v, err := New("") // empty path: no config file; defaults only
	if err != nil {
		t.Fatalf("New(\"\"): %v", err)
	}
	cfg := ToStruct(v)
	if cfg.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", cfg.Timeout)
	}
}

func assertEqual(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", label, got, want)
	}
}

func assertBool(t *testing.T, label string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}
