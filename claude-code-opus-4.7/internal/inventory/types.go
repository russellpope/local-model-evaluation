// Package inventory contains the vSphere inventory-retrieval logic and the
// pure helpers that support it. Retrieval functions take a context and a
// vim25 client and return typed results, keeping them independent of the
// Cobra command wiring and the tabwriter presentation so each can be tested
// directly.
package inventory

// Transport classification values for a datastore's backing storage.
const (
	TransportFC      = "FC"
	TransportISCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// LACP reporting values.
const (
	LACPEnabled  = "enabled"
	LACPDisabled = "disabled"
	LACPNA       = "N/A"
)

// Switch type values.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"
)

// VMInfo is the per-virtual-machine result of the vms feature.
type VMInfo struct {
	Name string
	// NumCPU is the configured vCPU count.
	NumCPU int32
	// MemoryMB is the configured memory size in megabytes.
	MemoryMB int64
	// CommittedBytes is the actual storage consumed (committed) by the VM,
	// not its provisioned/allocated capacity.
	CommittedBytes int64
}

// DatastoreInfo is the per-datastore result of the datastores feature.
type DatastoreInfo struct {
	Name string
	// Type is the underlying transport/protocol: FC, iSCSI, NVMe, NFS, or
	// unknown when it cannot be derived from the API.
	Type string
	// CapacityBytes is the total capacity of the datastore.
	CapacityBytes int64
	// FreeBytes is the free/available capacity of the datastore.
	FreeBytes int64
}

// UsedBytes returns used capacity, defined as total capacity minus available.
func (d DatastoreInfo) UsedBytes() int64 {
	return d.CapacityBytes - d.FreeBytes
}

// PortGroupInfo describes a single port group on a virtual switch.
type PortGroupInfo struct {
	Name string
	// VLAN is the rendered VLAN descriptor: a single ID ("100"), a trunk
	// range ("trunk 0-4094"), a private VLAN ("pvlan 200"), or "none".
	VLAN string
	// TotalPorts is the total number of ports available to the port group.
	// For standard switches this is the owning vSwitch's port total.
	TotalPorts int32
	// UsedPorts is the number of ports currently in use.
	UsedPorts int32
}

// SwitchInfo describes a virtual switch (standard or distributed) and its
// port groups.
type SwitchInfo struct {
	Name string
	// Type is SwitchStandard or SwitchDistributed.
	Type string
	// Uplinks are the physical NIC(s)/uplink port name(s) backing the switch.
	Uplinks []string
	// LACP is LACPEnabled/LACPDisabled for distributed switches and LACPNA
	// for standard switches (LACP is a distributed-switch concept).
	LACP       string
	PortGroups []PortGroupInfo
}

// StorageAdapterDescriptor captures, in neutral terms extractable from the
// vSphere API, the host bus adapter and/or LUN protocol backing a datastore
// extent. Classifying the transport from this descriptor is a pure decision,
// which is what makes criterion 4's logic testable without a live array.
type StorageAdapterDescriptor struct {
	// AdapterType is the concrete govmomi HBA type name when known, e.g.
	// "HostFibreChannelHba", "HostFibreChannelOverEthernetHba",
	// "HostInternetScsiHba", "HostParallelScsiHba", "HostBlockHba",
	// "HostTcpHba", "HostPcieHba".
	AdapterType string
	// Driver is the adapter driver name, e.g. "lpfc", "qlnativefc",
	// "iscsi_vmk", "nvmetcp".
	Driver string
	// Model is the human-readable adapter model string.
	Model string
	// Protocol is an explicit protocol hint from the API when available
	// (e.g. a target-transport type name), otherwise empty.
	Protocol string
}
