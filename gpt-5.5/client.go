package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
)

func NewClient(ctx context.Context, cfg Config) (*govmomi.Client, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("vCenter URL is required; set --url or VSPHERE_URL")
	}

	u, err := normalizeURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	if cfg.Username != "" {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("connect and authenticate to %s: %w", redactedURL(u), err)
	}
	return client, nil
}

func normalizeURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	u, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("parse vCenter URL %q: host is empty", raw)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	return u, nil
}

func redactedURL(u *url.URL) string {
	copy := *u
	copy.User = nil
	return copy.String()
}
