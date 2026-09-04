package inventory

import "testing"

// TestClassifyTransport proves the FC/iSCSI/NVMe decision on representative
// descriptors. This is where criterion 4's real logic is validated, since
// vcsim cannot model storage transport topology.
func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name string
		desc DeviceDescriptor
		want string
	}{
		// --- Fibre Channel ---
		{
			name: "fc hba by kind",
			desc: DeviceDescriptor{Kind: "fc", Device: "vmhba2", Driver: "lpfc"},
			want: TransportFC,
		},
		{
			name: "fc driver qla2xxx",
			desc: DeviceDescriptor{Device: "vmhba1", Driver: "qla2xxx"},
			want: TransportFC,
		},
		{
			name: "fc via populated WWN",
			desc: DeviceDescriptor{Device: "vmhba3", WWN: "21:00:00:1b:32:8a:95:c1"},
			want: TransportFC,
		},
		{
			name: "fcoe counts as fc",
			desc: DeviceDescriptor{Kind: "fcoe", Device: "vmhba4", Driver: "bnx2fc"},
			want: TransportFC,
		},

		// --- iSCSI ---
		{
			name: "iscsi hba by kind",
			desc: DeviceDescriptor{Kind: "iscsi", Device: "vmhba65"},
			want: TransportiSCSI,
		},
		{
			name: "software iscsi driver",
			desc: DeviceDescriptor{Device: "vmhba66", Driver: "iscsi_tcp"},
			want: TransportiSCSI,
		},
		{
			name: "bnx2i offload driver",
			desc: DeviceDescriptor{Device: "vmhba67", Driver: "bnx2i"},
			want: TransportiSCSI,
		},
		{
			name: "iqn address",
			desc: DeviceDescriptor{Device: "vmhba68", IQN: "iqn.1992-04.com.emc:cx.apo1234"},
			want: TransportiSCSI,
		},

		// --- NVMe ---
		{
			name: "nvme kind",
			desc: DeviceDescriptor{Kind: "nvme", Device: "vmhba33"},
			want: TransportNVMe,
		},
		{
			name: "nvme driver",
			desc: DeviceDescriptor{Device: "vmhba34", Driver: "nvme"},
			want: TransportNVMe,
		},
		{
			name: "nvme over tcp kind",
			desc: DeviceDescriptor{Kind: "nvme_tcp", Device: "vmhba35"},
			want: TransportNVMe,
		},
		{
			name: "eui namespace identifier",
			desc: DeviceDescriptor{Device: "naa.6d03259d", NAA: "eui.02004cf880546789"},
			want: TransportNVMe,
		},
		{
			name: "nqn subsystem",
			desc: DeviceDescriptor{Device: "nqn.2014-08.org.nvmexpress:uuid:deadbeef"},
			want: TransportNVMe,
		},

		// --- precedence and negatives ---
		{
			name: "kind wins over misleading driver",
			desc: DeviceDescriptor{Kind: "fc", Device: "vmhba2", Driver: "bnx2i"},
			want: TransportFC,
		},
		{
			name: "driver wins over address hint",
			desc: DeviceDescriptor{Driver: "lpfc", NAA: "eui.02004cf880546789"},
			want: TransportFC,
		},
		{
			name: "local sata is unknown",
			desc: DeviceDescriptor{Kind: "sata", Device: "vmhba0", Driver: "ahci"},
			want: TransportUnknown,
		},
		{
			name: "plain naa prefix is ambiguous, stays unknown",
			desc: DeviceDescriptor{Device: "mpx.vmhba0:C0:T0:L0", NAA: "naa.6000c2900"},
			want: TransportUnknown,
		},
		{
			name: "parallel scsi is unknown",
			desc: DeviceDescriptor{Kind: "sas", Device: "vmhba1", Driver: "pvscsi"},
			want: TransportUnknown,
		},
		{
			name: "zero wwn is not fc",
			desc: DeviceDescriptor{Device: "vmhba0", WWN: "0"},
			want: TransportUnknown,
		},
		{
			name: "empty descriptor",
			desc: DeviceDescriptor{},
			want: TransportUnknown,
		},
		{
			name: "whitespace tolerated",
			desc: DeviceDescriptor{Kind: "  FC  "},
			want: TransportFC,
		},
		{
			name: "case insensitive driver",
			desc: DeviceDescriptor{Driver: "NVME_TCP"},
			want: TransportNVMe,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyTransport(tc.desc); got != tc.want {
				t.Errorf("ClassifyTransport(%+v) = %q, want %q", tc.desc, got, tc.want)
			}
		})
	}
}
