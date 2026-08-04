package config

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

func NewClient(ctx context.Context, c *Config) (*vim25.Client, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL %q: %w", c.URL, err)
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), "443")
	}

	soapClient := soap.NewClient(u, c.Insecure)
	client, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("connecting to vSphere: %w", err)
	}

	if c.Username != "" && c.Password != "" {
		sm := session.NewManager(client)
		if err := sm.Login(ctx, url.UserPassword(c.Username, c.Password)); err != nil {
			return nil, fmt.Errorf("authenticating to vSphere: %w", err)
		}
	}

	return client, nil
}

func Logout(ctx context.Context, client *vim25.Client) {
	sm := session.NewManager(client)
	_ = sm.Logout(ctx)
}
