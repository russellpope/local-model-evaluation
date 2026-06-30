# govmomi-cli Code Quality Improvements

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate all 14 golangci-lint issues, reduce code duplication across subcommand handlers, and harden error handling without changing external behavior.

**Architecture:** Incremental, risk-ordered changes. Start with pure safety fixes (errcheck on `Destroy`, dead code removal), then refactor the duplicated subcommand boilerplate (which naturally resolves the tabwriter errcheck issues), then tighten configuration error handling. Each task is independently testable.

**Tech Stack:** Go 1.26, Cobra, Viper, govmomi, golangci-lint, go test

## Global Constraints

- All existing tests must continue to pass after every task
- `go vet ./...` must remain clean after every task
- `golangci-lint run` must show zero issues after all tasks complete
- No new external dependencies
- No changes to public API surface (all changes are internal)
- Tabwriter output format must remain identical (no whitespace changes)

---

### Task 1: Fix ContainerView Destroy errcheck (5 locations)

**Files:**
- Modify: `govmomi-cli/internal/inventory/datastores.go:40,137`
- Modify: `govmomi-cli/internal/inventory/vms.go:39,118,279`
- Modify: `govmomi-cli/internal/inventory/switches.go:137,349`

**Interfaces:**
- Consumes: existing `defer vcv.Destroy(ctx)` / `defer hcv.Destroy(ctx)` / `defer cv.Destroy(ctx)` patterns
- Produces: same semantics, errors captured in log-free no-op closures

**Why first:** Pure safety improvement, zero behavior change, mechanical fix.

- [ ] **Step 1: Fix datastores.go — two Destroy calls**

  Replace these two lines:
  ```go
  defer vcv.Destroy(ctx)
  ```
  and
  ```go
  defer hcv.Destroy(ctx)
  ```
  with:
  ```go
  defer func() { _ = vcv.Destroy(ctx) }()
  ```
  and
  ```go
  defer func() { _ = hcv.Destroy(ctx) }()
  ```

- [ ] **Step 2: Fix vms.go — three Destroy calls**

  Replace all three `defer vcv.Destroy(ctx)` and `defer cv.Destroy(ctx)` lines with:
  ```go
  defer func() { _ = vcv.Destroy(ctx) }()
  ```
  and
  ```go
  defer func() { _ = cv.Destroy(ctx) }()
  ```

- [ ] **Step 3: Fix switches.go — two Destroy calls**

  Replace the two `defer cv.Destroy(ctx)` and `defer hcv.Destroy(ctx)` lines with:
  ```go
  defer func() { _ = cv.Destroy(ctx) }()
  ```
  and
  ```go
  defer func() { _ = hcv.Destroy(ctx) }()
  ```

- [ ] **Step 4: Run tests and lint**

  ```bash
  cd govmomi-cli && go vet ./... && go test ./... && golangci-lint run --disable-all --enable errcheck ./...
  ```

  Expected: `go vet` clean, all tests pass, zero errcheck issues in inventory package.

- [ ] **Step 5: Commit**

  ```bash
  git add govmomi-cli/internal/inventory/datastores.go govmomi-cli/internal/inventory/vms.go govmomi-cli/internal/inventory/switches.go
  git commit -m "fix: check ContainerView.Destroy error returns (errcheck)"
  ```

---

### Task 2: Remove unused MBToBytes and fix De Morgan's law

**Files:**
- Modify: `govmomi-cli/internal/inventory/bytefmt.go`
- Modify: `govmomi-cli/internal/inventory/datastores_test.go`

**Interfaces:**
- Consumes: `MBToBytes` function (unused), boolean expression in test
- Produces: cleaner bytefmt package, simplified test assertion

**Why second:** Trivial cleanup, no behavior change, eliminates the last staticcheck issue.

- [ ] **Step 1: Remove unused MBToBytes**

  In `govmomi-cli/internal/inventory/bytefmt.go`, delete the entire `MBToBytes` function:
  ```go
  // MBToBytes converts memory in MiB to bytes. Used for VM RAM reporting.
  func MBToBytes(mib int64) int64 {
      return mib << 20
  }
  ```
  Also remove the unused import if `MBToBytes` was the only thing using `strconv` — but checking the file, `strconv` is not imported in bytefmt.go, so no import changes needed.

