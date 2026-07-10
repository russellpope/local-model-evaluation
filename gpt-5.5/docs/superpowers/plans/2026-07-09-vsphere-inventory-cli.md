# vSphere Inventory CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI with `vms`, `datastores`, and `vswitches` subcommands for VMware vCenter inventory.

**Architecture:** A Cobra root command owns flags and Viper configuration, while inventory retrieval lives in testable methods on `Inventory`. Presentation uses `text/tabwriter`, and simulator tests exercise the retrieval paths through govmomi's embedded simulator.

**Tech Stack:** Go 1.22+, `github.com/vmware/govmomi`, `github.com/spf13/cobra`, `github.com/spf13/viper`, Go standard library.

## Global Constraints

- Direct dependencies are only `github.com/vmware/govmomi`, `github.com/spf13/cobra`, `github.com/spf13/viper`, and the Go standard library.
- Tables use `text/tabwriter`.
- `go build ./...`, `go vet ./...`, and `go test ./...` must pass.
- All command operations respect `context.Context` timeout and return wrapped errors.
- Verification must run the binary against local `vcsim`.

---

### Task 1: Tests and Public Interfaces

**Files:**
- Create: `config_test.go`
- Create: `format_test.go`
- Create: `transport_test.go`
- Create: `inventory_simulator_test.go`

**Interfaces:**
- Produces: `NewRootCommand() *cobra.Command`, `LoadConfigForCommand(*cobra.Command) (Config, *viper.Viper, error)`, `FormatBytes(int64) string`, `UsedBytes(int64, int64) int64`, `ClassifyTransportDescriptor(string) StorageTransport`, `NewClient(context.Context, Config) (*govmomi.Client, error)`, `NewInventory(*vim25.Client) *Inventory`.

- [x] **Step 1: Write failing tests**
- [ ] **Step 2: Run tests to verify failure**
- [ ] **Step 3: Implement minimal code**
- [ ] **Step 4: Run tests to verify pass**

### Task 2: CLI, Config, and Table Rendering

**Files:**
- Create: `main.go`
- Create: `config.go`
- Create: `output.go`

**Interfaces:**
- Consumes: `Inventory` methods from Task 3.
- Produces: runnable Cobra commands and tabwriter output.

- [ ] **Step 1: Add command wiring**
- [ ] **Step 2: Add Viper loading with flag > env > file > default precedence**
- [ ] **Step 3: Add table renderers**
- [ ] **Step 4: Run config and formatter tests**

### Task 3: govmomi Inventory Retrieval

**Files:**
- Create: `client.go`
- Create: `inventory.go`
- Create: `types.go`

**Interfaces:**
- Produces: `ListVMs`, `ListDatastores`, `ListSwitches`, `ListVMsByPortGroup`.

- [ ] **Step 1: Implement VM retrieval with committed storage**
- [ ] **Step 2: Implement datastore retrieval and graceful transport classification**
- [ ] **Step 3: Implement standard and distributed switch rows**
- [ ] **Step 4: Implement portgroup-to-VM lookup**
- [ ] **Step 5: Run simulator tests**

### Task 4: Verification Automation and Docs

**Files:**
- Create: `Makefile`
- Create: `README.md`

**Interfaces:**
- Produces: `make verify` running vet, tests, `vcsim`, and all CLI paths.

- [ ] **Step 1: Add verification target**
- [ ] **Step 2: Add build/run docs and sample config**
- [ ] **Step 3: Run `make verify`**
