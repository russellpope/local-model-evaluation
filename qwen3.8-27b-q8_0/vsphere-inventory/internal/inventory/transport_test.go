package inventory

import "testing"

func TestHbaProtocol(t *testing.T) {
	tests := []struct {
		name string
		hba  HbaInfo
		want string
	}{
		{"fibre channel hba", HbaInfo{Key: "vmhba1", Type: "HostFibreChannelHba"}, TransportFC},
		{"iscsi hba", HbaInfo{Key: "vmhba32", Type: "HostInternetScsiHba"}, TransportISCSI},
		{"nvme protocol on pcie hba", HbaInfo{Key: "vmhba2", Type: "HostPcieHba", StorageProtocol: "nvme"}, TransportNVMe},
		{"nvme protocol on unknown type", HbaInfo{Key: "nvme0", StorageProtocol: "NVMe"}, TransportNVMe},
		{"sas hba is not one of the reported fabrics", HbaInfo{Key: "vmhba3", Type: "HostSerialAttachedHba"}, TransportUnknown},
		{"scsi protocol is ambiguous", HbaInfo{Key: "vmhba4", Type: "HostPcieHba", StorageProtocol: "scsi"}, TransportUnknown},
		{"empty descriptor", HbaInfo{}, TransportUnknown},
	}

	for _, tt := range tests {
		if got := HbaProtocol(tt.hba); got != tt.want {
			t.Errorf("%s: HbaProtocol(%+v) = %q, want %q", tt.name, tt.hba, got, tt.want)
		}
	}
}

func TestParsePathName(t *testing.T) {
	hba, lun, ok := ParsePathName("vmhba1:C0:T0:L0")
	if !ok || hba != "vmhba1" || lun != "C0:T0:L0" {
		t.Errorf("ParsePathName = (%q, %q, %v), want (vmhba1, C0:T0:L0, true)", hba, lun, ok)
	}

	if _, _, ok := ParsePathName("vmhba1"); ok {
		t.Error("ParsePathName without a colon should fail")
	}
	if _, _, ok := ParsePathName(":C0:T0:L0"); ok {
		t.Error("ParsePathName with an empty hba key should fail")
	}
	if _, _, ok := ParsePathName("vmhba1:"); ok {
		t.Error("ParsePathName with an empty lun should fail")
	}
}

