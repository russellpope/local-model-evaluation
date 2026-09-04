package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		descriptor string
		want       string
	}{
		// Fiber Channel
		{"*types.HostFibreChannelHba", "FC"},
		{"HostFibreChannelHba", "FC"},
		{"lpfc", "FC"},
		{"qla2xxx fc mode", "FC"},
		{"fibrechannel", "FC"},
		// iSCSI
		{"*types.HostInternetScsiHba", "iSCSI"},
		{"iscsi", "iSCSI"},
		{"iscsi vmhba37", "iSCSI"},
		// NVMe
		{"*types.HostNvmeController", "NVMe"},
		{"nvme", "NVMe"},
		{"nvmetcp", "NVMe"},
		{"nvme over rdma", "NVMe"},
		// NFS
		{"nfs", "NFS"},
		{"NFS41", "NFS"},
		// Non-answer inputs must degrade to unknown, not be guessed.
		{"*types.HostParallelScsiHba", "unknown"},
		{"*types.HostBlockHba", "unknown"},
		{"vmfs", "unknown"},
		{"", "unknown"},
		{"anything else", "unknown"},
	}
	for _, tt := range tests {
		if got := ClassifyTransport(tt.descriptor); got != tt.want {
			t.Errorf("ClassifyTransport(%q) = %q, want %q", tt.descriptor, got, tt.want)
		}
	}
}

func TestDvsVlanRendering(t *testing.T) {
	tests := []struct {
		name    string
		setting *types.VMwareDVSPortSetting
		want    string
	}{
		{"access vlan 10", &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{VlanId: 10}}, "10"},
		{"access vlan 0", &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{VlanId: 0}}, "0"},
		{"trunk single", &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
			VlanId: []types.NumericRange{{Start: 100, End: 100}},
		}}, "100"},
		{"trunk range", &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
			VlanId: []types.NumericRange{{Start: 100, End: 200}, {Start: 300, End: 300}},
		}}, "100-200,300"},
		{"trunk full", &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchTrunkVlanSpec{
			VlanId: []types.NumericRange{{Start: 0, End: 4094}},
		}}, "0-4094"},
		{"private vlan", &types.VMwareDVSPortSetting{Vlan: &types.VmwareDistributedVirtualSwitchPvlanSpec{PvlanId: 42}}, "private-vlan:42"},
		{"nil vlan", &types.VMwareDVSPortSetting{}, "unknown"},
	}
	for _, tt := range tests {
		if got := dvsVlan(tt.setting); got != tt.want {
			t.Errorf("%s: dvsVlan = %q, want %q", tt.name, got, tt.want)
		}
	}
	if got := dvsVlan(nil); got != "unknown" {
		t.Errorf("nil port config: dvsVlan = %q, want unknown", got)
	}
	if got := dvsVlan(&types.DVPortSetting{}); got != "unknown" {
		t.Errorf("generic port setting (no VLAN modelled): dvsVlan = %q, want unknown", got)
	}
}

func TestSwitchPortUsage(t *testing.T) {
	vsw := types.HostVirtualSwitch{
		NumPorts:          1536,
		NumPortsAvailable: 1530,
		Spec:              types.HostVirtualSwitchSpec{NumPorts: 128},
	}
	total, used, ok := switchPortUsage(vsw)
	if !ok || total != 1536 || used != 6 {
		t.Errorf("switchPortUsage = (%d,%d,%v), want (1536,6,true)", total, used, ok)
	}
	// No runtime counters: falls back to the configured spec size.
	specOnly := types.HostVirtualSwitch{Spec: types.HostVirtualSwitchSpec{NumPorts: 128}}
	total, _, ok = switchPortUsage(specOnly)
	if ok || total != 128 {
		t.Errorf("spec-only switch: total=%d derived=%v, want 128,false", total, ok)
	}
}

func TestPnicNames(t *testing.T) {
	got := pnicNames([]string{"key-vim.host.PhysicalNic-vmnic1", "key-vim.host.PhysicalNic-vmnic0"})
	if got != "vmnic0,vmnic1" {
		t.Errorf("pnicNames = %q, want vmnic0,vmnic1", got)
	}
	if got := mergeUplinks("vmnic0", "vmnic1"); got != "vmnic0,vmnic1" {
		t.Errorf("mergeUplinks = %q", got)
	}
}
