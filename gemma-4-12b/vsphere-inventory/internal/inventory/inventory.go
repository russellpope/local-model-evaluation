package inventory

import (
	"context"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/vim25"
)

type VMInfo struct {
	Name     string
	VCPU     int
	RAM      int64
	Storage  int64
}

func (l VMInfoList) Len() int           { return len(l) }
func (l VMInfoList) Swap(i, j int)     { l[i], l[j] = l[j], l[i] }
func (l VMInfoList) Less(i, j int) bool { return l[i].Name < l[j].Name }
type VMInfoList []VMInfo

type DatastoreInfo struct {
	Name       string
	Type       string
	Used       int64
	Available  int64
}

func (l DatastoreInfoList) Len() int           { return len(l) }
func (l DatastoreInfoList) Swap(i, j int)     { l[i], l[j] = l[j], l[i] }
func (l DatastoreInfoList) Less(i, j int) bool { return l[i].Name < l[j].Name }
type DatastoreInfoList []DatastoreInfo

type SwitchInfo struct {
	SwitchName  string
	SwitchType  string
	PortGroup   string
	VLAN        string
	Uplinks     string
	LACP        string
	TotalPorts  int
	UsedPorts   int
}

// GetVMs retrieves all VMs and their details.
func GetVMs(ctx context.Context, client *vim25.Client) ([]VMInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetDatastores retrieves all datastores and their details.
func GetDatastores(ctx context.Context, client *vim25.Client) ([]DatastoreInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetSwitches retrieves all switches or VMs in a port group.
func GetSwitches(ctx context.Context, client *vim25.Client, portGroupName string) ([]SwitchInfo, error) {
	if portGroupName != "" {
		return getVMsInPortGroup(ctx, client, portGroupName)
	}

	return nil, fmt.Errorf("not implemented")
}

func getVMsInPortGroup(ctx context.Context, client *vim25.Client, portGroupName string) ([]SwitchInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// ClassifyTransport is a pure function for determining storage transport.
func ClassifyTransport(deviceType string, adapterType string) string {
	if strings.Contains(strings.ToLower(deviceType), "nfs") || strings.Contains(strings.ToLower(adapterType), "nfs") {
		return "NFS"
	}
	if strings.Contains(strings.ToLower(deviceType), "fc") || strings.Contains(strings.ToLower(adapterType), "fc") {
		return "FC"
	}
	if strings.Contains(strings.ToLower(deviceType), "iscsi") || strings.Contains(strings.ToLower(adapterType), "iscsi") {
		return "iSCSI"
	}
	if strings.Contains(strings.ToLower(deviceType), "nvme") || strings.Contains(strings.ToLower(adapterType), "nvme") {
		return "NVMe"
	}
	return "unknown"
}
