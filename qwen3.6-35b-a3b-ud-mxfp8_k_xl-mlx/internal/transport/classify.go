package transport

// Classify determines the storage transport protocol from a disk device identifier string.
// It recognizes patterns for FC, iSCSI, NVMe, and returns "unknown" if the pattern is not recognized.
func Classify(diskDevice string) string {
	if diskDevice == "" {
		return "unknown"
	}

	deviceUpper := deviceDiskName(diskDevice)

	switch {
	case startsWith(deviceUpper, "NAA:"):
		return classifyNAADevice(diskDevice)
	case startsWith(deviceUpper, "T10:"):
		return classifyT10Device(diskDevice)
	case startsWith(deviceUpper, "VMHBA"):
		return classifyVMHBADevice(diskDevice)
	case startsWith(deviceUpper, "EUI:"):
		return "NVMe"
	// Also handle dot-delimited govmomi canonical names
	case startsWith(deviceUpper, "NAA."):
		return classifyNAADevice(diskDevice)
	case startsWith(deviceUpper, "T10."):
		return classifyT10Device(diskDevice)
	case startsWith(deviceUpper, "EUI."):
		return "NVMe"
	default:
		return "unknown"
	}
}

func deviceDiskName(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func classifyNAADevice(diskDevice string) string {
	deviceUpper := deviceDiskName(diskDevice)

	// NAA identifiers with EUI prefix indicate NVMe (colon-delimited: NAA:EUI:...)
	if len(deviceUpper) >= 8 && deviceUpper[:8] == "NAA:EUI:" {
		return "NVMe"
	}

	// NAA identifiers with EUI prefix indicate NVMe (dot-delimited: NAA.EUI...)
	if len(deviceUpper) >= 7 && deviceUpper[:7] == "NAA.EUI" {
		return "NVMe"
	}

	// iSCSI devices often contain IP addresses or IQN patterns in their NAA identifier
	if contains(deviceUpper, "IP:") || contains(deviceUpper, "IQN:") {
		return "iSCSI"
	}

	// Default to FC for standard NAA identifiers (FC WWN)
	return "FC"
}

func classifyT10Device(diskDevice string) string {
	deviceUpper := deviceDiskName(diskDevice)

	if contains(deviceUpper, "NVME") || contains(deviceUpper, "NVM-E") {
		return "NVMe"
	}

	if contains(deviceUpper, "ISCSI") || contains(deviceUpper, "IQN:") {
		return "iSCSI"
	}

	if contains(deviceUpper, "FC") || contains(deviceUpper, "WWN") {
		return "FC"
	}

	return "unknown"
}

func classifyVMHBADevice(diskDevice string) string {
	deviceUpper := deviceDiskName(diskDevice)

	if contains(deviceUpper, "NVME") {
		return "NVMe"
	}

	if contains(deviceUpper, "ISCSI") {
		return "iSCSI"
	}

	if contains(deviceUpper, "FC") || contains(deviceUpper, "VMHBA") {
		return "FC"
	}

	return "unknown"
}

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
