# Build a vSphere Inventory CLI in Go (govmomi)

## Objective

Build a single command-line application in Go that connects to a VMware vCenter
Server and reports virtualization inventory. The application exposes three
subcommands (`vms`, `datastores`, `vswitches`). Deliver complete, compiling,
runnable source code.

You are not finished when the code compiles — you are finished when it has been
**run and verified against the local `vcsim` simulator** in an iterative
build → run → diagnose → fix loop. See "Self-verification loop" below; treat it
as part of the task, not an afterthought.

## Hard constraints

- **Language:** Go 1.22+ using Go modules.
- **Direct dependencies — use ONLY these:**
  - `github.com/vmware/govmomi` — vSphere API client
  - `github.com/spf13/cobra` — command structure
  - `github.com/spf13/viper` — configuration
  - The Go standard library. Use `text/tabwriter` for all table output.
  - Do not import any other third-party table, CLI, or VMware libraries.
- **Architecture:** one binary, a root command with three subcommands.
- **Quality bar:** `go build ./...` and `go vet ./...` must pass clean,
  `gofmt`-clean, idiomatic Go. No `panic` in normal flow — return wrapped
  errors with context. No goroutine leaks. Respect `context.Context`
  cancellation/timeout throughout.

## Shared connection configuration

All three subcommands connect to the same vCenter. Wire configuration through
**Viper** with this precedence (highest first):

1. command-line flag
2. environment variable
3. config file
4. built-in default

Configuration fields:

| Key        | Flag           | Env var              | Notes                                            |
|------------|----------------|----------------------|--------------------------------------------------|
| url        | `--url`        | `VSPHERE_URL`        | vCenter URL or host, e.g. `https://vc.lab/sdk`   |
| username   | `--username`   | `VSPHERE_USERNAME`   |                                                  |
| password   | `--password`   | `VSPHERE_PASSWORD`   |                                                  |
| insecure   | `--insecure`   | `VSPHERE_INSECURE`   | bool, skip TLS verification, default `false`     |
| timeout    | `--timeout`    | `VSPHERE_TIMEOUT`    | overall operation timeout, default `60s`         |
| config     | `--config`     | —                    | optional path to a YAML config file              |

- Environment variable prefix is `VSPHERE_`.
- The YAML config file holds the same keys (`url`, `username`, `password`,
  `insecure`, `timeout`).
- Establish one authenticated client (`govmomi.Client` / `vim25.Client`) using a
  `context.Context` derived from the configured timeout. `defer` a clean logout.
- Surface authentication and connection failures with clear, actionable error
  messages.

## Subcommand 1 — `vms`

Print a table of all virtual machines in the inventory. Columns:

- **NAME**
- **VCPU** — configured vCPU count
- **RAM** — configured memory, shown in GB
- **STORAGE** — *actual storage consumed* (committed), human-readable
  (GiB/TiB). This is consumed storage, **not** provisioned/allocated capacity.

Sort rows by VM name.

## Subcommand 2 — `datastores`

Print a table of all datastores. Columns:

- **NAME**
- **TYPE** — the underlying transport/protocol: one of `FC`, `iSCSI`, `NVMe`,
  or `NFS`.
  - **Important:** the datastore's filesystem type (`VMFS`, `NFS`, etc.) is
    **not** the answer. A single VMFS datastore may be backed by FC, iSCSI, or
    NVMe. You must derive the actual transport from the datastore's backing
    storage device(s) / host bus adapter(s). NFS datastores report as `NFS`.
- **USED** — used capacity, human-readable (GiB/TiB)
- **AVAILABLE** — free/available capacity, human-readable (GiB/TiB)

Sort rows by datastore name.

## Subcommand 3 — `vswitches`

**Default behavior:** print all virtual switches — both **standard** (host
vSwitches) and **distributed** (vDS) — along with their port groups. For each
port group, show:

