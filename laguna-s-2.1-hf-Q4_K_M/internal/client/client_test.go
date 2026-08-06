package client

import (
	"context"
	"testing"
)

func TestNewInvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := New(ctx, "://invalid-url", "user", "pass", false)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestNewMissingCredentials(t *testing.T) {
	ctx := context.Background()
	_, err := New(ctx, "https://vc.example.com/sdk", "", "", false)
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
}

func TestNewMissingPassword(t *testing.T) {
	ctx := context.Background()
	_, err := New(ctx, "https://vc.example.com/sdk", "user", "", false)
	if err == nil {
		t.Fatal("expected error for missing password, got nil")
	}
}

func TestNewMissingUsername(t *testing.T) {
	ctx := context.Background()
	_, err := New(ctx, "https://vc.example.com/sdk", "", "pass", false)
	if err == nil {
		t.Fatal("expected error for missing username, got nil")
	}
}

func TestLogoutNilClient(t *testing.T) {
	c := &Client{APIClient: nil}
	err := c.Logout(context.Background())
	if err != nil {
		t.Errorf("Logout with nil APIClient should return nil, got %v", err)
	}
}

func TestVim25NilClient(t *testing.T) {
	c := &Client{APIClient: nil}
	if c.Vim25() != nil {
		t.Error("Vim25 with nil APIClient should return nil")
	}
}
