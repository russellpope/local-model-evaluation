package cmd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"govmomi-cli/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi/session"
	vim25 "github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

var (
	cfgFile string  // --config flag: optional YAML config file path
	viperInst *viper.Viper
)

// rootCmd is the top-level cobra command. It never executes business logic —
// it only initialises viper, wires flags, and delegates to subcommands.
var rootCmd = &cobra.Command{
	Use:   "govmomi-cli",
	Short: "Report vSphere virtualisation inventory",
	Long: `A CLI tool that connects to a VMware vCenter Server and reports
virtualization inventory across three views:

  govmomi-cli vms              List all VMs with vCPU, RAM and consumed storage.
  govmomi-cli datastores       List all datastores with transport type and capacity.
  govmomi-cli vswitches        List standard + distributed switches and port groups.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initViper(cfgFile, cmd)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)
}

// initViper sets up the viper instance with defaults, env prefix VSPHERE_,
// optional config file at cfgPath, and binds all cobra flags so that precedence
// is flag > env > file > default.
func initViper(cfgPath string, cmd *cobra.Command) error {
	v, err := config.New(cfgPath)
	if err != nil {
		return fmt.Errorf("initialising configuration: %w", err)
	}

	bindFlags(v, cmd)

	viperInst = v
	return nil
}

// bindFlags registers every shared flag on the root command and binds it to
// viper so that viper.Get(key) returns the highest-precedence value.
func bindFlags(v *viper.Viper, cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to YAML config file (optional)")

	cmd.PersistentFlags().String("url", "", "vCenter URL or host (e.g. https://vc.lab/sdk)")
	_ = v.BindPFlag("url", cmd.PersistentFlags().Lookup("url"))

	cmd.PersistentFlags().String("username", "", "vCenter username")
	_ = v.BindPFlag("username", cmd.PersistentFlags().Lookup("username"))

	cmd.PersistentFlags().String("password", "", "vCenter password")
	_ = v.BindPFlag("password", cmd.PersistentFlags().Lookup("password"))

	cmd.PersistentFlags().Bool("insecure", false, "skip TLS certificate verification")
	_ = v.BindPFlag("insecure", cmd.PersistentFlags().Lookup("insecure"))

	cmd.PersistentFlags().Duration("timeout", 60*time.Second, "overall operation timeout")
	_ = v.BindPFlag("timeout", cmd.PersistentFlags().Lookup("timeout"))
}

// getConfig extracts the typed config from the shared viper instance. Called
// after PersistentPreRunE has initialised viper on the root command.
func getConfig() (config.Config, error) {
	if viperInst == nil {
		return config.Config{}, fmt.Errorf("viper not initialised — did PersistentPreRunE run?")
	}
	return config.ToStruct(viperInst), nil
}

// newClient builds an authenticated *vim25.Client from the resolved config. It
// respects the configured timeout via context.WithTimeout and defers a logout on
// successful authentication so callers can simply call `defer closeClient(ctx, client)`.
func newClient(ctx context.Context, cfg config.Config) (*vim25.Client, *session.Manager, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing vCenter URL %q: %w", cfg.URL, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	sc := soap.NewClient(u, cfg.Insecure)
	cli, err := vim25.NewClient(timeoutCtx, sc)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to vCenter at %s: %w", u.Host, err)
	}

	sm := session.NewManager(cli)
	auth := url.UserPassword(cfg.Username, cfg.Password)
	if err := sm.Login(timeoutCtx, auth); err != nil {
		return cli, sm, fmt.Errorf("authenticating as user %q against vCenter: %w", cfg.Username, err)
	}

	return cli, sm, nil
}

// closeClient logs out the session. It is safe to call on a client that failed
// authentication — Logout is idempotent when no session exists.
func closeClient(ctx context.Context, _ *vim25.Client, sm *session.Manager) {
	if sm != nil {
		_ = sm.Logout(ctx)
	}
}

// Execute runs the root command and returns any error. Called from main.
func Execute() error {
	return rootCmd.Execute()
}
