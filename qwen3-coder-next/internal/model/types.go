package model

import (
	"fmt"
)

type VMInfo struct {
	Name    string
	VCPU    int32
	RAM     int64
	Storage int64
}

func (v VMInfo) RAMGB() float64 {
	return float64(v.RAM) / (1024 * 1024 * 1024)
}

func (v VMInfo) StorageHuman() string {
	return formatBytes(v.Storage)
}

type DatastoreInfo struct {
	Name      string
	Type      string
	Capacity  int64
	Used      int64
	Available int64
}

func (d DatastoreInfo) UsedHuman() string {
	return formatBytes(d.Used)
}

func (d DatastoreInfo) AvailableHuman() string {
	return formatBytes(d.Available)
}

type SwitchInfo struct {
	SwitchName    string
	SwitchType    string
	PortgroupName string
	VLAN          string
	Uplinks       []string
	LACP          string
	TotalPorts    int32
	UsedPorts     int32
}

type StorageTransport string

const (
	StorageFC    StorageTransport = "FC"
	StorageISCSI StorageTransport = "iSCSI"
	StorageNVMe  StorageTransport = "NVMe"
	StorageNFS   StorageTransport = "NFS"
)

func (t StorageTransport) String() string {
	return string(t)
}

func ClassifyTransport(info interface{}) string {
	_ = info
	return "unknown"
}

func formatBytes(bytes int64) string {
	const (
		GiB = 1024 * 1024 * 1024
		TiB = 1024 * GiB
	)

	if bytes >= TiB {
		return fmt.Sprintf("%.1f TiB", float64(bytes)/TiB)
	}
	return fmt.Sprintf("%.1f GiB", float64(bytes)/GiB)
}

func init() {
	_ = fmt.Sprintf
}