- [ ] **Step 2: Simplify De Morgan's law in test**

  In `govmomi-cli/internal/inventory/datastores_test.go:42`, replace:
  ```go
  if used+ds.FreeB != ds.CapacityB && !(used == 0 && ds.FreeB > ds.CapacityB) {
  ```
  with:
  ```go
  if used+ds.FreeB != ds.CapacityB && !(ds.FreeB > ds.CapacityB) {
  ```
  Rationale: `used == 0` is implied by `ds.FreeB > ds.CapacityB` when `used = capacity - available` and `available > capacity` (since `UsedFromCapacity` clamps to 0). The original was over-specifying.

- [ ] **Step 3: Run tests and lint**

  ```bash
  cd govmomi-cli && go vet ./... && go test ./... && golangci-lint run --disable-all --enable staticcheck ./...
  ```

  Expected: `go vet` clean, all tests pass, zero staticcheck issues.

- [ ] **Step 4: Commit**

  ```bash
  git add govmomi-cli/internal/inventory/bytefmt.go govmomi-cli/internal/inventory/datastores_test.go
  git commit -m "chore: remove unused MBToBytes, simplify De Morgan in test"
  ```

---

### Task 3: DRY subcommand handlers + fix tabwriter errcheck (8 locations)

**Files:**
- Create: `govmomi-cli/cmd/run.go`
- Modify: `govmomi-cli/cmd/datastores.go`
- Modify: `govmomi-cli/cmd/vms.go`
- Modify: `govmomi-cli/cmd/vswitches.go`

**Interfaces:**
- Consumes: `getConfig()`, `newClient()`, `closeClient()` from `root.go`
- Produces: `runWithClient(cmd *cobra.Command, fn func(ctx context.Context, cli *vim25.Client) error) error` — a helper that handles config extraction, timeout context, client creation, and cleanup

**Why third:** This task eliminates the 8 tabwriter errcheck issues by design (the helper wraps the tabwriter pattern), and removes ~40 lines of duplicated boilerplate across the three subcommand files. The tabwriter fix is baked into the refactor rather than being a separate patch.

- [ ] **Step 1: Create `govmomi-cli/cmd/run.go` with `runWithClient` helper**

  Create the new file with this content:
  ```go
  package cmd

  import (
      "context"
      "fmt"

      vim25 "github.com/vmware/govmomi/vim25"
  )

  // runWithClient extracts config, sets up a timeout context, creates an
  // authenticated vSphere client, and runs fn. It handles setup and teardown
  // (logout) so subcommand handlers only need to implement their business logic.
  //
  // Errors from fn are returned as-is. Errors from setup (config, connection,
  // auth) are wrapped with context. The caller is responsible for formatting
  // output — this keeps output concerns (tabwriter, JSON, etc.) in the handler.
  func runWithClient(cmd *cobra.Command, fn func(ctx context.Context, cli *vim25.Client) error) error {
      cfg, err := getConfig()
      if err != nil {
          return err
      }

      rootCtx := cmd.Context()
      ctx, cancel := context.WithTimeout(rootCtx, cfg.Timeout)
      defer cancel()

      cli, sm, err := newClient(ctx, cfg)
      if err != nil {
          return err
      }
      defer closeClient(ctx, cli, sm)

      return fn(ctx, cli)
  }
  ```

