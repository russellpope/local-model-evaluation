package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
)

type Client struct {
	APIClient *govmomi.Client
}

func New(ctx context.Context, vCenterURL, username, password string, insecure bool) (*Client, error) {
	u, err := url.Parse(vCenterURL)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL %q: %w", vCenterURL, err)
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	if u.Path == "" {
		u.Path = "/sdk"
	}

	if username != "" && password != "" {
		u.User = url.UserPassword(username, password)
	} else if u.User == nil {
		return nil, fmt.Errorf("no credentials provided: set username/password via --username/--password flags, VSPHERE_USERNAME/VSPHERE_PASSWORD env vars, or config file")
	}

	apiClient, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("connect to vCenter at %s: %w", vCenterURL, err)
	}

	return &Client{APIClient: apiClient}, nil
}

func (c *Client) Logout(ctx context.Context) error {
	if c.APIClient == nil {
		return nil
	}
	if err := c.APIClient.Logout(ctx); err != nil {
		return fmt.Errorf("logout from vCenter: %w", err)
	}
	return nil
}

func (c *Client) Vim25() *vim25.Client {
	return c.APIClient.Client
}
