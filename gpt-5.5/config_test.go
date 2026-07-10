package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPrecedenceFlagEnvFileDefault(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://env.example/sdk")

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("url: https://file.example/sdk\ninsecure: true\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cmd := NewRootCommand()
	if err := cmd.ParseFlags([]string{"--config", configFile, "--url", "https://flag.example/sdk", "vms"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	cfg, _, err := LoadConfigForCommand(cmd)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.URL != "https://flag.example/sdk" {
		t.Fatalf("flag should override env and file, got %q", cfg.URL)
	}
	if !cfg.Insecure {
		t.Fatalf("config file value should override default insecure=false")
	}
	if cfg.Timeout.String() != "1m0s" {
		t.Fatalf("default timeout = %s, want 1m0s", cfg.Timeout)
	}

	cmd = NewRootCommand()
	if err := cmd.ParseFlags([]string{"--config", configFile, "vms"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cfg, _, err = LoadConfigForCommand(cmd)
	if err != nil {
		t.Fatalf("load config without flag url: %v", err)
	}
	if cfg.URL != "https://env.example/sdk" {
		t.Fatalf("env should override file, got %q", cfg.URL)
	}

	t.Setenv("VSPHERE_URL", "")
	cmd = NewRootCommand()
	if err := cmd.ParseFlags([]string{"--config", configFile, "vms"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cfg, _, err = LoadConfigForCommand(cmd)
	if err != nil {
		t.Fatalf("load config without env url: %v", err)
	}
	if cfg.URL != "https://file.example/sdk" {
		t.Fatalf("file should override default, got %q", cfg.URL)
	}
}
