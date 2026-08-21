package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// pgTarget identifies a port group (standard or distributed) by name so that
// connected virtual machines can be located.
type pgTarget struct {
	name    string
	kind    string // "standard" or "distributed"
	netRef  string // standard: Network managed object reference
	dvsUuid string // distributed: switch uuid
	pgKey   string // distributed: port group key
}

// VMsForPortGroup returns the virtual machines connected to the named port
// group, working for both standard and distributed port groups.
func VMsForPortGroup(ctx context.Context, client *vim25.Client, name string) ([]PortGroupVM, error) {
	targets, err := findPortGroups(ctx, client, name)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no port group named %q was found", name)
	}

	finder, err := newFinder(ctx, client)
	if err != nil {
		return nil, err
	}
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing virtual machines: %w", err)
	}

	var result []PortGroupVM
	for _, vm := range vms {
		var moVM mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device", "config.name"}, &moVM); err != nil {
			return nil, fmt.Errorf("reading devices of VM %q: %w", vmName(vm, &moVM), err)
		}
		for _, tgt := range targets {
			if vmConnectedTo(&moVM, tgt) {
				result = append(result, PortGroupVM{Name: vmName(vm, &moVM)})
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// findPortGroups locates every port group (standard and distributed) with the
// given name.
func findPortGroups(ctx context.Context, client *vim25.Client, name string) ([]pgTarget, error) {
	var targets []pgTarget

	finder, err := newFinder(ctx, client)
	if err != nil {
		return nil, err
	}
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}
	for _, host := range hosts {
		var moHost mo.HostSystem
		if err := host.Properties(ctx, host.Reference(), []string{"config.network"}, &moHost); err != nil {
			return nil, fmt.Errorf("reading network config of host %q: %w", host.InventoryPath, err)
		}
		net := moHost.Config.Network
		if net == nil {
			continue
		}
		for _, pg := range net.Portgroup {
			if pg.Spec.Name != name {
				continue
			}
			targets = append(targets, pgTarget{
				name:   pg.Spec.Name,
				kind:   "standard",
				netRef: strings.TrimPrefix(pg.Key, "key-vim.host."),
			})
		}
	}

	dvsTargets, err := distributedPortGroups(ctx, client, name)
	if err != nil {
		return nil, err
	}
	targets = append(targets, dvsTargets...)
	return targets, nil
}

// distributedPortGroups returns distributed port groups matching name.
func distributedPortGroups(ctx context.Context, client *vim25.Client, name string) ([]pgTarget, error) {
	dvsList, err := distributedSwitches(ctx, client)
	if err != nil {
		return nil, err
	}

	pc := property.DefaultCollector(client)
	var targets []pgTarget
	for _, dvs := range dvsList {
		for _, pgRef := range dvs.Portgroup {
			var pg mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(ctx, pgRef, nil, &pg); err != nil {
				return nil, fmt.Errorf("reading port group: %w", err)
			}
			if pg.Config.Name != name {
				continue
			}
			targets = append(targets, pgTarget{
				name:    pg.Config.Name,
				kind:    "distributed",
				dvsUuid: dvs.Uuid,
				pgKey:   pg.Key,
			})
		}
	}
	return targets, nil
}

// vmConnectedTo reports whether any virtual NIC of the VM is connected to the
// given port group target.
func vmConnectedTo(moVM *mo.VirtualMachine, tgt pgTarget) bool {
	for _, dev := range moVM.Config.Hardware.Device {
		card, ok := dev.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		switch b := card.GetVirtualEthernetCard().Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if tgt.kind != "standard" {
				continue
			}
			// Prefer the port group name (DeviceName), which is populated even when
			// the backing uses auto-detect (Network is nil); fall back to the network
			// reference for clients that expose it.
			if b.DeviceName != "" && b.DeviceName == tgt.name {
				return true
			}
			if b.Network != nil && b.Network.String() == tgt.netRef {
				return true
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			if tgt.kind == "distributed" &&
				b.Port.PortgroupKey == tgt.pgKey &&
				(tgt.dvsUuid == "" || b.Port.SwitchUuid == tgt.dvsUuid) {
				return true
			}
		}
	}
	return false
}