- **SWITCH** — switch name
- **SWITCH TYPE** — `standard` or `distributed`
- **PORTGROUP** — port group name
- **VLAN** — VLAN ID. For trunk or private-VLAN port groups, show the range or
  type rather than a single ID.
- **UPLINKS** — physical NIC(s) / uplink port name(s) backing the switch
- **LACP** — whether LACP is enabled.
  - Note: LACP applies to distributed switches only. For standard vSwitches,
    report `N/A` (or `disabled`).
- **PORTS** — total ports
- **USED** — ports currently in use

**With `--portgroup <name>`:** do not print the switch listing. Instead, print
the list of virtual machines connected to the named port group. This must work
for both standard and distributed port groups.

## Output & formatting rules

- Use `text/tabwriter` for all tables; aligned columns with a clear header row.
- Use consistent units (GiB/TiB) with one decimal place.
- Output must be plain, greppable text (no color codes, no box-drawing).

## Unit tests (required — must pass)

**Design for testability.** Put the inventory-retrieval logic for each feature in
functions that take a `context.Context` and a vSphere client and return typed
results (e.g. `[]VMInfo`, `[]DatastoreInfo`, `[]SwitchInfo`, and a `[]VMInfo` for
the port-group lookup). Keep that logic separate from the Cobra command wiring
and from the `tabwriter` presentation so each feature can be tested directly.

Write tests with Go's standard `testing` package using govmomi's embedded
**`github.com/vmware/govmomi/simulator`** package (NOT the external `vcsim`
binary), so tests are hermetic and run with a plain `go test ./...`. The
`simulator.Test(...)` / `simulator.VPX().Run(...)` helpers give a connected
client against an in-process vCenter; configure the model
(machine/datastore/port-group counts) so assertions are deterministic.

Provide at minimum one meaningful test per feature:

1. **VMs** — configure the model to a known VM count, assert the function returns
   that count, and that each result has a non-empty name, vCPU > 0, RAM > 0, and
   storage ≥ 0.
2. **Datastores** — assert each datastore has a non-empty name, that
   `used + available` is consistent with capacity (within rounding) and
   `available ≤ capacity`, and that `TYPE` ∈ `FC`/`iSCSI`/`NVMe`/`NFS`/`unknown`.
3. **vSwitches** — assert at least one switch is returned, VLAN values parse,
   `used ports ≤ total ports`, and `LACP` ∈ `enabled`/`disabled`/`N/A`.
4. **Port group → VMs** — configure the model so known VMs are attached to a
   known port group, then assert the lookup returns exactly that set.

Also unit-test the pure helpers, where the strongest deterministic assertions
live (no simulator needed):

- **Config precedence** — set a config-file value, override with an env var,
  override with a flag, and assert the resolved value follows
  flag > env > file > default.
- **Byte formatting** — table-driven tests for the human-readable GiB/TiB
  formatter and the `used = total − available` math.
- **Transport classifier** — because `vcsim` cannot model storage transport,
  factor the FC/iSCSI/NVMe decision into a pure function mapping a device/HBA
  descriptor to a protocol, and table-test it with representative FC, iSCSI, and
  NVMe inputs. This is how you actually prove criterion 4's logic.

**Test integrity rules:** every test contains real assertions tied to expected
values. No `t.Skip`, no empty or tautological tests, no rigging a test to pass,
and no loosening an assertion just to get green. `go test ./...` must pass with
zero failures and zero skips.

## Definition of done (acceptance criteria)

1. `go build ./...` produces a working binary with three subcommands.
2. Viper precedence works: a flag overrides an env var, which overrides the
   config file, which overrides the default.
3. `vms` reports consumed (committed) storage, not provisioned.
4. `datastores` reports real transport (FC/iSCSI/NVMe/NFS), not filesystem type.
5. `vswitches` covers both standard and distributed switches; LACP is reported
   correctly (distributed-only); used ports = total − available.
6. `vswitches --portgroup <name>` lists connected VMs for standard *and*
   distributed port groups.
