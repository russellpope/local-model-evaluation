package inventory

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"

	"vsphere-inventory/internal/config"
)

// Connect builds an authenticated govmomi client from cfg. Callers must
// eventually invoke Logout on the returned client.
func Connect(ctx context.Context, cfg *config.Config) (*govmomi.Client, error) {
	u, err := soap.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vCenter URL %q: %w (expected e.g. https://vc.lab/sdk)", cfg.URL, err)
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	c, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("connect to %s (insecure=%t): %w; verify the URL is reachable and TLS settings are correct (--insecure for self-signed certificates)", cfg.URL, cfg.Insecure, err)
	}
	// NewClient already logs in when the URL carries credentials; only log
	// in explicitly when that did not happen.
	if !c.Valid() {
		if err := c.Login(ctx, u.User); err != nil {
			_ = c.Logout(context.Background())
			return nil, fmt.Errorf("login to %s as %q: %w; check --username/--password (or VSPHERE_USERNAME/VSPHERE_PASSWORD)", cfg.URL, cfg.Username, err)
		}
	}
	return c, nil
}
