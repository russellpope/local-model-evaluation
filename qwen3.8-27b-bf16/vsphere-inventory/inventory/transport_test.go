package inventory

import "testing"

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name          string
		datastoreType string
		hba           HBADescriptor
		want          string
	}{
		{
			name:          "NFS datastore wins over HBA",
			datastoreType: "NFS",
			hba:           HBADescriptor{Kind: "fibrechannel", Model: "QLE2672"},
			want:          TransportNFS,
		},
		{
			name:          "nfs lowercase",
			datastoreType: "nfs",
			hba:           HBADescriptor{},
			want:          TransportNFS,
		},
		{
			name:          "NVMe storage protocol on any HBA",
			datastoreType: "VMFS",
			hba:           HBADescriptor{StorageProtocol: "nvme", Kind: "block", Model: "PM9A3"},
			want:          TransportNVMe,
		},
		{
			name:          "fibre channel HBA class",
			datastoreType: "VMFS",
			hba:           HBADescriptor{StorageProtocol: "scsi", Kind: "fibrechannel", Model: "QLE2672"},
			want:          TransportFC,
		},
		{
			name:          "fibre channel over ethernet HBA class",
			datastoreType: "VMFS",
			hba:           HBADescriptor{Kind: "fibrechannelethernet", Model: "FCoE"},
			want:          TransportFC,
		},
		{
			name:          "internet SCSI HBA class",
			datastoreType: "VMFS",
			hba:           HBADescriptor{StorageProtocol: "scsi", Kind: "internetscsi", Model: "VMware Virtual iSCSI", Driver: "vmkata"},
			want:          TransportISCSI,
		},
		{
			name:          "block HBA with NVMe model",
			datastoreType: "VMFS",
			hba:           HBADescriptor{StorageProtocol: "nvme", Kind: "block", Model: "Samsung MZWL NVMe"},
			want:          TransportNVMe,
		},
		{
			name:          "block HBA with iSCSI model",
			datastoreType: "VMFS",
			hba:           HBADescriptor{Kind: "block", Model: "Chelsio iSCSI", Driver: "cxgbe"},
			want:          TransportISCSI,
		},
		{
			name:          "block HBA with fibre channel model",
			datastoreType: "VMFS",
			hba:           HBADescriptor{Kind: "block", Model: "Emulex Beacon2 FC"},
			want:          TransportFC,
		},
		{
			name:          "local parallel SCSI degrades to unknown",
			datastoreType: "local",
			hba:           HBADescriptor{StorageProtocol: "scsi", Kind: "parallelscsi", Model: "PVSCSI SCSI Controller"},
			want:          TransportUnknown,
		},
		{
			name:          "unrecognized block HBA degrades to unknown",
			datastoreType: "VMFS",
			hba:           HBADescriptor{StorageProtocol: "scsi", Kind: "block", Model: "Dell PERC H730 Mini"},
			want:          TransportUnknown,
		},
		{
			name:          "no HBA info degrades to unknown",
			datastoreType: "VMFS",
			hba:           HBADescriptor{},
			want:          TransportUnknown,
		},
	}
	for _, tc := range tests {
		if got := ClassifyTransport(tc.datastoreType, tc.hba); got != tc.want {
			t.Errorf("%s: ClassifyTransport(%q, %+v) = %q, want %q", tc.name, tc.datastoreType, tc.hba, got, tc.want)
		}
	}
}
