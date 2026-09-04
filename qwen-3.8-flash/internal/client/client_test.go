package client

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"

	"local-model-evaluation/govmomi-cli/internal/config"
)

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://vc.lab/sdk", "https://vc.lab/sdk"},
		{"vc.lab", "https://vc.lab/sdk"},
		{"http://127.0.0.1:8989/sdk", "http://127.0.0.1:8989/sdk"},
		{"127.0.0.1:8989", "https://127.0.0.1:8989/sdk"},
		{"https://vc.lab/", "https://vc.lab/sdk"},
		{"https://vc.lab", "https://vc.lab/sdk"},
	}
	for _, tt := range tests {
		u, err := ParseEndpoint(tt.in)
		if err != nil {
			t.Fatalf("ParseEndpoint(%q): %v", tt.in, err)
		}
		if got := u.String(); got != tt.want {
			t.Errorf("ParseEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if _, err := ParseEndpoint(""); err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestConnectUnreachableHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := config.Values{URL: "https://127.0.0.1:1/sdk", Username: "u", Password: "p", Insecure: true}
	if _, err := Connect(ctx, cfg); err == nil {
		t.Fatal("expected connection failure")
	} else if !strings.Contains(err.Error(), "cannot reach vCenter") {
		t.Errorf("error not actionable: %v", err)
	}
}

func TestConnectBadCredentials(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	s := model.Service.NewServer()
	defer s.Close()
	// vcsim accepts any non-empty credentials by default; pin the expected
	// password so the InvalidLogin path is actually exercised.
	model.Service.Listen.User = url.UserPassword("user", "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg := config.Values{URL: s.URL.String(), Username: "user", Password: "wrong", Insecure: true}
	_, err := Connect(ctx, cfg)
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error not actionable: %v", err)
	}
}

func TestConnectAndLogoutAgainstSimulator(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	s := model.Service.NewServer()
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg := config.Values{URL: s.URL.String(), Username: "user", Password: "pass", Insecure: true}
	cl, err := Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	sm := session.NewManager(cl.Vim())
	info, err := sm.UserSession(ctx)
	if err != nil {
		t.Fatalf("session not active after login: %v", err)
	}
	if info.UserName != "user" {
		t.Errorf("session user = %q, want user", info.UserName)
	}
	cl.Logout()
	if active, err := sm.SessionIsActive(ctx); err == nil && active {
		t.Error("session still active after logout")
	}
}
