// Package inventory retrieves typed vSphere inventory data through a
// *vim25.Client. Every function takes a context.Context and returns typed,
// presentation-free results so the logic can be unit-tested against the in
// process govmomi simulator without going through Cobra or tabwriter.
package inventory

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

// Connect dials and authenticates against the vCenter at serverURL. A URL
// without an explicit scheme is treated as https. username/password may be
// empty. The caller is responsible for Client.Logout.
func Connect(ctx context.Context, serverURL, username, password string, insecure bool) (*govmomi.Client, error) {
	if !strings.Contains(serverURL, "://") {
		serverURL = "https://" + serverURL
	}

	u, err := soap.ParseURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL %q: %w", serverURL, err)
	}
	if username == "" {
		u.User = nil
	} else {
		u.User = url.UserPassword(username, password)
	}

	c, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("connect to vCenter %s (user %q): %w", serverURL, username, err)
	}
	return c, nil
}
