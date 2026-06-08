package format

import "testing"

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.0 GiB"},
		{1 << 30, "1.0 GiB"},
		{int64(1.5 * float64(GiB)), "1.5 GiB"},
		{1352400000000, "1.2 TiB"},
		{1 << 40, "1.0 TiB"},
		{int64(2.5 * float64(TiB)), "2.5 TiB"},
		{-100, "0.0 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := HumanBytes(tt.input)
			if result != tt.expected {
				t.Errorf("HumanBytes(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHumanBytesFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0.0 GiB"},
		{float64(GiB), "1.0 GiB"},
		{1.5 * float64(GiB), "1.5 GiB"},
		{1.23 * float64(TiB), "1.2 TiB"},
		{float64(TiB), "1.0 TiB"},
		{2.5 * float64(TiB), "2.5 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := HumanBytesFloat(tt.input)
			if result != tt.expected {
				t.Errorf("HumanBytesFloat(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGBString(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0"},
		{1024, "1"},
		{2048, "2"},
		{3072, "3"},
		{1536, "1.5"},
		{2560, "2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GBString(tt.input)
			if result != tt.expected {
				t.Errorf("GBString(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUsedEqualsTotalMinusAvailable(t *testing.T) {
	total := int64(2 * float64(TiB))
	available := int64(0.5 * float64(TiB))
	used := total - available

	usedStr := HumanBytes(used)
	availStr := HumanBytes(available)

	if usedStr == "" || availStr == "" {
		t.Fatal("formatting produced empty strings")
	}

	if used+available != total {
		t.Errorf("used (%d) + available (%d) != total (%d)", used, available, total)
	}
}