7. Errors are wrapped and surfaced; no panics; context timeout is honored.
8. `go test ./...` passes with zero failures and zero skips, with at least one
   meaningful test per feature (VMs, datastores, vSwitches, port group → VMs)
   plus pure-function tests for config precedence, byte formatting, and the
   transport classifier.

Criteria 1, 2, 3, 6, 7, and 8 are fully verifiable locally (unit tests + the
`vcsim` loop). The full-fidelity parts of 4 (real FC/iSCSI/NVMe transport) and 5
(real LACP/uplink state) are validated on a **live vCenter** — the simulator does
not model them (see "Self-verification loop"). Against `vcsim` and in unit tests,
those fields must still render without error, degrading to `unknown`/`N/A`; the
transport classifier's own logic is proven by its dedicated pure-function test.

## Self-verification loop (required — use the local `vcsim` simulator)

Do not consider the task complete until the program builds **and runs cleanly**
against govmomi's bundled vCenter simulator, `vcsim`. Work in a
build → run → diagnose → fix loop and repeat until every subcommand executes
without error and produces well-formed output.

**Start the simulator.** It ships inside the govmomi module you already depend
on, so it runs at the same version — no extra dependency:

```
go run github.com/vmware/govmomi/vcsim
```

Defaults: endpoint `https://127.0.0.1:8989/sdk`, username `user`, password
`pass`, self-signed TLS (connect with `insecure=true`). To get more rows for
exercising sorting/formatting, scale the inventory, e.g.:

```
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3
```

**Drive every code path** against the running simulator:

```
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

./<binary> vms
./<binary> datastores
./<binary> vswitches
# discover a real portgroup name from the vswitches output above, then:
./<binary> vswitches --portgroup "<name from output>"
```

Discover the `--portgroup` value from your own `vswitches` output rather than
hardcoding a name — inventory names vary by simulator version.

**Exit condition — what "achieves its goal" means against `vcsim`:**

- `go build ./...` and `go vet ./...` pass clean.
- All three subcommands run against the simulator with a zero exit code, no
  panics, and no unhandled errors.
- Output is correctly shaped: aligned columns, headers, consistent units, and
  the specified sort order.
- `vswitches --portgroup <name>` returns the VMs attached to a real simulator
  port group.
- Fields the simulator does not populate degrade gracefully to `unknown`/`N/A` —
  the program must never crash or drop a row because a value is missing.

**Simulator fidelity — read this.** `vcsim` is an integration/smoke harness, not
a full correctness oracle. It does not richly model storage transport topology
(HBA → LUN → extent) or LACP/uplink detail, so the datastore `TYPE` and the
`vswitches` `LACP`/`UPLINKS` fields may legitimately return `unknown`/`N/A`/`NFS`
against the simulator. That is expected and acceptable. Real FC/iSCSI/NVMe and
real LACP state are the spec for a live vCenter.

**Do not fabricate data.** Never hardcode, fake, stub, or special-case values to
make the simulator output look complete, and never loosen the spec to force a
pass. Report only what you can truthfully derive from the API; use
`unknown`/`N/A` otherwise. Passing the loop means the code is correct and robust
against the data the API actually returns.

## Deliverables

- Complete source for every file.
- `go.mod` (and a note on running `go mod tidy`).
- Project layout (directory tree).
- Build and run instructions, including an example `config.yaml` and an example
  using environment variables.
- A script or `Makefile` target (e.g. `make verify`) that automates the full
  check: runs `go vet ./...` and `go test ./...`, then starts `vcsim` in the
  background, waits for it to be ready, runs all three subcommands plus a
  `--portgroup` invocation against it, and exits non-zero on any failure. Tear
  the simulator down when finished.
- A short note confirming the code was actually run: paste a sample `go test ./...`
  result and a sample `vcsim` run.
- The same code must also be deployable against a live vCenter, where the
  full-fidelity transport/LACP behavior applies.
