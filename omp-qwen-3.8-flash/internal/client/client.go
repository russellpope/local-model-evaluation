// Package client establishes the authenticated govmomi connection used by
// all subcommands.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/ldh/vsphere-inventory/internal/config"
)

// Conn is an authenticated govmomi client plus the operation context derived
// from the configured timeout. Close logs out and releases the context.
type Conn struct {
	*govmomi.Client
	Ctx    context.Context
	cancel context.CancelFunc
}

// Vim returns the underlying vim25 client.
func (c *Conn) Vim() *vim25.Client { return c.Client.Client }

// Close logs out of vCenter and cancels the operation context. It is safe to
// call more than once; the first call's logout error is returned.
func (c *Conn) Close() error {
	defer c.cancel()
	if c.SessionManager == nil {
		return nil
	}
	return c.Logout(c.Ctx)
}

// New builds a govmomi client from cfg: normalises the URL, creates a
// context bounded by cfg.Timeout, and logs in with basic authentication.
func New(ctx context.Context, cfg config.Config) (*Conn, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("no vCenter URL configured: set --url, VSPHERE_URL, or url in the config file")
	}
	u, err := ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vCenter URL %q: %w", cfg.URL, err)
	}
	if cfg.Username == "" {
		return nil, errors.New("no username configured: set --username or VSPHERE_USERNAME")
	}

	opCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)

	soapClient := soap.NewClient(u, cfg.Insecure)
	vimClient, err := vim25.NewClient(opCtx, soapClient)
	if err != nil {
		cancel()
		return nil, connectionError(u, cfg.Insecure, err)
	}

	c := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	user := url.UserPassword(cfg.Username, cfg.Password)
	if err := c.Login(opCtx, user); err != nil {
		cancel()
		return nil, authError(u, cfg.Username, err)
	}

	return &Conn{Client: c, Ctx: opCtx, cancel: cancel}, nil
}

// ParseURL normalises a vCenter endpoint: a bare host, host/sdk, or full URL
// all resolve to https://host/sdk. Any credentials embedded in the URL are
// stripped; authentication uses the configured username/password.
func ParseURL(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, errors.New("missing host")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	u.User = nil
	return u, nil
}

// connectionError adds actionable context to transport-level failures.
func connectionError(u *url.URL, insecure bool, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "certificate") || strings.Contains(msg, "x509") {
		hint := ""
		if !insecure {
			hint = " (retry with --insecure or VSPHERE_INSECURE=true for self-signed certificates)"
		}
		return fmt.Errorf("connecting to %s: TLS verification failed%s: %w", u, hint, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("connecting to %s timed out: check the URL, network, and --timeout: %w", u, err)
	}
	return fmt.Errorf("connecting to %s: %w", u, err)
}

// authError distinguishes rejected credentials from generic login failures.
func authError(u *url.URL, username string, err error) error {
	if strings.Contains(err.Error(), "InvalidLogin") || strings.Contains(err.Error(), "Cannot complete login") {
		return fmt.Errorf("login to %s as %q failed: invalid username or password: %w", u, username, err)
	}
	return fmt.Errorf("login to %s as %q failed: %w", u, username, err)
}
