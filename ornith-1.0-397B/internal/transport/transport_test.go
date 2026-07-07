package transport

import (
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		hbaType    string
		deviceName string
		expected   string
	}{
		{"NVMe by device", "", "nvme0n1", "NVMe"},
		{"NVMe by HBA", "nvme", "", "NVMe"},
		{"NVMe mixed case", "NVMe", "disk1", "NVMe"},
		{"iSCSI by HBA", "vmhba33 (iSCSI)", "", "iSCSI"},
		{"iSCSI by device", "", "iscsi-lun-0", "iSCSI"},
		{"FC by fibre keyword", "fibre channel HBA", "", "FC"},
		{"FC by fc keyword", "fc adapter", "", "FC"},
		{"FC by qla", "qla2xxx", "", "FC"},
		{"FC by lpfc", "lpfc", "", "FC"},
		{"FC by emulex", "emulex", "", "FC"},
		{"unknown empty", "", "", "unknown"},
		{"unknown generic", "generic-hba", "disk1", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.hbaType, tt.deviceName)
			if got != tt.expected {
				t.Errorf("Classify(%q, %q) = %q, want %q", tt.hbaType, tt.deviceName, got, tt.expected)
			}
		})
	}
}

func TestClassifyDatastoreTransport(t *testing.T) {
	tests := []struct {
		name        string
		isNFS       bool
		hbaTypes    []string
		deviceNames []string
		expected    string
	}{
		{"NFS direct", true, nil, nil, "NFS"},
		{"NVMe datastore", false, []string{"nvme"}, []string{"disk1"}, "NVMe"},
		{"iSCSI datastore", false, []string{"vmhba33 (iSCSI)"}, []string{"lun0"}, "iSCSI"},
		{"FC datastore", false, []string{"qla2xxx"}, []string{"lun0"}, "FC"},
		{"no info", false, nil, nil, "unknown"},
		{"multiple extents first NVMe", false, []string{"nvme", "qla2xxx"}, []string{"d1", "d2"}, "NVMe"},
		{"fallback to device name", false, []string{""}, []string{"nvme0n1"}, "NVMe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyDatastoreTransport(tt.isNFS, tt.hbaTypes, tt.deviceNames)
			if got != tt.expected {
				t.Errorf("ClassifyDatastoreTransport(%v, %v, %v) = %q, want %q",
					tt.isNFS, tt.hbaTypes, tt.deviceNames, got, tt.expected)
			}
		})
	}
}
