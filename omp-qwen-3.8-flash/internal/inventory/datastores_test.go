package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

// TestHostDiskProtocols proves the live derivation that maps a host's
// storage subsystem (HBAs, SCSI topology, LUNs) to per-disk protocol names —
// the layer vcsim cannot exercise because it never reports real transport.
func TestHostDiskProtocols(t *testing.T) {
	fcHba := &types.HostFibreChannelHba{
		HostHostBusAdapter: types.HostHostBusAdapter{Key: "key-1", Device: "vmhba2", Driver: "lpfc"},
		PortWorldWideName:  0x2100001b328a95c1,
	}
	iscsiHba := &types.HostInternetScsiHba{
		HostHostBusAdapter: types.HostHostBusAdapter{Key: "key-2", Device: "vmhba65", Driver: "iscsi_tcp"},
		IScsiName:          "iqn.1998-01.com.vmware:esx1-1",
	}
	sasHba := &types.HostParallelScsiHba{
		HostHostBusAdapter: types.HostHostBusAdapter{Key: "key-3", Device: "vmhba0", Driver: "pvscsi"},
	}

	sd := &types.HostStorageDeviceInfo{
		HostBusAdapter: []types.BaseHostHostBusAdapter{fcHba, iscsiHba, sasHba},
		ScsiLun: []types.BaseScsiLun{
			&types.ScsiLun{
				HostDevice:    types.HostDevice{DeviceName: "naa.6000c29f", DeviceType: "disk"},
				Key:           "naa.6000c29f",
				CanonicalName: "naa.6000c29f",
			},
			&types.HostScsiDisk{
				ScsiLun: types.ScsiLun{
					HostDevice:    types.HostDevice{DeviceName: "iqniscsi", DeviceType: "disk"},
					Key:           "iqn.iscsi.disk",
					CanonicalName: "iqn.1992-04.com.emc:cx.disk",
				},
			},
			&types.ScsiLun{
				HostDevice:    types.HostDevice{DeviceName: "local", DeviceType: "disk"},
				Key:           "naa.650local",
				CanonicalName: "naa.650local",
			},
		},
		ScsiTopology: &types.HostScsiTopology{
			Adapter: []types.HostScsiTopologyInterface{
				{
					Key:     "vmhba2",
					Adapter: "vmhba2",
					Target: []types.HostScsiTopologyTarget{{
						Key:       "t1",
						Transport: &types.HostFibreChannelTargetTransport{},
						Lun:       []types.HostScsiTopologyLun{{Key: "l1", Lun: 0, ScsiLun: "naa.6000c29f"}},
					}},
				},
				{
					Key:     "vmhba65",
					Adapter: "vmhba65",
					Target: []types.HostScsiTopologyTarget{{
						Key:       "t2",
						Transport: &types.HostInternetScsiTargetTransport{},
						Lun:       []types.HostScsiTopologyLun{{Key: "l2", Lun: 1, ScsiLun: "iqn.iscsi.disk"}},
					}},
				},
				{
					Key:     "vmhba0",
					Adapter: "vmhba0",
					Target: []types.HostScsiTopologyTarget{{
						Key:       "t3",
						Transport: &types.HostParallelScsiTargetTransport{},
						Lun:       []types.HostScsiTopologyLun{{Key: "l3", Lun: 0, ScsiLun: "naa.650local"}},
					}},
				},
			},
		},
		NvmeTopology: &types.HostNvmeTopology{
			Adapter: []types.HostNvmeTopologyInterface{{
				Key:     "vmhba33",
				Adapter: "vmhba33",
				ConnectedController: []types.HostNvmeController{{
					Key: "c1",
					AttachedNamespace: []types.HostNvmeNamespace{
						{Key: "ns1", Name: "eui.02004cf880546789"},
					},
				}},
			}},
		},
	}
	sd.HostBusAdapter = append(sd.HostBusAdapter, &types.HostBlockHba{
		HostHostBusAdapter: types.HostHostBusAdapter{Key: "key-4", Device: "vmhba33", Driver: "nvme", StorageProtocol: "nvme"},
	})

	got := hostDiskProtocols(sd)

	cases := []struct {
		disk string
		want string
	}{
		{"naa.6000c29f", TransportFC}, // via FC topology target
		{"l1", TransportFC},           // via topology LUN key
		{"iqn.1992-04.com.emc:cx.disk", TransportiSCSI},
		{"naa.650local", ""}, // SAS: must NOT be classified
		{"eui.02004cf880546789", TransportNVMe},
	}
	for _, tc := range cases {
		p := got[tc.disk]
		if tc.want == "" {
			if p != "" {
				t.Errorf("disk %s: got protocol %q, want none", tc.disk, p)
			}
			continue
		}
		if p != tc.want {
			t.Errorf("disk %s: got protocol %q, want %q", tc.disk, p, tc.want)
		}
	}
	if _, ok := got["vmhba0"]; ok {
		t.Error("SAS adapter leaked a protocol binding")
	}
}
