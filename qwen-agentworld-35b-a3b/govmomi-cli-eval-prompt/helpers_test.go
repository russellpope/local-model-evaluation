package main

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero bytes", 0, "0.0 MiB"},
		{"1 MiB", 1024 * 1024, "1.0 MiB"},
		{"1 GiB", 1024 * 1024 * 1024, "1.0 GiB"},
		{"1.5 GiB", int64(1.5*1024*1024*1024), "1.5 GiB"},
		{"2 GiB", 2 * 1024 * 1024 * 1024, "2.0 GiB"},
		{"1 TiB", 1024 * 1024 * 1024 * 1024, "1.0 TiB"},
		{"2.5 TiB", int64(2.5 * 1024 * 1024 * 1024 * 1024), "2.5 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestConfigPrecedence(t *testing.T) {
	// Reset viper for each test
	viper.Reset()

	viper.SetDefault("timeout", 30*time.Second)

	// Set config file value
	viper.SetConfigType("yaml")
	viper.ReadConfig(strings.NewReader("timeout: 45s\nurl: https://config.url/sdk"))

	// Set env var
	t.Setenv("VSPHERE_TIMEOUT", "90s")
	t.Setenv("VSPHERE_URL", "https://env.url/sdk")

	// Set flag value (simulate via viper set)
	viper.Set("timeout", 120*time.Second)
	viper.Set("url", "https://flag.url/sdk")

	// Verify flag takes precedence
	if viper.GetDuration("timeout") != 120*time.Second {
		t.Errorf("Expected timeout 120s, got %v", viper.GetDuration("timeout"))
	}
	if viper.GetString("url") != "https://flag.url/sdk" {
		t.Errorf("Expected url https://flag.url/sdk, got %s", viper.GetString("url"))
	}
}

func TestIsNVMeDevice(t *testing.T) {
	tests := []struct {
		device   string
		expected bool
	}{
		{"nvme0n1", true},
		{"nvmexxxx", true},
		{"ns0", true},
		{"mpx.vmhba32", false},
		{"naa.12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			result := isNVMeDevice(tt.device)
			if result != tt.expected {
				t.Errorf("isNVMeDevice(%q) = %v, want %v", tt.device, result, tt.expected)
			}
		})
	}
}

func TestIsISCSIDevice(t *testing.T) {
	tests := []struct {
		device   string
		expected bool
	}{
		{"naa.5000c50012345678", true},
		{"iqn.2001-04.com.example:storage.lun1", true},
		{"eui.1234567890abcdef", true},
		{"tpgt 1", true},
		{"iscsi", true},
		{"vmhba33", true},
		{"nvme0n1", false},
		{"mpx.vmhba32", false},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			result := isISCSIDevice(tt.device)
			if result != tt.expected {
				t.Errorf("isISCSIDevice(%q) = %v, want %v", tt.device, result, tt.expected)
			}
		})
	}
}

func TestIsFCDevice(t *testing.T) {
	tests := []struct {
		device   string
		expected bool
	}{
		{"mpx.vmhba32:C0:T0:L0", true},
		{"t10.ATA_____WDC_WD1003FZEX_________________07N0Y0", true},
		{"vmhba0", true},
		{"vmhba15", true},
		{"vmhba32", true},
		{"nvme0n1", false},
		{"naa.5000c50012345678", false},
		{"iqn.2001-04.com.example:storage.lun1", false},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			result := isFCDevice(tt.device)
			if result != tt.expected {
				t.Errorf("isFCDevice(%q) = %v, want %v", tt.device, result, tt.expected)
			}
		})
	}
}

func TestClassifyStorageFromDevice(t *testing.T) {
	tests := []struct {
		device   string
		expected string
	}{
		{"nvme0n1", "NVMe"},
		{"nvmexxxx", "NVMe"},
		{"naa.5000c50012345678", "iSCSI"},
		{"iqn.2001-04.com.example:storage.lun1", "iSCSI"},
		{"mpx.vmhba32:C0:T0:L0", "FC"},
		{"t10.ATA_____WDC_WD1003FZEX_________________07N0Y0", "FC"},
		{"unknown_device", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			result := classifyStorageFromDevice(tt.device)
			if result != tt.expected {
				t.Errorf("classifyStorageFromDevice(%q) = %q, want %q", tt.device, result, tt.expected)
			}
		})
	}
}
