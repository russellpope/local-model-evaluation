package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newVMsCommand(state *appConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithInventory(cmd, state, func(ctx context.Context, inv *Inventory) error {
				rows, err := inv.ListVMs(ctx)
				if err != nil {
					return err
				}
				if err := WriteVMs(cmd.OutOrStdout(), rows); err != nil {
					return fmt.Errorf("write VM table: %w", err)
				}
				return nil
			})
		},
	}
}

func newDatastoresCommand(state *appConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithInventory(cmd, state, func(ctx context.Context, inv *Inventory) error {
				rows, err := inv.ListDatastores(ctx)
				if err != nil {
					return err
				}
				if err := WriteDatastores(cmd.OutOrStdout(), rows); err != nil {
					return fmt.Errorf("write datastore table: %w", err)
				}
				return nil
			})
		},
	}
}

func newSwitchesCommand(state *appConfig) *cobra.Command {
	var portgroup string
	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List virtual switches and port groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithInventory(cmd, state, func(ctx context.Context, inv *Inventory) error {
				if portgroup != "" {
					rows, err := inv.ListVMsByPortGroup(ctx, portgroup)
					if err != nil {
						return err
					}
					if err := WriteVMs(cmd.OutOrStdout(), rows); err != nil {
						return fmt.Errorf("write portgroup VM table: %w", err)
					}
					return nil
				}

				rows, err := inv.ListSwitches(ctx)
				if err != nil {
					return err
				}
				if err := WriteSwitches(cmd.OutOrStdout(), rows); err != nil {
					return fmt.Errorf("write switch table: %w", err)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&portgroup, "portgroup", "", "list VMs connected to the named port group")
	return cmd
}

func runWithInventory(cmd *cobra.Command, state *appConfig, run func(context.Context, *Inventory) error) error {
	if err := state.load(cmd); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), state.cfg.Timeout)
	defer cancel()

	client, err := NewClient(ctx, state.cfg)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout(context.Background()) }()

	return run(ctx, NewInventory(client.Client))
}
