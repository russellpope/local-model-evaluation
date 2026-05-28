package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestDefaultValues(t *testing.T) {
	cmd := &cobra.Command{}
	cfg := NewConfig()

	cfg.BindFlags(cmd)

	result, err := cfg.Load()
	if err != nil {
		t.Fatal(err)
	}

	if result.URL != "https://localhost/sdk" {
		t.Errorf("Expected default URL, got %s", result.URL)
	}
	if result.Username != "" {
		t.Errorf("Expected empty default username, got %s", result.Username)
	}
	if result.Password != "" {
		t.Errorf("Expected empty default password, got %s", result.Password)
	}
	if result.Insecure {
		t.Errorf("Expected default insecure=false, got true")
	}
	if result.Timeout != 60*time.Second {
		t.Errorf("Expected default timeout=60s, got %v", result.Timeout)
	}
}

func TestConfigFileOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(`
url: https://file.example.com/sdk
username: fileuser
password: filepass
insecure: true
timeout: 30s
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cfg := NewConfig()

	cfg.BindFlags(cmd)

	cfg.Cfg.Config = configFile

	result, err := cfg.Load()
	if err != nil {
		t.Fatal(err)
	}

	if result.URL != "https://file.example.com/sdk" {
		t.Errorf("Expected URL from file, got %s", result.URL)
	}
	if result.Username != "fileuser" {
		t.Errorf("Expected username from file, got %s", result.Username)
	}
	if result.Password != "filepass" {
		t.Errorf("Expected password from file, got %s", result.Password)
	}
	if !result.Insecure {
		t.Errorf("Expected insecure from file, got false")
	}
	if result.Timeout != 30*time.Second {
		t.Errorf("Expected timeout from file, got %v", result.Timeout)
	}
}

func TestTimeoutParsing(t *testing.T) {
	cmd := &cobra.Command{}
	cfg := NewConfig()

	cfg.BindFlags(cmd)

	t.Setenv("VSPHERE_TIMEOUT", "120s")

	result, err := cfg.Load()
	if err != nil {
		t.Fatal(err)
	}

	if result.Timeout != 120*time.Second {
		t.Errorf("Expected timeout=120s, got %v", result.Timeout)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(`
url: https://file.example.com/sdk
username: fileuser
password: filepass
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cfg := NewConfig()

	cfg.BindFlags(cmd)

	t.Setenv("VSPHERE_URL", "https://env.example.com/sdk")
	t.Setenv("VSPHERE_USERNAME", "envuser")

	cfg.Cfg.Config = configFile

	result, err := cfg.Load()
	if err != nil {
		t.Fatal(err)
	}

	if result.URL != "https://env.example.com/sdk" {
		t.Errorf("Expected URL from env (overrides file), got %s", result.URL)
	}
	if result.Username != "envuser" {
		t.Errorf("Expected username from env (overrides file), got %s", result.Username)
	}
	if result.Password != "filepass" {
		t.Errorf("Expected password from file, got %s", result.Password)
	}
}

func TestInsecureDefaultFalse(t *testing.T) {
	cmd := &cobra.Command{}
	cfg := NewConfig()

	cfg.BindFlags(cmd)

	result, err := cfg.Load()
	if err != nil {
		t.Fatal(err)
	}

	if result.Insecure {
		t.Errorf("Expected default insecure=false")
	}
}
