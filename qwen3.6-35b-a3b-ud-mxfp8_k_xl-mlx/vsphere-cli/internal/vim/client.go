package vim

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/local-model-evaluation/vsphere-cli/internal/config"
)

type Client struct {
	*govmomi.Client
	Finder *find.Finder
	View   *view.Manager
}

func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing vCenter URL: %w", err)
	}

	if cfg.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password is required")
	}

	soapClient := soap.NewClient(u, cfg.Insecure)
	if cfg.Insecure {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		soapClient.Client.Transport = tr
	}

	vim25Client, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("creating vim25 client: %w", err)
	}

	sm := session.NewManager(vim25Client)

	client := &govmomi.Client{
		Client:         vim25Client,
		SessionManager: sm,
	}

	if err := client.Login(ctx, url.UserPassword(cfg.Username, cfg.Password)); err != nil {
		return nil, fmt.Errorf("logging in to vCenter: %w", err)
	}

	finder := find.NewFinder(client.Client, false)
	viewManager := view.NewManager(client.Client)

	return &Client{Client: client, Finder: finder, View: viewManager}, nil
}

func (c *Client) Logout(ctx context.Context) error {
	return c.Client.Logout(ctx)
}