func TestClassifyTransport(t *testing.T) {
	fc := HbaInfo{Key: "vmhba1", Type: "HostFibreChannelHba"}
	iscsi := HbaInfo{Key: "vmhba32", Type: "HostInternetScsiHba"}
	nvme := HbaInfo{Key: "vmhba2", Type: "HostPcieHba", StorageProtocol: "nvme"}
	sas := HbaInfo{Key: "vmhba3", Type: "HostSerialAttachedHba"}

	tests := []struct {
		name    string
		devices []string
		luns    []LunPath
		hbas    map[string]HbaInfo
		want    string
	}{
		{
			name:    "single fc path",
			devices: []string{"naa.6001405001"},
			luns:    []LunPath{{Device: "naa.6001405001", Path: "vmhba1:C0:T0:L0"}},
			hbas:    map[string]HbaInfo{"vmhba1": fc},
			want:    TransportFC,
		},
		{
			name:    "single iscsi path",
			devices: []string{"naa.6001405002"},
			luns:    []LunPath{{Device: "naa.6001405002", Path: "vmhba32:C0:T2:L0"}},
			hbas:    map[string]HbaInfo{"vmhba32": iscsi},
			want:    TransportISCSI,
		},
		{
			name:    "single nvme path",
			devices: []string{"naa.6001405003"},
			luns:    []LunPath{{Device: "naa.6001405003", Path: "vmhba2:C0:T0:L0"}},
			hbas:    map[string]HbaInfo{"vmhba2": nvme},
			want:    TransportNVMe,
		},
		{
			name:    "multipath across two fc hbas",
			devices: []string{"naa.6001405001"},
			luns: []LunPath{
				{Device: "naa.6001405001", Path: "vmhba1:C0:T0:L0"},
				{Device: "naa.6001405001", Path: "vmhba5:C1:T0:L0"},
			},
			hbas: map[string]HbaInfo{
				"vmhba1": fc,
				"vmhba5": {Key: "vmhba5", Type: "HostFibreChannelHba"},
			},
			want: TransportFC,
		},
		{
			name:    "dominant protocol wins",
			devices: []string{"naa.6001405001"},
			luns: []LunPath{
				{Device: "naa.6001405001", Path: "vmhba1:C0:T0:L0"},
				{Device: "naa.6001405001", Path: "vmhba1:C0:T1:L0"},
				{Device: "naa.6001405001", Path: "vmhba32:C0:T0:L0"},
			},
			hbas: map[string]HbaInfo{"vmhba1": fc, "vmhba32": iscsi},
			want: TransportFC,
		},
		{
			name:    "tie is ambiguous",
			devices: []string{"naa.6001405001"},
			luns: []LunPath{
				{Device: "naa.6001405001", Path: "vmhba1:C0:T0:L0"},
				{Device: "naa.6001405001", Path: "vmhba32:C0:T0:L0"},
			},
			hbas: map[string]HbaInfo{"vmhba1": fc, "vmhba32": iscsi},
			want: TransportUnknown,
		},
		{
			name:    "paths to other devices are ignored",
			devices: []string{"naa.6001405001"},
			luns:    []LunPath{{Device: "naa.6001405999", Path: "vmhba1:C0:T0:L0"}},
			hbas:    map[string]HbaInfo{"vmhba1": fc},
			want:    TransportUnknown,
		},
		{
			name:    "unknown hba key",
			devices: []string{"naa.6001405001"},
			luns:    []LunPath{{Device: "naa.6001405001", Path: "vmhba9:C0:T0:L0"}},
			hbas:    map[string]HbaInfo{"vmhba1": fc},
			want:    TransportUnknown,
		},
		{
			name:    "malformed path",
			devices: []string{"naa.6001405001"},
			luns:    []LunPath{{Device: "naa.6001405001", Path: "not-a-path"}},
			hbas:    map[string]HbaInfo{"vmhba1": fc},
			want:    TransportUnknown,
		},
		{
			name:    "sas is not a reported fabric",
			devices: []string{"naa.6001405001"},
			luns:    []LunPath{{Device: "naa.6001405001", Path: "vmhba3:C0:T0:L0"}},
			hbas:    map[string]HbaInfo{"vmhba3": sas},
			want:    TransportUnknown,
		},
		{
			name:    "no devices",
			devices: nil,
			luns:    []LunPath{{Device: "naa.6001405001", Path: "vmhba1:C0:T0:L0"}},
			hbas:    map[string]HbaInfo{"vmhba1": fc},
			want:    TransportUnknown,
		},
	}

	for _, tt := range tests {
		if got := ClassifyTransport(tt.devices, tt.luns, tt.hbas); got != tt.want {
			t.Errorf("%s: ClassifyTransport = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDatastoreVolumeUUID(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"ds:///vmfs/volumes/51b8c3d2-e4f1-4a1a-9a1a-1a1a1a1a1a1a/", "51b8c3d2-e4f1-4a1a-9a1a-1a1a1a1a1a1a"},
		{"ds:///vmfs/volumes/51b8c3d2-e4f1-4a1a-9a1a-1a1a1a1a1a1a/subdir/file.vmdk", "51b8c3d2-e4f1-4a1a-9a1a-1a1a1a1a1a1a"},
		{"file:///tmp/LocalDS_0", ""},
		{"nfs://host/share", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := datastoreVolumeUUID(tt.url); got != tt.want {
			t.Errorf("datastoreVolumeUUID(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
