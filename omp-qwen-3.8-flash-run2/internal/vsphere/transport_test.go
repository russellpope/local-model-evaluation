package vsphere

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyCanonicalName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Fibre Channel LUNs: durable NAA ids, fc-prefixed.
		{"fc naa", "naa.6006016041203d0e241ce7ee63e1e701", TransportFC},
		{"fc naa partition", "naa.600a09818d5a536e0000000000000001", TransportFC},
		{"fc prefix", "fc.210000109b20d3c4", TransportFC},
		{"fc mixed case", "NAA.6006016041203D0E", TransportFC},

		// iSCSI LUNs/aliases.
		{"iscsi iqn", "iqn.1992-04.com.example:storage.disk1", TransportiSCSI},
		{"iscsi embedded iqn", "mpx.iqn.2001-04.com.example:array:1", TransportiSCSI},

		// NVMe namespaces.
		{"nvme eui", "eui.0025385b511948a5", TransportNVMe},
		{"nvme nqn", "nqn.2020-06.com.example:nvme:subsys1", TransportNVMe},
		{"nvme prefix", "nvme.0000-abcdef-000000001", TransportNVMe},

		// Local / unclassifiable names intentionally unknown.
		{"local mpx", "mpx.vmhba0:C0:T0:L0", TransportUnknown},
		{"local t10", "t10.VMware___Virtual_disk___00000000", TransportUnknown},
		{"empty", "", TransportUnknown},
		{"garbage", "disk-7", TransportUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyCanonicalName(tc.in); got != tc.want {
				t.Errorf("ClassifyCanonicalName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClassifyHBA(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"fc hba type", "*types.HostFibreChannelHba", TransportFC},
		{"iscsi hba type", "*types.HostInternetScsiHba", TransportiSCSI},
		{"nvme hba type", "*types.HostNvmeHba", TransportNVMe},
		{"parallel scsi", "*types.HostParallelScsiHba", TransportUnknown},
		{"device descriptor nvme", "vmhba2 (nvme)", TransportNVMe},
		{"lowercase isc si", "software iscsi", TransportiSCSI},
		{"empty", "", TransportUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyHBA(tc.in); got != tc.want {
				t.Errorf("ClassifyHBA(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClassifyScsiLun(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		durableNS string
		want      string
	}{
		{"canonical drives", "naa.6006016041203d0e", "", TransportFC},
		{"durable eui overrides local canonical", "t10.SOMELOCAL", "eui", TransportNVMe},
		{"durable nqn", "naa.6001", "nqn", TransportNVMe},
		{"unknown ns falls back", "iqn.1994-09.com.example", "vendor", TransportiSCSI},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyScsiLun(tc.canonical, tc.durableNS); got != tc.want {
				t.Errorf("ClassifyScsiLun(%q, %q) = %q, want %q", tc.canonical, tc.durableNS, got, tc.want)
			}
		})
	}
}

func TestClassifyNVMeController(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"pcie", TransportNVMe},
		{"loopback", TransportNVMe},
		{"rdma", TransportNVMe},
		{"tcp", TransportNVMe},
		{"fc", TransportNVMe},
		{"infiniband", TransportNVMe},
		{"PCIE", TransportNVMe},
		{"", TransportUnknown},
	}
	for _, tc := range tests {
		if got := ClassifyNVMeController(tc.in); got != tc.want {
			t.Errorf("ClassifyNVMeController(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDeviceTransport(t *testing.T) {
	hs := hostStorage{
		adapterTransport: map[string]string{
			"vmhba1": TransportFC,
			"vmhba2": TransportiSCSI,
			"vmhba3": TransportNVMe,
			"vmhba0": TransportUnknown, // local sata
		},
		lunTransport: map[string]string{
			"naa.6006016041203d0e":          TransportFC,
			"key-vim.scsilun-naa.600601604": TransportFC,
			"eui.0025385b511948a5":          TransportNVMe,
		},
		nvmeTransport: map[string]string{
			"nvme-naa.689e10da": TransportNVMe,
			"key-vim.nvme.NS42": TransportNVMe,
		},
	}

	tests := []struct {
		name string
		disk string
		want string
	}{
		{"exact lun match", "naa.6006016041203d0e", TransportFC},
		{"lun key", "key-vim.scsilun-naa.600601604", TransportFC},
		{"naa with partition suffix", "naa.6006016041203d0e:1", TransportFC},
		{"nvme namespace by key", "nvme-naa.689e10da", TransportNVMe},
		{"nvme by lun table", "eui.0025385b511948a5", TransportNVMe},
		{"iscsi path via adapter", "mpx.vmhba2:C0:T1:L2", TransportiSCSI},
		{"fc path via adapter", "mpx.vmhba1:C0:T1:L2", TransportFC},
		{"nvme path via adapter", "nvme.3:0:C0:T1:L2", TransportNVMe},
		{"local adapter unknown", "mpx.vmhba0:C0:T0:L0", TransportUnknown},
		{"unmatched iqn string", "iqn.1992-04.com.example:vol", TransportiSCSI},
		{"unknown junk", "disk-7", TransportUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hs.deviceTransport(tc.disk); got != tc.want {
				t.Errorf("deviceTransport(%q) = %q, want %q", tc.disk, got, tc.want)
			}
		})
	}
}

func TestClassifyVolume(t *testing.T) {
	hs := hostStorage{
		adapterTransport: map[string]string{"vmhba1": TransportFC, "vmhba2": TransportiSCSI},
		lunTransport: map[string]string{
			"naa.600aaa": TransportFC,
			"naa.600bbb": TransportFC,
			"naa.700ccc": TransportiSCSI,
		},
		nvmeTransport: map[string]string{},
	}

	mk := func(names ...string) []types.HostScsiDiskPartition {
		out := make([]types.HostScsiDiskPartition, 0, len(names))
		for _, n := range names {
			out = append(out, types.HostScsiDiskPartition{DiskName: n})
		}
		return out
	}

	tests := []struct {
		name    string
		extents []types.HostScsiDiskPartition
		want    string
	}{
		{"single fc", mk("naa.600aaa"), TransportFC},
		{"multi consistent", mk("naa.600aaa", "naa.600bbb"), TransportFC},
		{"mixed fabric is unknown", mk("naa.600aaa", "naa.700ccc"), TransportUnknown},
		{"one unclassifiable", mk("naa.600aaa", "mpx.vmhba0:C0:T0:L0"), TransportUnknown},
		{"empty", mk(), TransportUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyVolume(hs, tc.extents); got != tc.want {
				t.Errorf("classifyVolume = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsNFSFileSystem(t *testing.T) {
	nfs := []string{"NFS", "NFS41", "IFS", "CIFS"}
	for _, s := range nfs {
		if !IsNFSFileSystem(s) {
			t.Errorf("IsNFSFileSystem(%q) = false, want true", s)
		}
	}
	block := []string{"VMFS", "VSAN", "VFFS", "vVols", "FAT16", ""}
	for _, s := range block {
		if IsNFSFileSystem(s) {
			t.Errorf("IsNFSFileSystem(%q) = true, want false", s)
		}
	}
}

func TestFormatStandardVLAN(t *testing.T) {
	if got := formatStandardVLAN(0); got != "0" {
		t.Errorf("vlan 0 = %q", got)
	}
	if got := formatStandardVLAN(100); got != "100" {
		t.Errorf("vlan 100 = %q", got)
	}
	if got := formatStandardVLAN(4095); got != "trunk(0-4094)" {
		t.Errorf("vlan 4095 (trunk) = %q", got)
	}
}