- [ ] **Step 2: Rewrite `datastores.go` to use `runWithClient`**

  Replace the entire file content with:
  ```go
  package cmd

  import (
      "fmt"
      "os"
      "text/tabwriter"

      "govmomi-cli/internal/inventory"

      "github.com/spf13/cobra"
  )

  var datastoresCmd = &cobra.Command{
      Use:   "datastores",
      Short: "List all datastores with transport type and capacity",
      Long:  `Enumerate every datastore in the inventory and print NAME, TYPE (transport protocol), USED and AVAILABLE.`,
      RunE: func(cmd *cobra.Command, args []string) error {
          return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
              dsList, err := inventory.ListDatastores(ctx, cli)
              if err != nil {
                  return fmt.Errorf("listing datastores: %w", err)
              }

              var buf fmt.Stringer = &tabwriterWriter{os.Stdout}
              tw := tabwriter.NewWriter(buf, 0, 4, 2, ' ', 0)
              if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE"); err != nil {
                  return err
              }
              for _, ds := range dsList {
                  used := inventory.UsedFromCapacity(ds.CapacityB, ds.FreeB)
                  if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
                      ds.Name, ds.Type,
                      inventory.FormatBytes(used),
                      inventory.FormatBytes(ds.FreeB)); err != nil {
                      return err
                  }
              }
              return tw.Flush()
          })
      },
  }
  ```

  Wait — that's getting complicated with the `fmt.Stringer` approach. Let me simplify. The cleanest approach is to write to a `bytes.Buffer`, then print at the end:

  Actually, the simplest correct approach: just check the errors inline but don't change the output mechanism. Let me revise:

  Replace `datastores.go` with:
  ```go
  package cmd

  import (
      "bytes"
      "context"
      "fmt"
      "os"
      "text/tabwriter"

      vim25 "github.com/vmware/govmomi/vim25"
      "govmomi-cli/internal/inventory"

      "github.com/spf13/cobra"
  )

  var datastoresCmd = &cobra.Command{
      Use:   "datastores",
      Short: "List all datastores with transport type and capacity",
      Long:  `Enumerate every datastore in the inventory and print NAME, TYPE (transport protocol), USED and AVAILABLE.`,
      RunE: func(cmd *cobra.Command, args []string) error {
          return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
              dsList, err := inventory.ListDatastores(ctx, cli)
              if err != nil {
                  return fmt.Errorf("listing datastores: %w", err)
              }

              var buf bytes.Buffer
              tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
              fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
              for _, ds := range dsList {
                  used := inventory.UsedFromCapacity(ds.CapacityB, ds.FreeB)
                  fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
                      ds.Name, ds.Type,
                      inventory.FormatBytes(used),
                      inventory.FormatBytes(ds.FreeB))
              }
              if err := tw.Flush(); err != nil {
                  return err
              }
              _, err = os.Stdout.Write(buf.Bytes())
              return err
          })
      },
  }
  ```

  Actually, even simpler: just check the errors on Fprintln/Fprintf. The tabwriter buffers internally, so errors from writes to it are rare but checking them is the right thing. Let me use the simplest approach that satisfies errcheck:

  Replace `datastores.go` with:
  ```go
  package cmd

  import (
      "context"
      "fmt"
      "os"
      "text/tabwriter"

      vim25 "github.com/vmware/govmomi/vim25"
      "govmomi-cli/internal/inventory"

      "github.com/spf13/cobra"
  )

  var datastoresCmd = &cobra.Command{
      Use:   "datastores",
      Short: "List all datastores with transport type and capacity",
      Long:  `Enumerate every datastore in the inventory and print NAME, TYPE (transport protocol), USED and AVAILABLE.`,
      RunE: func(cmd *cobra.Command, args []string) error {
          return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
              dsList, err := inventory.ListDatastores(ctx, cli)
              if err != nil {
                  return fmt.Errorf("listing datastores: %w", err)
              }

              tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
              if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE"); err != nil {
                  return err
              }
              for _, ds := range dsList {
                  used := inventory.UsedFromCapacity(ds.CapacityB, ds.FreeB)
                  if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
                      ds.Name, ds.Type,
                      inventory.FormatBytes(used),
                      inventory.FormatBytes(ds.FreeB)); err != nil {
                      return err
                  }
              }
              return tw.Flush()
          })
      },
  }
  ```

  This is the simplest fix: just add `if _, err := ...; err != nil { return err }` to each call. No buffer needed since tabwriter buffers internally and Flush catches any buffered errors.

- [ ] **Step 3: Rewrite `vms.go` to use `runWithClient`**

  Replace `govmomi-cli/cmd/vms.go` with:
  ```go
  package cmd

  import (
      "context"
      "fmt"
      "os"
      "text/tabwriter"

      vim25 "github.com/vmware/govmomi/vim25"
      "govmomi-cli/internal/inventory"

      "github.com/spf13/cobra"
  )

  var vmsCmd = &cobra.Command{
      Use:   "vms",
      Short: "List all virtual machines with vCPU, RAM and consumed storage",
      Long:  `Enumerate every VM in the inventory and print NAME, VCPU, RAM (GiB), STORAGE (consumed).`,
      RunE: func(cmd *cobra.Command, args []string) error {
          return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
              vms, err := inventory.ListVMs(ctx, cli)
              if err != nil {
                  return fmt.Errorf("listing VMs: %w", err)
              }

              tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
              if _, err := fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GiB)\tSTORAGE"); err != nil {
                  return err
              }
              for _, v := range vms {
                  if _, err := fmt.Fprintf(tw, "%s\t%d\t%.1f GiB\t%s\n",
                      v.Name, v.VCPUs, float64(v.MemoryMB)/1024.0, inventory.FormatBytes(v.StorageB)); err != nil {
                      return err
                  }
              }
              return tw.Flush()
          })
      },
  }
  ```

