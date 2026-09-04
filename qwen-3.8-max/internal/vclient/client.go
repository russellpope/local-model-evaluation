package vclient

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/config"
)

func ParseURL(raw, username, password string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q (want http or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("missing host")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	if u.User == nil && username != "" {
		u.User = url.UserPassword(username, password)
	}
	return u, nil
}

func Connect(ctx context.Context, cfg *config.Config) (*govmomi.Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("no vCenter URL configured: set --url, VSPHERE_URL, or url in the config file")
	}
	if cfg.Username == "" {
		return nil, errors.New("no vCenter username configured: set --username, VSPHERE_USERNAME, or username in the config file")
	}
	u, err := ParseURL(cfg.URL, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL %q: %w", cfg.URL, err)
	}

	c, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, connectError(err, u, cfg)
	}
	return c, nil
}

func connectError(err error, u *url.URL, cfg *config.Config) error {
	if f := soapFault(err); f != nil {
		switch f.Detail.Fault.(type) {
		case types.InvalidLogin, *types.InvalidLogin:
			return fmt.Errorf("authentication failed for user %q on %s: check username and password (--username/--password or VSPHERE_USERNAME/VSPHERE_PASSWORD)", cfg.Username, u.Host)
		}
	}
	return fmt.Errorf("connect to vCenter %s: %w", u.Host, hint(err, cfg))
}

func soapFault(err error) *soap.Fault {
	for err != nil {
		if soap.IsSoapFault(err) {
			return soap.ToSoapFault(err)
		}
		err = errors.Unwrap(err)
	}
	return nil
}

func hint(err error, cfg *config.Config) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w (operation timeout of %s exceeded; raise --timeout/VSPHERE_TIMEOUT)", err, cfg.Timeout)
	}
	if !cfg.Insecure && (soap.IsCertificateUntrusted(err) || isHostnameError(err)) {
		return fmt.Errorf("%w (server certificate is not trusted or does not match the host; verify the URL or connect with --insecure/VSPHERE_INSECURE=true)", err)
	}
	return err
}

func isHostnameError(err error) bool {
	var hostname x509.HostnameError
	return errors.As(err, &hostname)
}
