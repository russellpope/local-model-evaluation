// Package cli wires Cobra subcommands to the vsphere collectors and the
// tabwriter presentation.
package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/vmware/govmomi"

	"github.com/example/govc-inventory/internal/config"
	"github.com/example/govc-inventory/internal/format"
	"github.com/example/govc-inventory/internal/vsphere"
)

// NewRootCmd builds the root command with its three subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "govc-inventory",
		Short:         "Report vSphere virtualization inventory (vms, datastores, vswitches)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	vms := &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines with configured vCPU, RAM, and consumed storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withClient(cmd, func(ctx context.Context, c *govmomi.Client) error {
				vms, err := vsphere.ListVMs(ctx, c)
				if err != nil {
					return err
				}
				return writeVMs(cmd.OutOrStdout(), vms)
			})
		},
	}

	datastores := &cobra.Command{
		Use:   "datastores",
		Short: "List datastores with transport, used and available capacity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withClient(cmd, func(ctx context.Context, c *govmomi.Client) error {
				dss, err := vsphere.ListDatastores(ctx, c)
				if err != nil {
					return err
				}
				return writeDatastores(cmd.OutOrStdout(), dss)
			})
		},
	}

	var pgName string
	vswitches := &cobra.Command{
		Use:   "vswitches",
		Short: "List standard and distributed virtual switches with port groups; --portgroup lists connected VMs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withClient(cmd, func(ctx context.Context, c *govmomi.Client) error {
				if pgName != "" {
					vms, err := vsphere.VMsOnPortGroup(ctx, c, pgName)
					if err != nil {
						return err
					}
					return writeVMs(cmd.OutOrStdout(), vms)
				}
				rows, err := vsphere.ListSwitches(ctx, c)
				if err != nil {
					return err
				}
				return writeSwitches(cmd.OutOrStdout(), rows)
			})
		},
	}

	addConnFlags(vms.Flags())
	addConnFlags(datastores.Flags())
	addConnFlags(vswitches.Flags())
	vswitches.Flags().StringVar(&pgName, "portgroup", "", "list VMs connected to this port group instead of the switch table")

	root.AddCommand(vms, datastores, vswitches)
	return root
}

// newConnFlagSet builds the connection flag set shape (used by NewRootCmd
// and tests).
func newConnFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("conn", pflag.ContinueOnError)
	addConnFlags(fs)
	return fs
}

func addConnFlags(fs *pflag.FlagSet) {
	fs.String(config.KeyURL, "", "vSphere endpoint URL or host, e.g. https://vc.lab/sdk")
	fs.String(config.KeyUsername, "", "vSphere username")
	fs.String(config.KeyPassword, "", "vSphere password")
	fs.Bool(config.KeyInsecure, false, "skip TLS certificate verification")
	fs.String(config.KeyTimeout, "60s", "overall operation timeout (e.g. 30s, 5m)")
	fs.String(config.KeyConfig, "", "optional path to a YAML config file")
}

// resolveConfig applies the config file (if any) then flag/env/default
// precedence through viper.
func resolveConfig(fs *pflag.FlagSet) (config.Settings, error) {
	v, err := config.NewViper(fs)
	if err != nil {
		return config.Settings{}, err
	}
	if err := config.ReadFile(v, v.GetString(config.KeyConfig)); err != nil {
		return config.Settings{}, err
	}
	return config.Load(v)
}

// withClient connects using resolved config and runs fn with a context
// bounded by the configured timeout. Logout is deferred.
func withClient(cmd *cobra.Command, fn func(context.Context, *govmomi.Client) error) error {
	s, err := resolveConfig(cmd.Flags())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), s.Timeout)
	defer cancel()

	u, err := parseURL(s.URL, s.Username, s.Password)
	if err != nil {
		return err
	}

	c, err := govmomi.NewClient(ctx, u, s.Insecure)
	if err != nil {
		return fmt.Errorf("connect to %s: %w (auth/connection failure: check --url/--username/--password or VSPHERE_* env)", u.Redacted(), err)
	}
	defer func() {
		// Best-effort logout with a fresh short context: the request ctx may
		// already be cancelled at this point.
		logoutCtx, lcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer lcancel()
		if err := c.Logout(logoutCtx); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: logout failed: %v\n", err)
		}
	}()

	if !c.IsVC() {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: connected endpoint is not a vCenter; distributed switch data may be unavailable")
	}

	return fn(ctx, c)
}

// parseURL normalises the configured endpoint: bare hosts get https:// and
// the /sdk path; credentials fold into the URL for govmomi's login path.
func parseURL(raw, user, pass string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("no vSphere URL configured: set --url or VSPHERE_URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url %q: %w", raw, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url %q has no host", raw)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	if u.User == nil {
		if user == "" {
			return nil, fmt.Errorf("no username configured: set --username or VSPHERE_USERNAME")
		}
		u.User = url.UserPassword(user, pass)
	}
	return u, nil
}

// newTabWriter builds a left-aligned, space-separated table writer.
func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
}

func writeVMs(w io.Writer, vms []vsphere.VMInfo) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", vm.Name, vm.VCPU, format.GB(vm.RAMMB), format.Bytes(vm.CommittedStorage))
	}
	return tw.Flush()
}

func writeDatastores(w io.Writer, dss []vsphere.DatastoreInfo) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		used := format.Used(ds.Capacity, ds.Free)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ds.Name, ds.Transport, format.Bytes(used), format.Bytes(ds.Free))
	}
	return tw.Flush()
}

func writeSwitches(w io.Writer, rows []vsphere.PortGroupInfo) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, r := range rows {
		uplinks := strings.Join(r.Uplinks, ",")
		if uplinks == "" {
			uplinks = "-" // switch exposes no uplink detail (e.g. simulator)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			r.SwitchName, r.SwitchType, r.Name, r.VLAN,
			uplinks, r.LACP, r.NumPorts, r.UsedPorts)
	}
	return tw.Flush()
}
