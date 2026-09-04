package transport

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestFromTargetTransport(t *testing.T) {
	tests := []struct {
		name string
		in   types.BaseHostTargetTransport
		want string
	}{
		{"fibre channel", &types.HostFibreChannelTargetTransport{}, FC},
		{"iscsi", &types.HostInternetScsiTargetTransport{}, ISCSI},
		{"parallel scsi is not a fabric protocol", &types.HostParallelScsiTargetTransport{}, Unknown},
		{"block adapter is local", &types.HostBlockAdapterTargetTransport{}, Unknown},
		{"nil transport", nil, Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromTargetTransport(tt.in); got != tt.want {
				t.Errorf("FromTargetTransport(%T) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFromHBA(t *testing.T) {
	tests := []struct {
		name string
		in   types.BaseHostHostBusAdapter
		want string
	}{
		{"fibre channel HBA", &types.HostFibreChannelHba{}, FC},
		{"iscsi HBA", &types.HostInternetScsiHba{}, ISCSI},
		{"block HBA is local", &types.HostBlockHba{}, Unknown},
		{"nil HBA", nil, Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromHBA(tt.in); got != tt.want {
				t.Errorf("FromHBA(%T) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func nvmeTopology(device string) *types.HostNvmeTopology {
	return &types.HostNvmeTopology{
		Adapter: []types.HostNvmeTopologyInterface{{
			Adapter: "vmhba64",
			ConnectedController: []types.HostNvmeController{{
				AttachedNamespace: []types.HostNvmeNamespace{{Name: device}},
			}},
		}},
	}
}

func TestDeviceInNVMeTopology(t *testing.T) {
	topo := nvmeTopology("nqn.2014-08.org.nvmexpress:uuid:ns1")
	tests := []struct {
		name   string
		device string
		topo   *types.HostNvmeTopology
		want   bool
	}{
		{"matching namespace", "nqn.2014-08.org.nvmexpress:uuid:ns1", topo, true},
		{"other device", "naa.6001405abc", topo, false},
		{"empty device", "", topo, false},
		{"nil topology", "nqn.2014-08.org.nvmexpress:uuid:ns1", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeviceInNVMeTopology(tt.device, tt.topo); got != tt.want {
				t.Errorf("DeviceInNVMeTopology(%q) = %v, want %v", tt.device, got, tt.want)
			}
		})
	}
}

func scsiDeviceInfo(device string, transport types.BaseHostTargetTransport) *types.HostStorageDeviceInfo {
	lun := &types.HostScsiDisk{
		ScsiLun: types.ScsiLun{
			Key:           "key-vim.host.ScsiDisk-" + device,
			DeviceName:    device,
			CanonicalName: device,
		},
	}
	return &types.HostStorageDeviceInfo{
		ScsiLun: []types.BaseScsiLun{lun},
		ScsiTopology: &types.HostScsiTopology{
			Adapter: []types.HostScsiTopologyInterface{{
				Adapter: "vmhba2",
				Target: []types.HostScsiTopologyTarget{{
					Transport: transport,
					Lun:       []types.HostScsiTopologyLun{{ScsiLun: lun.Key}},
				}},
			}},
		},
	}
}

func TestClassifyDatastore(t *testing.T) {
	fcDevice := "naa.6001405fc0ffee01"
	iscsiDevice := "naa.6001405iscsi01"
	nvmeDevice := "nqn.2014-08.org.nvmexpress:uuid:ns1"

	nvmeInfo := &types.HostStorageDeviceInfo{NvmeTopology: nvmeTopology(nvmeDevice)}

	tests := []struct {
		name    string
		fsType  string
		extents []string
		info    *types.HostStorageDeviceInfo
		want    string
	}{
		{
			name:   "NFS datastore reports NFS without host storage info",
			fsType: "NFS",
			info:   nil,
			want:   NFS,
		},
		{
			name:    "VMFS extent on a fibre channel target",
			fsType:  "VMFS",
			extents: []string{fcDevice},
			info:    scsiDeviceInfo(fcDevice, &types.HostFibreChannelTargetTransport{}),
			want:    FC,
		},
		{
			name:    "VMFS extent on an iSCSI target",
			fsType:  "VMFS",
			extents: []string{iscsiDevice},
			info:    scsiDeviceInfo(iscsiDevice, &types.HostInternetScsiTargetTransport{}),
			want:    ISCSI,
		},
		{
			name:    "VMFS extent backed by an NVMe namespace",
			fsType:  "VMFS",
			extents: []string{nvmeDevice},
			info:    nvmeInfo,
			want:    NVMe,
		},
		{
			name:    "VMFS extent on an unrecognized transport",
			fsType:  "VMFS",
			extents: []string{fcDevice},
			info:    scsiDeviceInfo(fcDevice, &types.HostParallelScsiTargetTransport{}),
			want:    Unknown,
		},
		{
			name:    "VMFS with no storage device info",
			fsType:  "VMFS",
			extents: []string{fcDevice},
			info:    nil,
			want:    Unknown,
		},
		{
			name:    "extent not present in topology",
			fsType:  "VMFS",
			extents: []string{"naa.deadbeef"},
			info:    scsiDeviceInfo(fcDevice, &types.HostFibreChannelTargetTransport{}),
			want:    Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyDatastore(tt.fsType, tt.extents, tt.info); got != tt.want {
				t.Errorf("ClassifyDatastore(%q, %v) = %q, want %q", tt.fsType, tt.extents, got, tt.want)
			}
		})
	}
}
