package main

import "time"

type StorageTransport string

const (
	TransportFC      StorageTransport = "FC"
	TransportISCSI   StorageTransport = "iSCSI"
	TransportNVMe    StorageTransport = "NVMe"
	TransportNFS     StorageTransport = "NFS"
	TransportUnknown StorageTransport = "unknown"
)

type Config struct {
	URL        string
	Username   string
	Password   string
	Insecure   bool
	Timeout    time.Duration
	ConfigFile string
}

type VMInfo struct {
	Name         string
	VCPU         int32
	RAMBytes     int64
	StorageBytes int64
}

type DatastoreInfo struct {
	Name           string
	Type           StorageTransport
	UsedBytes      int64
	AvailableBytes int64
	CapacityBytes  int64
}

type SwitchInfo struct {
	Switch     string
	SwitchType string
	PortGroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	TotalPorts int32
	UsedPorts  int32
}