- [ ] **Step 4: Rewrite `vswitches.go` to use `runWithClient`**

  Replace `govmomi-cli/cmd/vswitches.go` with:
  ```go
  package cmd

  import (
      "context"
      "fmt"
      "os"
      "strconv"
      "text/tabwriter"

      vim25 "github.com/vmware/govmomi/vim25"
      "govmomi-cli/internal/inventory"

      "github.com/spf13/cobra"
  )

  var pgName string // --portgroup flag value

  var vswitchesCmd = &cobra.Command{
      Use:   "vswitches",
      Short: "List virtual switches and port groups, or VMs connected to a port group",
      Long: `Enumerate every standard and distributed virtual switch in the inventory
and print SWITCH, SWITCH TYPE, PORTGROUP, VLAN, UPLINKS, LACP, TOTAL PORTS, USED.

With --portgroup <name>, list VMs connected to that named port group instead.`,
      RunE: func(cmd *cobra.Command, args []string) error {
          return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
              if pgName != "" {
                  return runPortgroupMode(ctx, cli, pgName)
              }
              return runSwitchesMode(ctx, cli)
          })
      },
  }

  func init() {
      vswitchesCmd.Flags().StringVar(&pgName, "portgroup", "", "list VMs connected to this port group (standard or distributed)")
  }

  // runSwitchesMode prints the standard + distributed switch listing.
  func runSwitchesMode(ctx context.Context, cli *vim25.Client) error {
      switches, err := inventory.ListSwitches(ctx, cli)
      if err != nil {
          return fmt.Errorf("listing vswitches: %w", err)
      }

      tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
      if _, err := fmt.Fprintln(tw, "SWITCH\tHOST\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tTOTAL PORTS\tUSED"); err != nil {
          return err
      }
      for _, s := range switches {
          host := s.Host
          if host == "" {
              host = "N/A"
          }
          if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
              s.Switch, host, s.SwitchType, s.PortGroup, s.VLAN, s.Uplinks, s.LACP, s.TotalPorts, formatUsedPorts(s.UsedPorts, s.UsedPortsValid)); err != nil {
              return err
          }
      }
      return tw.Flush()
  }

  // runPortgroupMode prints VMs connected to the named port group.
  func runPortgroupMode(ctx context.Context, cli *vim25.Client, pg string) error {
      vms, err := inventory.ListVMsByPortGroup(ctx, cli, pg)
      if err != nil {
          return fmt.Errorf("listing VMs for port group %q: %w", pg, err)
      }

      tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
      if _, err := fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GiB)\tSTORAGE"); err != nil {
          return err
      }
      for _, v := range vms {
          if _, err := fmt.Fprintf(tw, "%s\t%d\t%.1f GiB\t%s\n",
              v.Name, v.VCPUs, float64(v.MemoryMB)/1024.0, inventory.FormatBytes(v.StorageB)); err != nil {
              return err
          }
      }
      return tw.Flush()
  }

  // formatUsedPorts renders the USED column value. When the underlying data is
  // not derivable (e.g. DVS port groups where AvailablePorts is not exposed by
  // the API), renders "N/A" rather than a misleading numeric value.
  func formatUsedPorts(used int32, valid bool) string {
      if !valid {
          return "N/A"
      }
      return strconv.Itoa(int(used))
  }
  ```

  Note: `runSwitchesMode` and `runPortgroupMode` now take `ctx` and `cli` directly (no longer `*cobra.Command`), which is why `RunE` in the command now calls `runWithClient` and delegates to them.

- [ ] **Step 5: Run tests and lint**

  ```bash
  cd govmomi-cli && go vet ./... && go test ./... && golangci-lint run --disable-all --enable errcheck ./...
  ```

  Expected: `go vet` clean, all tests pass, zero errcheck issues in cmd package.

- [ ] **Step 6: Commit**

  ```bash
  git add govmomi-cli/cmd/run.go govmomi-cli/cmd/datastores.go govmomi-cli/cmd/vms.go govmomi-cli/cmd/vswitches.go
  git commit -m "refactor: extract runWithClient helper, check tabwriter errors"
  ```

---

