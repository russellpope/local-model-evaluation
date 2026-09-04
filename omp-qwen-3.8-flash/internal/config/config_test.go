package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newTestViper mirrors the cmd wiring: shared cobra flags bound into viper.
func newTestViper() (*viper.Viper, *cobra.Command) {
	cmd := &cobra.Command{Use: "test"}
	fs := cmd.Flags()
	fs.String("url", "", "")
	fs.String("username", "", "")
	fs.String("password", "", "")
	fs.Bool("insecure", false, "")
	fs.Duration("timeout", 0, "")
	fs.String("config", "", "")
	v := NewViper()
	for _, key := range []string{"url", "username", "password", "insecure", "timeout", "config"} {
		_ = v.BindPFlag(key, fs.Lookup(key))
	}
	return v, cmd
}

func writeFile(t *testing.T, url, username string, timeout time.Duration, insecure bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "url: " + url + "\nusername: " + username + "\ntimeout: " + timeout.String() + "\ninsecure: " + boolStr(insecure) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestPrecedence verifies flag > env > file > default for every layer.
func TestPrecedence(t *testing.T) {
	filePath := writeFile(t, "https://file.lab/sdk", "fileuser", 20*time.Second, true)

	// --- default only ---
	t.Run("default", func(t *testing.T) {
		v, _ := newTestViper()
		cfg, err := Resolve(v)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("default timeout = %v, want 60s", cfg.Timeout)
		}
		if cfg.Insecure {
			t.Error("default insecure = true, want false")
		}
	})

	// --- file beats default ---
	t.Run("file", func(t *testing.T) {
		v, cmd := newTestViper()
		if err := cmd.Flags().Set("config", filePath); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(v)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://file.lab/sdk" {
			t.Errorf("file url = %q, want https://file.lab/sdk", cfg.URL)
		}
		if cfg.Timeout != 20*time.Second {
			t.Errorf("file timeout = %v, want 20s", cfg.Timeout)
		}
		if !cfg.Insecure {
			t.Error("file insecure = false, want true")
		}
	})

	// --- env beats file ---
	t.Run("env", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env.lab/sdk")
		t.Setenv("VSPHERE_TIMEOUT", "30s")
		t.Setenv("VSPHERE_INSECURE", "false")
		v, cmd := newTestViper()
		if err := cmd.Flags().Set("config", filePath); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(v)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://env.lab/sdk" {
			t.Errorf("env url = %q, want https://env.lab/sdk", cfg.URL)
		}
		if cfg.Timeout != 30*time.Second {
			t.Errorf("env timeout = %v, want 30s", cfg.Timeout)
		}
		if cfg.Insecure {
			t.Error("env insecure = true, want false (env overrides file's true)")
		}
	})

	// --- flag beats env ---
	t.Run("flag", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env.lab/sdk")
		t.Setenv("VSPHERE_USERNAME", "envuser")
		t.Setenv("VSPHERE_TIMEOUT", "30s")
		v, cmd := newTestViper()
		if err := cmd.Flags().Set("config", filePath); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("url", "https://flag.lab/sdk"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("username", "flaguser"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("timeout", "5s"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(v)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://flag.lab/sdk" {
			t.Errorf("flag url = %q, want https://flag.lab/sdk", cfg.URL)
		}
		if cfg.Username != "flaguser" {
			t.Errorf("flag username = %q, want flaguser", cfg.Username)
		}
		if cfg.Timeout != 5*time.Second {
			t.Errorf("flag timeout = %v, want 5s", cfg.Timeout)
		}
	})

	// --- precedence chain: unset flag falls through to env, unset env to file ---
	t.Run("partial chain", func(t *testing.T) {
		t.Setenv("VSPHERE_PASSWORD", "envpass")
		v, cmd := newTestViper()
		if err := cmd.Flags().Set("config", filePath); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("url", "https://flag.lab/sdk"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(v)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://flag.lab/sdk" { // flag layer
			t.Errorf("url = %q, want flag value", cfg.URL)
		}
		if cfg.Password != "envpass" { // env layer, flag unset
			t.Errorf("password = %q, want env value envpass", cfg.Password)
		}
		if cfg.Username != "fileuser" { // file layer, flag+env unset
			t.Errorf("username = %q, want file value fileuser", cfg.Username)
		}
		if cfg.Timeout != 20*time.Second { // file layer
			t.Errorf("timeout = %v, want file value 20s", cfg.Timeout)
		}
	})
}

func TestResolveInvalidConfigFile(t *testing.T) {
	v, cmd := newTestViper()
	if err := cmd.Flags().Set("config", filepath.Join(t.TempDir(), "missing.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(v); err == nil {
		t.Fatal("Resolve with missing config file: want error, got nil")
	}
}

func TestResolveInvalidTimeout(t *testing.T) {
	t.Setenv("VSPHERE_TIMEOUT", "not-a-duration")
	v, _ := newTestViper()
	if _, err := Resolve(v); err == nil {
		t.Fatal("Resolve with bad timeout: want error, got nil")
	}
}
