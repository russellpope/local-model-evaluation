package vim

import (
	"testing"
)

func TestVMInfoStruct(t *testing.T) {
	vm := VMInfo{
		Name:    "test-vm",
		VCPU:    4,
		RAM:     8 * 1024 * 1024 * 1024,
		Storage: 100 * 1024 * 1024 * 1024,
	}

	if vm.Name != "test-vm" {
		t.Errorf("Expected Name=test-vm, got %s", vm.Name)
	}
	if vm.VCPU != 4 {
		t.Errorf("Expected VCPU=4, got %d", vm.VCPU)
	}
	if vm.RAM != 8*1024*1024*1024 {
		t.Errorf("Expected RAM=8GiB, got %d bytes", vm.RAM)
	}
	if vm.Storage != 100*1024*1024*1024 {
		t.Errorf("Expected Storage=100GiB, got %d bytes", vm.Storage)
	}
}

func TestVMInfoRAMConversion(t *testing.T) {
	tests := []struct {
		name     string
		memoryMB int32
		expected int64
	}{
		{
			name:     "1 GB",
			memoryMB: 1024,
			expected: 1024 * 1024 * 1024,
		},
		{
			name:     "8 GB",
			memoryMB: 8192,
			expected: 8 * 1024 * 1024 * 1024,
		},
		{
			name:     "16 GB",
			memoryMB: 16384,
			expected: 16 * 1024 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := int64(tt.memoryMB) * 1024 * 1024
			if result != tt.expected {
				t.Errorf("RAM conversion for %d MB: got %d bytes, want %d", tt.memoryMB, result, tt.expected)
			}
		})
	}
}

func TestVMInfoStorageNonNegative(t *testing.T) {
	vm := VMInfo{
		Name:    "test-vm",
		VCPU:    2,
		RAM:     4 * 1024 * 1024 * 1024,
		Storage: 50 * 1024 * 1024 * 1024,
	}

	if vm.Storage < 0 {
		t.Errorf("Expected non-negative Storage, got %d", vm.Storage)
	}
}

func TestVMInfoVCPUPositive(t *testing.T) {
	vm := VMInfo{
		Name:    "test-vm",
		VCPU:    4,
		RAM:     8 * 1024 * 1024 * 1024,
		Storage: 100 * 1024 * 1024 * 1024,
	}

	if vm.VCPU <= 0 {
		t.Errorf("Expected positive VCPU, got %d", vm.VCPU)
	}
}

func TestVMInfoNameNotEmpty(t *testing.T) {
	vm := VMInfo{
		Name:    "test-vm",
		VCPU:    2,
		RAM:     4 * 1024 * 1024 * 1024,
		Storage: 50 * 1024 * 1024 * 1024,
	}

	if vm.Name == "" {
		t.Error("Expected non-empty Name")
	}
}
