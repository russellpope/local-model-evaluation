package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  func()
		flagValue string
		envValue  string
		assert    func(t *testing.T, cfg *Config)
	}{
		{
			name:      "flag overrides env",
			setupEnv:  func() { os.Setenv("VSPHERE_URL", "env://vc") },
			flagValue: "flag://vc",
			assert: func(t *testing.T, cfg *Config) {
				if cfg.URL != "flag://vc" {
					t.Errorf("expected URL to be 'flag://vc', got %q", cfg.URL)
				}
			},
		},
		{
			name:      "env overrides default",
			setupEnv:  func() { os.Setenv("VSPHERE_URL", "env://vc") },
			assert: func(t *testing.T, cfg *Config) {
				if cfg.URL != "env://vc" {
					t.Errorf("expected URL to be 'env://vc', got %q", cfg.URL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			
			v = nil
			if err := Init(); err != nil {
				t.Fatalf("failed to init: %v", err)
			}
			
			if tt.flagValue != "" {
				v.Set("url", tt.flagValue)
			}
			
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}
			
			if tt.assert != nil {
				tt.assert(t, cfg)
			}
			
			os.Unsetenv("VSPHERE_URL")
		})
	}
}

func TestDefaultTimeout(t *testing.T) {
	v = nil
	if err := Init(); err != nil {
		t.Fatalf("failed to init: %v", err)
	}
	
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	
	expected := 60 * time.Second
	if cfg.Timeout != expected {
		t.Errorf("expected timeout %v, got %v", expected, cfg.Timeout)
	}
}

func TestDefaultURL(t *testing.T) {
	v = nil
	if err := Init(); err != nil {
		t.Fatalf("failed to init: %v", err)
	}
	
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	
	expected := "https://localhost/sdk"
	if cfg.URL != expected {
		t.Errorf("expected URL %q, got %q", expected, cfg.URL)
	}
}

func TestDefaultInsecure(t *testing.T) {
	v = nil
	if err := Init(); err != nil {
		t.Fatalf("failed to init: %v", err)
	}
	
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	
	if cfg.Insecure {
		t.Errorf("expected insecure to be false, got true")
	}
}

func init() {
	_ = os.RemoveAll
}
