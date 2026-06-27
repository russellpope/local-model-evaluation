package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfigPrecedence(t *testing.T) {
	// Create a temporary YAML config file with known values.
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

	// Build a viper instance pointing at the temp file.
	v, err := New(cfgPath)
	if err != nil {
		t.Fatalf("New(%q): %v", cfgPath, err)
	}

	// Step 1 — after ReadInConfig with no env or flag overrides we should see
	// the config-file values (plus defaults for unset keys).
	cfg := ToStruct(v)
	assertEqual(t, "url after file-read", "https://from-config/file/sdk", cfg.URL)
	assertEqual(t, "username after file-read", "configFileUser", cfg.Username)
	assertBool(t, "insecure after file-read", true, cfg.Insecure)

	// Step 2 — override via env var. Viper's AutomaticEnv reads the value when
	// Get is called; we simulate this by setting the env and re-reading.
	t.Setenv("VSPHERE_URL", "https://from-env/sdk")
	cfg = ToStruct(v)
	assertEqual(t, "url after env override", "https://from-env/sdk", cfg.URL)

	// Step 3 — override via a flag (BindPFlag). We bind the url flag to viper
	// and set it programmatically. This should beat both file and env.
	v.Set("url", "https://from-flag/sdk") // Set() has highest precedence in viper
	cfg = ToStruct(v)
	assertEqual(t, "url after explicit Set (simulating flag)", "https://from-flag/sdk", cfg.URL)

	// Step 4 — default fallback: unset env, clear file value, ensure default.
	t.Setenv("VSPHERE_URL", "") // clear the env override
	v2 := viper.New()
	v2.SetDefault("url", "default-url")
	v2.SetEnvPrefix("VSPHERE")
	v2.AutomaticEnv()
	_ = v2.ReadInConfig() // no file exists, returns error we ignore

	cfg2 := ToStruct(v2)
	assertEqual(t, "url default fallback", "default-url", cfg2.URL)
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
