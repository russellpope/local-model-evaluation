package vsphere

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"

	"vsphere-inventory/internal/config"
)

// Connect establishes an authenticated session to vCenter and returns a client.
// The caller must call client.Logout() when done.
func Connect(ctx context.Context, c *config.Config) (*govmomi.Client, error) {
	if c.URL == "" {
		return nil, fmt.Errorf("vSphere URL is required (set --url, VSPHERE_URL env var, or url in config file)")
	}

	u, err := soap.ParseURL(c.URL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", c.URL, err)
	}

	var userinfo *url.Userinfo
	if c.Username != "" && c.Password != "" {
		userinfo = url.UserPassword(c.Username, c.Password)
		u.User = userinfo
	}

	client, err := govmomi.NewClient(ctx, u, c.Insecure)
	if err != nil {
		return nil, fmt.Errorf("create vSphere client: %w", err)
	}

	return client, nil
}
