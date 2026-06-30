package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/view"
)

func getVMView(ctx context.Context, c *vim25.Client) (*view.ContainerView, error) {
	mgr := view.NewManager(c)
	
	var typeFilter []string = []string{"VirtualMachine"}
	rootFolder := c.ServiceContent.RootFolder
	
	v, err := mgr.CreateContainerView(ctx, rootFolder, typeFilter, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	
	return v, nil
}

func getDatastoreView(ctx context.Context, c *vim25.Client) (*view.ContainerView, error) {
	mgr := view.NewManager(c)
	
	var typeFilter []string = []string{"Datastore"}
	rootFolder := c.ServiceContent.RootFolder
	
	v, err := mgr.CreateContainerView(ctx, rootFolder, typeFilter, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	
	return v, nil
}

func getNetworkView(ctx context.Context, c *vim25.Client) (*view.ContainerView, error) {
	mgr := view.NewManager(c)
	
	var typeFilter []string = []string{"HostNetwork", "DistributedVirtualSwitch", "DistributedVirtualPortgroup"}
	rootFolder := c.ServiceContent.RootFolder
	
	v, err := mgr.CreateContainerView(ctx, rootFolder, typeFilter, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	
	return v, nil
}

func getHostView(ctx context.Context, c *vim25.Client) (*view.ContainerView, error) {
	mgr := view.NewManager(c)
	
	var typeFilter []string = []string{"HostSystem"}
	rootFolder := c.ServiceContent.RootFolder
	
	v, err := mgr.CreateContainerView(ctx, rootFolder, typeFilter, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	
	return v, nil
}
