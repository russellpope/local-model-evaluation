package inventory

func ClassifyTransport(name string, fsType string) string {
	if fsType == "NFS" {
		return "NFS"
	}
	if contains(name, "fc") || contains(name, "FC") {
		return "FC"
	}
	if contains(name, "iscsi") || contains(name, "iSCSI") {
		return "iSCSI"
	}
	if contains(name, "nvme") || contains(name, "NVMe") {
		return "NVMe"
	}
	return "unknown"
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