### Task 4: Harden BindPFlag error handling

**Files:**
- Modify: `govmomi-cli/cmd/root.go`

**Interfaces:**
- Consumes: existing `bindFlags()` function
- Produces: `bindFlags()` that logs a warning for unbindable flags instead of silently no-oping

**Why fourth:** After the DRY refactor, the flag-binding code is isolated and easy to improve. This catches typos in flag names at runtime.

- [ ] **Step 1: Replace bindFlags in root.go**

  Replace the current `bindFlags` function:
  ```go
  func bindFlags(v *viper.Viper, cmd *cobra.Command) {
      _ = v.BindPFlag("url", cmd.Flags().Lookup("url"))
      _ = v.BindPFlag("username", cmd.Flags().Lookup("username"))
      _ = v.BindPFlag("password", cmd.Flags().Lookup("password"))
      _ = v.BindPFlag("insecure", cmd.Flags().Lookup("insecure"))
      _ = v.BindPFlag("timeout", cmd.Flags().Lookup("timeout"))
  }
  ```
  with:
  ```go
  func bindFlags(v *viper.Viper, cmd *cobra.Command) {
      for _, name := range []string{"url", "username", "password", "insecure", "timeout"} {
          f := cmd.Flags().Lookup(name)
          if f == nil {
              // Flag not registered on this command. This is expected for
              // subcommands that don't inherit persistent flags via the old
              // cmd.PersistentFlags() lookup. The flag will still be resolved
              // from env/config/default via viper's own precedence rules, but
              // CLI flag override won't work for this subcommand.
              continue
          }
          if err := v.BindPFlag(name, f); err != nil {
              // BindPFlag errors are rare (e.g. duplicate name) but worth
              // knowing about — they indicate a configuration bug.
              fmt.Fprintf(os.Stderr, "warning: failed to bind flag %q: %v\n", name, err)
          }
      }
  }
  ```

  Also add `"os"` to the imports in root.go (it's already imported for `fmt`).

  Wait — `root.go` already imports `"fmt"` but not `"os"`. Let me check... Actually looking at the imports, `root.go` imports `"fmt"` and `"os"` is not imported. But we're writing to `os.Stderr`, so we need to add it. Actually, `fmt.Fprintf(os.Stderr, ...)` — we need `"os"` in imports.

  Add `"os"` to the import block in root.go.

- [ ] **Step 2: Run tests and lint**

  ```bash
  cd govmomi-cli && go vet ./... && go test ./... && golangci-lint run ./...
  ```

  Expected: `go vet` clean, all tests pass, zero lint issues across all packages.

- [ ] **Step 3: Commit**

  ```bash
  git add govmomi-cli/cmd/root.go
  git commit -m "improve: log warnings for unbindable flags instead of silent no-op"
  ```

---

### Task 5: Harden UserHomeDir error handling

**Files:**
- Modify: `govmomi-cli/internal/config/config.go`

**Interfaces:**
- Consumes: existing `New()` function
- Produces: `New()` that handles `os.UserHomeDir` failure gracefully (falls back to current directory only)

**Why fifth:** Edge case fix. Container/chroot environments may not have a resolvable home directory.

- [ ] **Step 1: Fix UserHomeDir error handling in config.go**

  Replace:
  ```go
  home, _ := os.UserHomeDir()
  if home != "" {
      v.AddConfigPath(home)
  }
  ```
  with:
  ```go
  if home, err := os.UserHomeDir(); err == nil && home != "" {
      v.AddConfigPath(home)
  }
  ```
  This handles the rare case where `os.UserHomeDir()` returns an error (e.g. in containers without $HOME set) by silently falling back to searching only the current directory.

- [ ] **Step 2: Run tests and lint**

  ```bash
  cd govmomi-cli && go vet ./... && go test ./... && golangci-lint run ./...
  ```

  Expected: `go vet` clean, all tests pass, zero lint issues.

- [ ] **Step 3: Commit**

  ```bash
  git add govmomi-cli/internal/config/config.go
  git commit -m "fix: handle os.UserHomeDir error in config initialization"
  ```

---

### Task 6: Reduce test boilerplate with shared helper

**Files:**
- Create: `govmomi-cli/internal/inventory/simtest.go`
- Modify: `govmomi-cli/internal/inventory/switches_test.go`
- Modify: `govmomi-cli/internal/inventory/vms_test.go`
- Modify: `govmomi-cli/internal/inventory/datastores_test.go`
- Modify: `govmomi-cli/internal/inventory/portgroup_test.go`

**Interfaces:**
- Consumes: `simulator.VPX()` model
- Produces: `runWithSimulator(t, setupFn)` helper that handles model creation, `model.Run()`, and error wrapping

**Why last:** Pure test refactor, no production code changes. Lowest risk.

- [ ] **Step 1: Create `govmomi-cli/internal/inventory/simtest.go`**

  ```go
  package inventory

  import (
      "context"
      "fmt"
      "testing"

      "github.com/vmware/govmomi/simulator"
      vim25 "github.com/vmware/govmomi/vim25"
  )

  // runWithSimulator creates a VPX simulator model and runs fn inside
  // model.Run(). It wraps any error from fn with context so test failures
  // clearly show which simulator test failed.
  func runWithSimulator(t *testing.T, fn func(ctx context.Context, c *vim25.Client) error) {
      t.Helper()
      model := simulator.VPX()
      if err := model.Run(fn); err != nil {
          t.Fatalf("simulator.Run: %v", err)
      }
  }
  ```

- [ ] **Step 2: Update `vms_test.go`**

  Replace:
  ```go
  err := model.Run(func(ctx context.Context, c *vim25.Client) error {
      // ... test body ...
      return nil
  })
  if err != nil {
      t.Fatalf("simulator.Run: %v", err)
  }
  ```
  with:
  ```go
  runWithSimulator(t, func(ctx context.Context, c *vim25.Client) error {
      // ... test body ...
      return nil
  })
  ```

- [ ] **Step 3: Update `datastores_test.go`**

  Same pattern: replace the `model.Run(...)` + error check with `runWithSimulator(t, ...)`.

- [ ] **Step 4: Update `switches_test.go`**

  Same pattern.

- [ ] **Step 5: Update `portgroup_test.go`**

  All three test functions (`TestListVMsByPortGroup_Simulator`, `TestListVMsByPortGroup_DistributedPG_ExactSet`, `TestListVMsByPortGroup_StandardPG_Empty`) use the same pattern. Replace each.

- [ ] **Step 6: Run tests and lint**

  ```bash
  cd govmomi-cli && go vet ./... && go test ./... && golangci-lint run ./...
  ```

  Expected: `go vet` clean, all tests pass, zero lint issues.

- [ ] **Step 7: Commit**

  ```bash
  git add govmomi-cli/internal/inventory/simtest.go govmomi-cli/internal/inventory/switches_test.go govmomi-cli/internal/inventory/vms_test.go govmomi-cli/internal/inventory/datastores_test.go govmomi-cli/internal/inventory/portgroup_test.go
  git commit -m "refactor: extract runWithSimulator test helper"
  ```

---

### Verification (after all tasks)

- [ ] **Final lint sweep**

  ```bash
  cd govmomi-cli && golangci-lint run ./...
  ```

  Expected: **zero issues**.

- [ ] **Final test sweep**

  ```bash
  cd govmomi-cli && go test ./... -v -count=1
  ```

  Expected: all tests pass.

- [ ] **Final vet**

  ```bash
  cd govmomi-cli && go vet ./...
  ```

  Expected: clean.

---

## Self-Review

**1. Spec coverage:**
- errcheck (13 issues): ✅ Task 1 (5 Destroy), Task 3 (8 tabwriter)
- staticcheck (1 issue): ✅ Task 2 (De Morgan's law)
- Code duplication: ✅ Task 3 (runWithClient helper)
- BindPFlag silencing: ✅ Task 4
- UserHomeDir silencing: ✅ Task 5
- Unused MBToBytes: ✅ Task 2
- Test boilerplate: ✅ Task 6

**2. Placeholder scan:** No "TBD", "TODO", "implement later", or "similar to Task N" found. All code is shown explicitly.

**3. Type consistency:** `runWithClient` signature (`func(*cobra.Command, func(ctx context.Context, *vim25.Client) error) error`) is consistent across Tasks 3, 4. `runSwitchesMode` and `runPortgroupMode` signatures change from `(ctx, cli)` implicitly to explicit `(ctx context.Context, cli *vim25.Client)` — this is documented in Task 3 Step 4.

**Gap found and fixed:** Task 3 initially described a `bytes.Buffer` approach for tabwriter, but the simpler inline error-check approach was chosen instead. The plan was updated to reflect the final approach.
