package inventory

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name       string
		descriptor string
		want       string
	}{
		// HBA type names (as seen from %T of concrete VIM types).
		{"FC HBA", "*types.HostFibreChannelHba", TransportFC},
		{"FC HBA bare", "HostFibreChannelHba", TransportFC},
		{"FCoE HBA", "HostFibreChannelOverEthernetHba", TransportFC},
		{"iSCSI HBA", "*types.HostInternetScsiHba", TransportISCSI},
		{"iSCSI HBA bare", "HostInternetScsiHba", TransportISCSI},
		{"NVMe adapter", "HostNVMEAdapter", TransportNVMe},
		{"NVMe controller", "*types.HostNvmeController", TransportNVMe},
		{"parallel SCSI is not a SAN transport", "HostParallelScsiHba", TransportUnknown},
		{"block HBA is not a SAN transport", "HostBlockHba", TransportUnknown},

		// Device identifiers.
		{"FC canonical NAA disk", "naa.6000c29916d8954d5a1b2c3d4e5f6071", TransportFC},
		{"iSCSI IQN", "iqn.1998-01.com.vmware:target-01", TransportISCSI},
		{"iSCSI T10 canonical", "t10.VMware   Virtual disk    1.0     ", TransportISCSI},
		{"NVMe EUI disk", "eui.002538B4547D4889", TransportNVMe},
		{"NVMe-oF NQN", "nqn.2014-08.com.example:nvme:nqn-subsystem-1", TransportNVMe},

		// Unidentifiable inputs must degrade, never misreport.
		{"local mpx disk", "mpx.vmhba1:C0:T0:L0", TransportUnknown},
		{"vml alias", "vml.0005000000766d6862613a303a30", TransportUnknown},
		{"bare adapter name", "vmhba64", TransportUnknown},
		{"empty", "", TransportUnknown},
		{"garbage", "not-a-device-id", TransportUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTransport(tt.descriptor); got != tt.want {
				t.Errorf("ClassifyTransport(%q) = %q, want %q", tt.descriptor, got, tt.want)
			}
		})
	}
}

func TestVlanRanges(t *testing.T) {
	got := vlanRanges([]types.NumericRange{{Start: 0, End: 4094}, {Start: 100, End: 100}})
	if got != "0-4094,100" {
		t.Errorf("vlanRanges = %q, want %q", got, "0-4094,100")
	}
	if got := vlanRanges(nil); got != unknownField {
		t.Errorf("vlanRanges(nil) = %q, want %q", got, unknownField)
	}
}

func TestStandardVLAN(t *testing.T) {
	if got := standardVLAN(0); got != "0" {
		t.Errorf("standardVLAN(0) = %q, want \"0\"", got)
	}
	if got := standardVLAN(123); got != "123" {
		t.Errorf("standardVLAN(123) = %q, want \"123\"", got)
	}
	if got := standardVLAN(4095); !strings.Contains(got, "trunk") {
		t.Errorf("standardVLAN(4095) = %q, want trunk annotation", got)
	}
}
