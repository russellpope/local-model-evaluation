package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/config"
)

// faultGetter is implemented by govmomi's soap fault errors, exposing the
// decoded vim fault so failures can be translated into actionable messages.
type faultGetter interface {
	Fault() types.BaseMethodFault
}

// connect resolves the configuration, builds a context bounded by the
// configured timeout, opens one authenticated client, and runs fn with it.
// The client is logged out when fn returns.
func connect(cmd *cobra.Command, fn func(ctx context.Context, c *vim25.Client) error) error {
	cfg, err := config.Load(cmd.Root().PersistentFlags())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer cancel()

	client, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout(context.WithoutCancel(ctx)) }()

	return fn(ctx, client.Client)
}

// dial opens an authenticated govmomi session against cfg.URL.
func dial(ctx context.Context, cfg *config.Config) (*govmomi.Client, error) {
	u, err := endpointURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		var fg faultGetter
		if errors.As(err, &fg) {
			if _, ok := fg.Fault().(*types.InvalidLogin); ok {
				return nil, fmt.Errorf(
					"authentication failed for user %q at %s: check --username/--password (or VSPHERE_USERNAME/VSPHERE_PASSWORD): %w",
					cfg.Username, cfg.URL, err)
			}
		}
		return nil, fmt.Errorf(
			"could not connect to vCenter at %s: verify --url and network; use --insecure for self-signed certificates: %w",
			cfg.URL, err)
	}
	return client, nil
}

// endpointURL normalizes the configured URL: it defaults the scheme to
// https for bare hostnames and appends the /sdk path when missing.
func endpointURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		u, err = url.Parse("https://" + raw)
		if err != nil {
			return nil, fmt.Errorf("invalid vCenter URL %q: %w", raw, err)
		}
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid vCenter URL %q: missing host", raw)
	}
	if !strings.HasSuffix(u.Path, "/sdk") {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/sdk"
	}
	return u, nil
}
