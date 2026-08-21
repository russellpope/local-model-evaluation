package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// writeConfig writes a minimal YAML config file and returns its path.
func writeConfig(t *testing.T, url string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "url: " + url + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	return path
}

// buildFlags returns a flag set with the config flag pointing at path (or "" to
// omit the file) and optionally sets url to mark the --url flag as changed.
func buildFlags(t *testing.T, cfgPath, url string, setURL bool) *pflag.FlagSet {
	t.Helper()
	f := Flags()
	if cfgPath != "" {
		if err := f.Set("config", cfgPath); err != nil {
			t.Fatalf("setting config flag: %v", err)
		}
	}
	if setURL {
		if err := f.Set("url", url); err != nil {
			t.Fatalf("setting url flag: %v", err)
		}
	}
	return f
}

func TestConfigPrecedence(t *testing.T) {
	const (
		defaultURL = ""
		fileURL    = "https://file.example/sdk"
		envURL     = "https://env.example/sdk"
		flagURL    = "https://flag.example/sdk"
	)

	t.Run("flag beats env beats file beats default", func(t *testing.T) {
		cfgPath := writeConfig(t, fileURL)

		t.Setenv("VSPHERE_URL", envURL)

		// With the flag set, it wins over everything.
		cfg, err := FromFlags(buildFlags(t, cfgPath, flagURL, true))
		if err != nil {
			t.Fatalf("FromFlags: %v", err)
		}
		if cfg.URL != flagURL {
			t.Errorf("flag precedence: got %q, want %q", cfg.URL, flagURL)
		}

		// Without the flag, env beats the file.
		cfg, err = FromFlags(buildFlags(t, cfgPath, "", false))
		if err != nil {
			t.Fatalf("FromFlags: %v", err)
		}
		if cfg.URL != envURL {
			t.Errorf("env precedence: got %q, want %q", cfg.URL, envURL)
		}
	})

	t.Run("file beats default when no env or flag", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "")

		cfgPath := writeConfig(t, fileURL)
		cfg, err := FromFlags(buildFlags(t, cfgPath, "", false))
		if err != nil {
			t.Fatalf("FromFlags: %v", err)
		}
		if cfg.URL != fileURL {
			t.Errorf("file precedence: got %q, want %q", cfg.URL, fileURL)
		}
	})

	t.Run("default when nothing set", func(t *testing.T) {
		os.Unsetenv("VSPHERE_URL")

		cfg, err := FromFlags(buildFlags(t, "", "", false))
		if err != nil {
			t.Fatalf("FromFlags: %v", err)
		}
		if cfg.URL != defaultURL {
			t.Errorf("default: got %q, want %q", cfg.URL, defaultURL)
		}
		if cfg.Timeout != DefaultTimeout {
			t.Errorf("default timeout: got %v, want %v", cfg.Timeout, DefaultTimeout)
		}
	})
}

func TestConfigTimeoutParsing(t *testing.T) {
	t.Setenv("VSPHERE_TIMEOUT", "1500ms")
	cfg, err := FromFlags(buildFlags(t, "", "", false))
	if err != nil {
		t.Fatalf("FromFlags: %v", err)
	}
	if cfg.Timeout != 1500*time.Millisecond {
		t.Errorf("timeout: got %v, want %v", cfg.Timeout, 1500*time.Millisecond)
	}
}
