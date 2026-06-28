# Build the `vswitches` Listing Subcommand of a vSphere Inventory CLI in Go (govmomi)

> **Scope note.** This is **3 of 4** decomposed specs derived from a single
> larger task. Build **only** the default `vswitches` switch/port-group listing
> described here, as a complete, standalone program. The `--portgroup <name>`
> VM-lookup mode is covered by its **own separate spec (4 of 4)** and is **out of
> scope here** — do not implement it. The `vms` and `datastores` subcommands are
> also out of scope.

## Objective

Build a single command-line application in Go that connects to a VMware vCenter
Server and reports its virtual-switch inventory through one subcommand,
`vswitches`. Deliver complete, compiling, runnable source code.

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
- **Architecture:** one binary, a root command with the `vswitches` subcommand.
- **Quality bar:** `go build ./...` and `go vet ./...` must pass clean,
  `gofmt`-clean, idiomatic Go. No `panic` in normal flow — return wrapped
  errors with context. No goroutine leaks. Respect `context.Context`
  cancellation/timeout throughout.

## Shared connection configuration

The subcommand connects to a vCenter. Wire configuration through **Viper** with
this precedence (highest first):

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

## Subcommand — `vswitches`

Print all virtual switches — both **standard** (host vSwitches) and
**distributed** (vDS) — along with their port groups. For each port group, show:

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

> The `--portgroup <name>` mode (listing VMs connected to a named port group) is
> **out of scope for this spec** — it is spec 4 of 4. Build only the default
> listing above.

## Output & formatting rules

- Use `text/tabwriter` for the table; aligned columns with a clear header row.
- Use consistent units where applicable.
- Output must be plain, greppable text (no color codes, no box-drawing).

## Unit tests (required — must pass)

**Design for testability.** Put the inventory-retrieval logic in a function that
takes a `context.Context` and a vSphere client and returns typed results (e.g.
`[]SwitchInfo`). Keep that logic separate from the Cobra command wiring and from
the `tabwriter` presentation so it can be tested directly.

Write tests with Go's standard `testing` package using govmomi's embedded
**`github.com/vmware/govmomi/simulator`** package (NOT the external `vcsim`
binary), so tests are hermetic and run with a plain `go test ./...`. The
`simulator.Test(...)` / `simulator.VPX().Run(...)` helpers give a connected
client against an in-process vCenter; configure the model so assertions are
deterministic.

Provide at minimum:

1. **vSwitches** — assert at least one switch is returned, VLAN values parse,
   `used ports ≤ total ports`, and `LACP` ∈ `enabled`/`disabled`/`N/A`.

Also unit-test the pure helpers, where the strongest deterministic assertions
live (no simulator needed):

- **Config precedence** — set a config-file value, override with an env var,
  override with a flag, and assert the resolved value follows
  flag > env > file > default.
- **VLAN formatting** — table-driven tests for the helper that renders a single
  VLAN ID, a trunk range, and a private-VLAN type.
- **Port math** — table-driven tests for the `used = total − available` helper.

**Test integrity rules:** every test contains real assertions tied to expected
values. No `t.Skip`, no empty or tautological tests, no rigging a test to pass,
and no loosening an assertion just to get green. `go test ./...` must pass with
zero failures and zero skips.

## Definition of done (acceptance criteria)

1. `go build ./...` produces a working binary with the `vswitches` subcommand.
2. Viper precedence works: a flag overrides an env var, which overrides the
   config file, which overrides the default.
3. `vswitches` covers both **standard and distributed** switches.
4. LACP is reported correctly (distributed-only; standard reports `N/A`).
5. `used ports = total − available`.
6. Errors are wrapped and surfaced; no panics; context timeout is honored.
7. `go test ./...` passes with zero failures and zero skips, with at least the
   meaningful `vswitches` simulator test plus pure-function tests for config
   precedence, VLAN formatting, and the port math.

Criteria 1, 2, 3, 5, 6, and 7 are fully verifiable locally (unit tests + the
`vcsim` loop). The full-fidelity parts of 4 (real LACP/uplink state) are
validated on a **live vCenter** — the simulator does not richly model them.
Against `vcsim` and in unit tests, the `LACP` and `UPLINKS` fields must still
render without error, degrading to `N/A`.

## Self-verification loop (required — use the local `vcsim` simulator)

Do not consider the task complete until the program builds **and runs cleanly**
against govmomi's bundled vCenter simulator, `vcsim`. Work in a
build → run → diagnose → fix loop and repeat until the subcommand executes
without error and produces well-formed output.

**Start the simulator.** It ships inside the govmomi module you already depend
on, so it runs at the same version — no extra dependency:

```
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3
```

> If your chosen govmomi version reports that it does not contain the `vcsim`
> package, either pin a govmomi version that ships the `vcsim` command or stand
> up an in-process server from the embedded `simulator` package
> (`simulator.Model` → `Service.NewServer()`). The hermetic unit test is the
> authoritative gate either way.

Defaults: endpoint `https://127.0.0.1:8989/sdk`, username `user`, password
`pass`, self-signed TLS (connect with `insecure=true`). The default VPX model
exposes both standard host vSwitches and a distributed switch with port groups;
scale `-pg` for more rows.

**Drive the code path** against the running simulator:

```
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

./<binary> vswitches
```

**Exit condition — what "achieves its goal" means against `vcsim`:**

- `go build ./...` and `go vet ./...` pass clean.
- `vswitches` runs against the simulator with a zero exit code, no panics, and
  no unhandled errors.
- The listing includes **both** standard and distributed switches with their
  port groups, with aligned columns and a header.
- Fields the simulator does not populate degrade gracefully to `N/A` — the
  program must never crash or drop a row because a value is missing.

**Simulator fidelity — read this.** `vcsim` is an integration/smoke harness, not
a full correctness oracle. It does not richly model LACP/uplink detail, so the
`LACP`/`UPLINKS` fields may legitimately return `N/A` against the simulator. That
is expected and acceptable. Real LACP/uplink state is the spec for a live
vCenter.

**Do not fabricate data.** Never hardcode, fake, stub, or special-case values to
make the simulator output look complete, and never loosen the spec to force a
pass. Report only what you can truthfully derive from the API; use `N/A`
otherwise. In particular, `USED` must be derived (`total − available`), not a
constant, and standard vSwitches must actually appear in the listing — do not
silently drop a switch category.

## Deliverables

- Complete source for every file.
- `go.mod` (and a note on running `go mod tidy`).
- Project layout (directory tree).
- Build and run instructions, including an example `config.yaml` and an example
  using environment variables.
- A script or `Makefile` target (e.g. `make verify`) that automates the full
  check: runs `go vet ./...` and `go test ./...`, then starts `vcsim` in the
  background, waits for it to be ready, runs the `vswitches` subcommand against
  it, and exits non-zero on any failure. Tear the simulator down when finished.
- A short note confirming the code was actually run: paste a sample
  `go test ./...` result and a sample `vcsim` run.
- The same code must also be deployable against a live vCenter, where the
  full-fidelity LACP/uplink behavior applies.
