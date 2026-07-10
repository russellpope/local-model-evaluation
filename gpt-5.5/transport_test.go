package main

import "testing"

func TestClassifyTransportDescriptor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want StorageTransport
	}{
		{name: "fibre channel", in: "HostFibreChannelHba vmhba2 fc.20000025b5000000", want: TransportFC},
		{name: "iscsi", in: "HostInternetScsiHba iqn.1998-01.com.vmware:host", want: TransportISCSI},
		{name: "nvme", in: "HostNvmeHba vmhba64 nvme controller", want: TransportNVMe},
		{name: "nfs", in: "NFS remote datastore", want: TransportNFS},
		{name: "unknown", in: "local sata disk", want: TransportUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTransportDescriptor(tt.in); got != tt.want {
				t.Fatalf("ClassifyTransportDescriptor(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
