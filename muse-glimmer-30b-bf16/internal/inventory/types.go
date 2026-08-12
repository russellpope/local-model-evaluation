package inventory

type VMInfo struct {
	Name    string
	VCPU    int32
	RAMGiB  float64
	Storage string
}

type DatastoreInfo struct {
	Name      string
	Type      string
	Used      string
	Available string
}

type SwitchInfo struct {
	SwitchName string
	SwitchType string
	PortGroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	Ports      int32
	Used       int32
}
