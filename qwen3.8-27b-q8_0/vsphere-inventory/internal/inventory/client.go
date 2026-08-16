// Package inventory contains the vSphere inventory retrieval logic, kept
// separate from the Cobra wiring and the tabwriter presentation so each
// feature can be tested directly against a client.
package inventory

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"

	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
)

// Connect establishes an authenticated client to the vCenter described by
// cfg. The caller owns the returned client and must Logout when done.
func Connect(ctx context.Context, cfg config.Config) (*govmomi.Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("no vCenter URL configured: set --url, VSPHERE_URL, or url in the config file")
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL %q: %w", cfg.URL, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("invalid vCenter URL %q: scheme must be http or https", cfg.URL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid vCenter URL %q: missing host", cfg.URL)
	}

	if u.User == nil && (cfg.Username != "" || cfg.Password != "") {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}
	if u.User == nil {
		return nil, fmt.Errorf("no credentials for %s: set --username/--password, VSPHERE_USERNAME/VSPHERE_PASSWORD, or embed them in the URL", cfg.URL)
	}

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("connect to vCenter %s: %w", cfg.URL, err)
	}

	return client, nil
}
