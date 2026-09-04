// Package client establishes authenticated govmomi connections.
package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
)

// ParseURL normalizes a vCenter URL or bare host. A missing scheme becomes
// https, and a missing or bare path becomes the /sdk endpoint.
func ParseURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("vSphere URL is empty: set --url or %s_URL (e.g. https://vc.lab/sdk)", config.EnvPrefix)
	}
	input := raw
	if !strings.Contains(input, "://") {
		input = "https://" + input
	}
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: missing host", raw)
	}
	switch u.Path {
	case "", "/":
		u.Path = "/sdk"
	default:
		if !strings.HasSuffix(u.Path, "/sdk") && !strings.Contains(u.Path, "/sdk/") {
			u.Path = strings.TrimSuffix(u.Path, "/") + "/sdk"
		}
	}
	return u, nil
}

// Connect creates and authenticates a govmomi client against vCenter.
func Connect(ctx context.Context, s config.Settings) (*govmomi.Client, error) {
	u, err := ParseURL(s.URL)
	if err != nil {
		return nil, err
	}
	if u.User == nil && (s.Username == "" || s.Password == "") {
		return nil, fmt.Errorf("username and password are required: set --username/--password or %s_USERNAME/%s_PASSWORD (or embed them in the URL)", config.EnvPrefix, config.EnvPrefix)
	}

	c, err := govmomi.NewClient(ctx, u, s.Insecure)
	if err != nil {
		return nil, describeConnectError(err, u, s)
	}

	// NewClient authenticates only when the URL carries userinfo; log in
	// explicitly with the configured credentials otherwise.
	if s.Username != "" || s.Password != "" {
		if err := c.Login(ctx, url.UserPassword(s.Username, s.Password)); err != nil {
			return nil, describeConnectError(err, u, s)
		}
	}
	return c, nil
}

func describeConnectError(err error, u *url.URL, s config.Settings) error {
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return fmt.Errorf("TLS verification failed for %s: use --insecure (or %s_INSECURE=true) for self-signed certificates: %w", u.Host, config.EnvPrefix, err)
	}
	if strings.Contains(err.Error(), "x509") {
		return fmt.Errorf("TLS verification failed for %s: use --insecure (or %s_INSECURE=true) for self-signed certificates: %w", u.Host, config.EnvPrefix, err)
	}
	var login types.InvalidLogin
	if fault.Is(err, &login) {
		return fmt.Errorf("authentication failed for %q@%s: check username and password (--username/--password or %s_USERNAME/%s_PASSWORD): %w", s.Username, u.Host, config.EnvPrefix, config.EnvPrefix, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("connection to vCenter at %s timed out: raise --timeout (or %s_TIMEOUT, currently %s): %w", u.Host, config.EnvPrefix, s.Timeout, err)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("connection to vCenter at %s was canceled: %w", u.Host, err)
	}
	var netErr interface{ Timeout() bool }
	isNet := errors.As(err, &netErr)
	var dnsErr *net.DNSError
	var opErr *net.OpError
	if errors.As(err, &dnsErr) || errors.As(err, &opErr) || isNet {
		return fmt.Errorf("cannot reach vCenter at %s: check %s_URL and network connectivity: %w", u.Host, config.EnvPrefix, err)
	}
	return fmt.Errorf("failed to connect to vCenter at %s: %w", u.Host, err)
}
