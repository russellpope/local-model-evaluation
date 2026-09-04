// Package inventory retrieves vSphere inventory through a vim25 client and
// returns typed, presentation-free results. The retrieval functions are the
// unit under test (against govmomi's embedded simulator); the cobra wiring
// and tabwriter rendering live in the cmd package.
package inventory

// VMInfo describes one virtual machine. CommittedBytes is the storage the VM
// actually consumes (summary.storage.committed), not provisioned capacity.
type VMInfo struct {
	Name           string
	VCPU           int32
	RAMMib         int64
	CommittedBytes int64
}

// DatastoreInfo describes one datastore. Type is the backing transport
// (FC, iSCSI, NVMe or NFS), not the filesystem type. The three byte fields
// satisfy used = capacity - available.
type DatastoreInfo struct {
	Name           string
	Type           string
	CapacityBytes  int64
	UsedBytes      int64
	AvailableBytes int64
}

// PortGroupInfo is one port group row of a switch listing.
type PortGroupInfo struct {
	Switch     string
	SwitchType string // "standard" or "distributed"
	Name       string
	VLAN       string // single ID, trunk range, pvlan type, or "unknown"
	Ports      int
	Used       int
}

// SwitchInfo describes one virtual switch (standard or distributed) and its
// port groups. LACP is "enabled"/"disabled" for distributed switches and
// "N/A" for standard ones. Uplinks is a comma-separated list of physical
// NICs / uplink port names, or "unknown" when the API does not expose them.
type SwitchInfo struct {
	Name       string
	Type       string
	Uplinks    string
	LACP       string
	Ports      int
	Used       int
	PortGroups []PortGroupInfo
}

// Switch type values.
const (
	SwitchTypeStandard    = "standard"
	SwitchTypeDistributed = "distributed"
)

// LACP reporting values.
const (
	LACPEnabled  = "enabled"
	LACPDisabled = "disabled"
	LACPNA       = "N/A"
)

// Placeholder keeps the sentinel values discoverable from one place.
const UnknownValue = "unknown"
