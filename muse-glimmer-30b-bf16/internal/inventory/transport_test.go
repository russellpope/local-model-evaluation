package inventory

import "testing"

func TestClassifyTransport(t *testing.T) {
	cases := []struct {
		name string
		fs   string
		want string
	}{
		{"ds-fc-01", "VMFS", "FC"},
		{"ds-iscsi-01", "VMFS", "iSCSI"},
		{"ds-nvme-01", "VMFS", "NVMe"},
		{"ds-nfs", "NFS", "NFS"},
		{"ds-unknown", "VMFS", "unknown"},
	}
	for _, c := range cases {
		got := ClassifyTransport(c.name, c.fs)
		if got != c.want {
			t.Errorf("ClassifyTransport(%s,%s) = %s, want %s", c.name, c.fs, got, c.want)
		}
	}
}
