package inventory

type VMInfo struct {
	Name       string
	VCPU       int32
	RAMGB      float64
	StorageGiB float64
}

type DatastoreInfo struct {
	Name        string
	Type        string
	UsedGiB     float64
	AvailGiB    float64
	CapacityGiB float64
}

type SwitchInfo struct {
	Name       string
	SwitchType string
	PortGroup  string
	VLAN       string
	Uplinks    []string
	LACP       string
	TotalPorts int
	UsedPorts  int
}

type PortGroupVM struct {
	Name string
}
