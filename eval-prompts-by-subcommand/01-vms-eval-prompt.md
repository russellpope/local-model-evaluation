# Build the `vms` Subcommand of a vSphere Inventory CLI in Go (govmomi)

> **Scope note.** This is **1 of 4** decomposed specs derived from a single
> larger task. Build **only** the `vms` subcommand described here, as a complete,
> standalone program. The `datastores` and `vswitches` subcommands are covered by
> their own separate specs and are **out of scope** — do not implement them.

## Objective

Build a single command-line application in Go that connects to a VMware vCenter
Server and reports its virtual-machine inventory through one subcommand, `vms`.
Deliver complete, compiling, runnable source code.

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
- **Architecture:** one binary, a root command with the `vms` subcommand.
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

## Subcommand — `vms`

Print a table of all virtual machines in the inventory. Columns:

- **NAME**
- **VCPU** — configured vCPU count
- **RAM** — configured memory, shown in GB
- **STORAGE** — *actual storage consumed* (committed), human-readable
  (GiB/TiB). This is consumed storage, **not** provisioned/allocated capacity.

Sort rows by VM name.

## Output & formatting rules

- Use `text/tabwriter` for the table; aligned columns with a clear header row.
- Use consistent units (GiB/TiB) with one decimal place.
- Output must be plain, greppable text (no color codes, no box-drawing).

## Unit tests (required — must pass)

**Design for testability.** Put the inventory-retrieval logic in a function that
takes a `context.Context` and a vSphere client and returns typed results (e.g.
`[]VMInfo`). Keep that logic separate from the Cobra command wiring and from the
`tabwriter` presentation so it can be tested directly.

Write tests with Go's standard `testing` package using govmomi's embedded
**`github.com/vmware/govmomi/simulator`** package (NOT the external `vcsim`
binary), so tests are hermetic and run with a plain `go test ./...`. The
`simulator.Test(...)` / `simulator.VPX().Run(...)` helpers give a connected
client against an in-process vCenter; configure the model (machine count) so
assertions are deterministic.

Provide at minimum:

1. **VMs** — configure the model to a known VM count, assert the function returns
   that count, and that each result has a non-empty name, vCPU > 0, RAM > 0, and
   storage ≥ 0.

Also unit-test the pure helpers, where the strongest deterministic assertions
live (no simulator needed):

- **Config precedence** — set a config-file value, override with an env var,
  override with a flag, and assert the resolved value follows
  flag > env > file > default.
- **Byte formatting** — table-driven tests for the human-readable GiB/TiB
  formatter.

**Test integrity rules:** every test contains real assertions tied to expected
values. No `t.Skip`, no empty or tautological tests, no rigging a test to pass,
and no loosening an assertion just to get green. `go test ./...` must pass with
zero failures and zero skips.

## Definition of done (acceptance criteria)

1. `go build ./...` produces a working binary with the `vms` subcommand.
2. Viper precedence works: a flag overrides an env var, which overrides the
   config file, which overrides the default.
3. `vms` reports consumed (committed) storage, not provisioned.
4. Errors are wrapped and surfaced; no panics; context timeout is honored.
5. `go test ./...` passes with zero failures and zero skips, with at least the
   meaningful `vms` simulator test plus pure-function tests for config
   precedence and byte formatting.

All of these are fully verifiable locally (unit tests + the `vcsim` loop).

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
`pass`, self-signed TLS (connect with `insecure=true`).

**Drive the code path** against the running simulator:

```
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

./<binary> vms
```

**Exit condition — what "achieves its goal" means against `vcsim`:**

- `go build ./...` and `go vet ./...` pass clean.
- `vms` runs against the simulator with a zero exit code, no panics, and no
  unhandled errors.
- Output is correctly shaped: aligned columns, a header, consistent units, and
  the specified sort order.
- Fields the simulator does not populate degrade gracefully — the program must
  never crash or drop a row because a value is missing.

**Do not fabricate data.** Never hardcode, fake, stub, or special-case values to
make the simulator output look complete, and never loosen the spec to force a
pass. Report only what you can truthfully derive from the API.

## Deliverables

- Complete source for every file.
- `go.mod` (and a note on running `go mod tidy`).
- Project layout (directory tree).
- Build and run instructions, including an example `config.yaml` and an example
  using environment variables.
- A script or `Makefile` target (e.g. `make verify`) that automates the full
  check: runs `go vet ./...` and `go test ./...`, then starts `vcsim` in the
  background, waits for it to be ready, runs the `vms` subcommand against it, and
  exits non-zero on any failure. Tear the simulator down when finished.
- A short note confirming the code was actually run: paste a sample
  `go test ./...` result and a sample `vcsim` run.
- The same code must also be deployable against a live vCenter.
