package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/config"
)

func New(ctx context.Context, cfg *config.Config) (*vim25.Client, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)
	c, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	c.Client.Timeout = cfg.Timeout
	return c.Client, nil
}

func Logout(ctx context.Context, c *vim25.Client) {
}
