package config

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

func NewClient(ctx context.Context, c *Config) (*vim25.Client, error) {
	u, err := soap.ParseURL(c.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL %q: %w", c.URL, err)
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
	if err := sm.Logout(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: logout failed: %v\n", err)
	}
}
