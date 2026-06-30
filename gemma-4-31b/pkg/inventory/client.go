package inventory

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"govmomi-cli/pkg/config"
)

func NewClient(ctx context.Context, cfg *config.Config) (*govmomi.Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("vCenter URL is required")
	}

	// Construct URL with credentials
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if cfg.Username != "" {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vCenter: %w", err)
	}

	return client, nil
}
