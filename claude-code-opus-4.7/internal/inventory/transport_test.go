package inventory

import "testing"

func TestClassifyTransport(t *testing.T) {
	cases := []struct {
		name string
		in   StorageAdapterDescriptor
		want string
	}{
		{
			name: "fibre channel hba",
			in:   StorageAdapterDescriptor{AdapterType: "HostFibreChannelHba", Driver: "lpfc"},
			want: TransportFC,
		},
		{
			name: "fibre channel over ethernet hba",
			in:   StorageAdapterDescriptor{AdapterType: "HostFibreChannelOverEthernetHba"},
			want: TransportFC,
		},
		{
			name: "fc target transport protocol hint",
			in:   StorageAdapterDescriptor{Protocol: "HostFibreChannelTargetTransport"},
			want: TransportFC,
		},
		{
			name: "iscsi hba with driver",
			in:   StorageAdapterDescriptor{AdapterType: "HostInternetScsiHba", Driver: "iscsi_vmk"},
			want: TransportISCSI,
		},
		{
			name: "iscsi by adapter type only",
			in:   StorageAdapterDescriptor{AdapterType: "HostInternetScsiHba"},
			want: TransportISCSI,
		},
		{
			name: "nvme by driver",
			in:   StorageAdapterDescriptor{Driver: "nvme"},
			want: TransportNVMe,
		},
		{
			name: "nvme over fabrics by protocol",
			in:   StorageAdapterDescriptor{AdapterType: "HostTcpHba", Protocol: "NVMe over Fabrics"},
			want: TransportNVMe,
		},
		{
			name: "local parallel scsi is unknown",
			in:   StorageAdapterDescriptor{AdapterType: "HostParallelScsiHba", Driver: "mpt3sas"},
			want: TransportUnknown,
		},
		{
			name: "generic block hba is unknown",
			in:   StorageAdapterDescriptor{AdapterType: "HostBlockHba"},
			want: TransportUnknown,
		},
		{
			name: "empty descriptor is unknown",
			in:   StorageAdapterDescriptor{},
			want: TransportUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyTransport(tc.in); got != tc.want {
				t.Fatalf("ClassifyTransport(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
