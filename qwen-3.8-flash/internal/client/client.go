// Package client establishes authenticated govmomi connections.
package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"local-model-evaluation/govmomi-cli/internal/config"
)

// Client wraps an authenticated govmomi client with a logout closure.
type Client struct {
	*govmomi.Client
	Logout func()
}

// Vim returns the underlying vim25 client used by inventory queries.
func (c *Client) Vim() *vim25.Client {
	return c.Client.Client
}

// ParseEndpoint normalizes a user-supplied URL or bare host into a vCenter
// SDK endpoint, e.g. "vc.lab" becomes "https://vc.lab/sdk".
func ParseEndpoint(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty vSphere URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse vSphere URL %q: %w", raw, err)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	return u, nil
}

// Connect logs in to vCenter using the resolved configuration and the
// supplied (already time-budgeted) context. The caller must invoke Logout.
func Connect(ctx context.Context, cfg config.Values) (*Client, error) {
	u, err := ParseEndpoint(cfg.URL)
	if err != nil {
		return nil, err
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	gc, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, describeConnectError(ctx, u, cfg.Username, err)
	}
	return &Client{
		Client: gc,
		Logout: func() {
			// Best-effort logout on a fresh context: the operation context may
			// already be canceled or expired by the time we get here.
			_ = gc.Logout(context.Background())
		},
	}, nil
}

// describeConnectError turns raw login/connection failures into actionable
// messages while preserving the underlying error for debugging.
func describeConnectError(ctx context.Context, u *url.URL, user string, err error) error {
	msg := err.Error()
	if ctx.Err() != nil {
		return fmt.Errorf("connecting to vCenter at %s: %w (operation timed out or was canceled)", u.Host, ctx.Err())
	}
	if soap.IsSoapFault(err) {
		switch soap.ToSoapFault(err).Detail.Fault.(type) {
		case *types.InvalidLogin, types.InvalidLogin:
			return fmt.Errorf("authentication failed for user %q at %s: check VSPHERE_USERNAME/VSPHERE_PASSWORD (server said: %w)", user, u.Host, err)
		}
	}
	if soap.IsVimFault(err) {
		if _, ok := soap.ToVimFault(err).(*types.InvalidLogin); ok {
			return fmt.Errorf("authentication failed for user %q at %s: check VSPHERE_USERNAME/VSPHERE_PASSWORD (server said: %w)", user, u.Host, err)
		}
	}
	switch {
	case strings.Contains(msg, "InvalidLogin"), strings.Contains(msg, "Cannot complete login"):
		return fmt.Errorf("authentication failed for user %q at %s: check VSPHERE_USERNAME/VSPHERE_PASSWORD (server said: %w)", user, u.Host, err)
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "x509"):
		return fmt.Errorf("TLS verification failed for %s: the server uses an untrusted certificate; set --insecure (or VSPHERE_INSECURE=true) to skip verification: %w", u.Host, err)
	case soap.IsSoapFault(err) || soap.IsVimFault(err):
		return fmt.Errorf("vCenter %s rejected the request: %w", u.Host, err)
	default:
		return fmt.Errorf("cannot reach vCenter at %s: %w", u.Host, err)
	}
}
