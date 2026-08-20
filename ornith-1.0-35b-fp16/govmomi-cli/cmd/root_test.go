package cmd

import (
	"strings"
	"testing"
)

// TestBindFlags_FlagOverridesEnv verifies that --url on the CLI takes precedence
// over the VSPHERE_URL environment variable. This is the regression test for
// the bindFlags bug: when bindFlags looked up flags on cmd.PersistentFlags()
// (empty on subcommands), every Lookup returned nil, BindPFlag no-op'd, and the
// env value won unchallenged. The fix looks up on cmd.Flags() (the merged set
// that includes inherited persistent flags).
//
// We cannot assert a successful connection in a unit test (no real vCenter), but
// we can assert which URL the CLI attempted to connect to: if --url won, the
// connection error will reference the flag's host; if env won, it references the
// env host.
func TestBindFlags_FlagOverridesEnv(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://env-host.example.com/sdk")

	rootCmd.SetArgs([]string{"--url", "https://flag-host.example.com/sdk", "vms"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected connection error (no real vCenter); got nil")
	}

	errStr := err.Error()
	if strings.Contains(errStr, "env-host.example.com") {
		t.Errorf("CLI connected to env-host, meaning VSPHERE_URL env won over --url flag. Error: %s", errStr)
	}
	if !strings.Contains(errStr, "flag-host.example.com") {
		t.Errorf("CLI did not attempt to connect to flag-host. Expected --url to take precedence. Error: %s", errStr)
	}
}
