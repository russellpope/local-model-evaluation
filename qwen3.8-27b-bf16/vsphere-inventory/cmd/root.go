package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/config"
)

const logoutTimeout = 10 * time.Second

var cmdErrWriter = os.Stderr

var rootCmd = &cobra.Command{
	Use:           "vsphere-inventory",
	Short:         "Report vSphere virtualization inventory",
	Long:          "Connects to a vCenter Server and reports virtualization inventory: virtual machines, datastores, and virtual switches.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	fs := rootCmd.PersistentFlags()
	fs.String("url", "", "vCenter URL or host (e.g. https://vc.lab/sdk or vc.lab)")
	fs.String("username", "", "vCenter username")
	fs.String("password", "", "vCenter password")
	fs.Bool("insecure", false, "skip TLS certificate verification")
	fs.Duration("timeout", 0, "overall operation timeout (e.g. 60s)")
	fs.String("config", "", "path to a YAML config file")

	rootCmd.AddCommand(vmsCmd, datastoresCmd, vswitchesCmd)
}

// resolveConfig builds the Viper layering (defaults < config file < env <
// flags) and resolves the final configuration.
func resolveConfig(cmd *cobra.Command) (config.Config, error) {
	cfgFile, _ := cmd.Flags().GetString("config")
	v, err := config.NewViper(cfgFile)
	if err != nil {
		return config.Config{}, err
	}
	if err := config.BindFlags(v, cmd.Flags()); err != nil {
		return config.Config{}, err
	}
	return config.Load(v)
}

// parseVCenterURL normalizes the configured URL: adds the https scheme when
// missing and the /sdk path when missing.
func parseVCenterURL(s string) (*url.URL, error) {
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid vCenter URL %q: %w", s, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported URL scheme %q in %q (use https or http)", u.Scheme, s)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	return u, nil
}

// runWithClient connects once, derives the operation context from the
// configured timeout, defers a clean logout, and runs fn.
func runWithClient(cfg config.Config, fn func(ctx context.Context, client *vim25.Client) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	u, err := parseVCenterURL(cfg.URL)
	if err != nil {
		return err
	}
	if cfg.Username != "" {
		// govmomi.NewClient only logs in when the URL carries userinfo, so
		// attach the configured credentials explicitly.
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return fmt.Errorf("connect to %s as %q: %w (check the URL and credentials; use --insecure for self-signed certificates)", u.Host, cfg.Username, err)
	}
	defer func() {
		lctx, lcancel := context.WithTimeout(context.Background(), logoutTimeout)
		defer lcancel()
		if err := client.Logout(lctx); err != nil {
			fmt.Fprintln(cmdErrWriter, "warning: logout failed:", err)
		}
	}()

	return fn(ctx, client.Client)
}
