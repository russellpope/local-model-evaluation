package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/config"
)

type Client struct {
	*vim25.Client
	*find.Finder
}

func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vCenter URL %q: %w", cfg.URL, err)
	}

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vCenter %s: %w", cfg.URL, err)
	}

	client.Timeout = cfg.Timeout

	vimClient := client.Client
	if vimClient == nil {
		return nil, fmt.Errorf("failed to get vim25 client")
	}

	finder := find.NewFinder(vimClient, false)

	return &Client{
		Client: vimClient,
		Finder: finder,
	}, nil
}

func (c *Client) Logout(ctx context.Context) error {
	if c.Client != nil {
		sm := session.NewManager(c.Client)
		return sm.Logout(ctx)
	}
	return nil
}

func init() {
	_ = fmt.Errorf
}
