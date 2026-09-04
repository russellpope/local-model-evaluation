// Package inventory retrieves typed vSphere inventory data from vCenter.
// Every retrieval function takes a context and a connected client and returns
// typed results, independent of Cobra wiring and tabwriter presentation.
package inventory

// VMInfo describes one virtual machine.
type VMInfo struct {
	Name      string
	NumCPU    int32
	MemoryMB  int64
	Committed int64 // actual storage consumed (bytes), not provisioned
}

// DatastoreInfo describes one datastore. Transport is the storage protocol
// (FC, iSCSI, NVMe, NFS) derived from the backing devices, not the
// filesystem type; it is "unknown" when no backing can be resolved.
type DatastoreInfo struct {
	Name      string
	Transport string
	FsType    string // VMFS / NFS / ... for diagnostics
	Capacity  int64
	Used      int64
	Available int64
}

// PortgroupInfo is one table row: a port group on a virtual switch.
type PortgroupInfo struct {
	Switch     string
	SwitchType string // "standard" or "distributed"
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string // enabled / disabled / N/A
	Ports      int32
	Used       int32
}
