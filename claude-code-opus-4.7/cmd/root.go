// Package cmd wires the Cobra command tree, the shared vCenter connection, and
// the tabwriter presentation for the three inventory subcommands.
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"vsphere-inventory/internal/config"
	"vsphere-inventory/internal/inventory"
)

const logoutTimeout = 10 * time.Second

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "Report VMware vSphere inventory (VMs, datastores, virtual switches)",
	Long: "vsphere-inventory connects to a VMware vCenter Server and reports " +
		"virtualization inventory: virtual machines, datastores, and virtual switches.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var portgroupFlag string

// Execute runs the root command with the given context (used for signal-driven
// cancellation) and returns any error to the caller.
func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	config.RegisterFlags(rootCmd)

	vswitchesCmd.Flags().StringVar(&portgroupFlag, "portgroup", "",
		"list the VMs connected to this port group instead of listing switches")

	rootCmd.AddCommand(vmsCmd, datastoresCmd, vswitchesCmd)
}

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List virtual machines with vCPU, RAM, and consumed storage",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return withClient(cmd, func(ctx context.Context, c *vim25.Client) error {
			vms, err := inventory.GetVMs(ctx, c)
			if err != nil {
				return err
			}
			return renderVMs(cmd.OutOrStdout(), vms)
		})
	},
}

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List datastores with transport type and used/available capacity",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return withClient(cmd, func(ctx context.Context, c *vim25.Client) error {
			dss, err := inventory.GetDatastores(ctx, c)
			if err != nil {
				return err
			}
			return renderDatastores(cmd.OutOrStdout(), dss)
		})
	},
}

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List virtual switches and port groups, or VMs on a --portgroup",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return withClient(cmd, func(ctx context.Context, c *vim25.Client) error {
			if portgroupFlag != "" {
				vms, err := inventory.GetPortgroupVMs(ctx, c, portgroupFlag)
				if err != nil {
					return err
				}
				return renderVMs(cmd.OutOrStdout(), vms)
			}
			sws, err := inventory.GetSwitches(ctx, c)
			if err != nil {
				return err
			}
			return renderSwitches(cmd.OutOrStdout(), sws)
		})
	},
}

// withClient resolves configuration, connects to vCenter, derives a
// timeout-bounded context, runs fn, and always logs out afterward.
func withClient(cmd *cobra.Command, fn func(ctx context.Context, c *vim25.Client) error) error {
	v := config.New()
	if path, _ := cmd.Flags().GetString(config.KeyConfig); path != "" {
		if err := config.LoadConfigFile(v, path); err != nil {
			return err
		}
	}
	if err := config.BindFlags(v, cmd); err != nil {
		return err
	}
	cfg := config.Resolve(v)
	if err := validate(cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer cancel()

	gc, err := connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		// Log out on a fresh context so cleanup still runs even if the
		// operation context has been cancelled or has timed out.
		logoutCtx, logoutCancel := context.WithTimeout(context.Background(), logoutTimeout)
		defer logoutCancel()
		_ = gc.Logout(logoutCtx)
	}()

	return fn(ctx, gc.Client)
}

func validate(cfg config.Config) error {
	var missing []string
	if cfg.URL == "" {
		missing = append(missing, "url")
	}
	if cfg.Username == "" {
		missing = append(missing, "username")
	}
	if cfg.Password == "" {
		missing = append(missing, "password")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s "+
			"(set via --flag, VSPHERE_<KEY> env var, or --config file)",
			strings.Join(missing, ", "))
	}
	return nil
}

func connect(ctx context.Context, cfg config.Config) (*govmomi.Client, error) {
	u, err := soap.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing vCenter URL %q: %w", cfg.URL, err)
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	gc, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("connecting to vCenter at %s: %w", u.Host, err)
	}
	return gc, nil
}

// --- presentation ---

func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
}

func formatGB(memoryMB int64) string {
	return fmt.Sprintf("%.1f GB", float64(memoryMB)/1024.0)
}

func renderVMs(w io.Writer, vms []inventory.VMInfo) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, v := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			v.Name, v.NumCPU, formatGB(v.MemoryMB), inventory.FormatBytes(v.CommittedBytes))
	}
	return tw.Flush()
}

func renderDatastores(w io.Writer, dss []inventory.DatastoreInfo) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, d := range dss {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			d.Name, d.Type, inventory.FormatBytes(d.UsedBytes()), inventory.FormatBytes(d.FreeBytes))
	}
	return tw.Flush()
}

func renderSwitches(w io.Writer, sws []inventory.SwitchInfo) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, s := range sws {
		uplinks := strings.Join(s.Uplinks, ",")
		if uplinks == "" {
			uplinks = "-"
		}
		if len(s.PortGroups) == 0 {
			fmt.Fprintf(tw, "%s\t%s\t-\t-\t%s\t%s\t-\t-\n", s.Name, s.Type, uplinks, s.LACP)
			continue
		}
		for _, pg := range s.PortGroups {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				s.Name, s.Type, pg.Name, pg.VLAN, uplinks, s.LACP, pg.TotalPorts, pg.UsedPorts)
		}
	}
	return tw.Flush()
}
