package transport

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		descriptor string
		want       string
	}{
		// FC: Fibre Channel WWN-prefixed disks and FC/FCoE HBAs.
		{"FC WWN disk", "naa.600507680281903f2f00000000000000", FC},
		{"Fibre Channel HBA", "Fibre Channel HostBusAdapter qfc28xx", FC},
		{"FCoE HBA", "fd--dm-5.5  FCoE 2 ports vmhba1", FC},

		// iSCSI: IQN targets, EUI LUNs and iSCSI HBAs.
		{"iSCSI IQN target", "iqn.1998-01.com.vmware:iscsi-disk-0001", ISCSI},
		{"iSCSI EUI LUN", "eui.0024261aaabbccdd", ISCSI},
		{"iSCSI HBA", "QLogic iSCSI vmhba33", ISCSI},

		// NVMe: both the model string and raw NVMe disk names. Note these
		// contain "fc"-like text too (NVMe over Fibre) and must stay NVMe.
		{"Nvme-over-FC HBA", "NvmeOverFc vmhba64", NVMe},
		{"NVMe disk", "t10.NVMe____2b0126f6e4d71173", NVMe},

		// Unclassifiable inputs degrade to unknown.
		{"local SATA disk", "/dev/sda", Unknown},
		{"bare adapter name", "vmhba1", Unknown},
		{"empty", "", Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.descriptor); got != tt.want {
				t.Errorf("Classify(%q) = %q, want %q", tt.descriptor, got, tt.want)
			}
		})
	}
}
