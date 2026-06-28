# Build the `vswitches --portgroup` Lookup of a vSphere Inventory CLI in Go (govmomi)

> **Scope note.** This is **4 of 4** decomposed specs derived from a single
> larger task. Build **only** the `vswitches --portgroup <name>` mode described
> here (list the VMs connected to a named port group), as a complete, standalone
> program. The default `vswitches` switch/port-group **listing** is covered by a
> separate spec (3 of 4) and is **out of scope here** — do not implement it. The
> `vms` and `datastores` subcommands are also out of scope.

## Objective

Build a single command-line application in Go that connects to a VMware vCenter
Server and, given a port-group name, lists the virtual machines connected to that
port group via `vswitches --portgroup <name>`. Deliver complete, compiling,
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
- **Architecture:** one binary, a root command with the `vswitches` subcommand
  carrying a `--portgroup <name>` flag.
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

## Subcommand — `vswitches --portgroup <name>`

**With `--portgroup <name>`:** print the list of virtual machines connected to
the named port group. Columns:

- **NAME**
- **VCPU** — configured vCPU count
- **RAM** — configured memory, shown in GB
- **STORAGE** — *actual storage consumed* (committed), human-readable (GiB/TiB)

Sort rows by VM name.

This must work for **both standard and distributed** port groups — the lookup
matches the named port group whether it is backed by a standard port group (the
VM NIC's network backing) or a distributed port group (the VM NIC's DVS port
backing). A VM is "connected" if any of its virtual NICs is attached to that port
group.

> The default `vswitches` switch/port-group **listing** (no flag) is **out of
> scope for this spec** — it is spec 3 of 4. If the binary is invoked without
> `--portgroup`, a brief usage/error message is acceptable; do not implement the
> full listing here.

## Output & formatting rules

- Use `text/tabwriter` for the table; aligned columns with a clear header row.
- Use consistent units (GiB/TiB) with one decimal place.
- Output must be plain, greppable text (no color codes, no box-drawing).

## Unit tests (required — must pass)

**Design for testability.** Put the lookup logic in a function that takes a
`context.Context`, a vSphere client, and a port-group name, and returns typed
results (e.g. `[]VMInfo`). Keep that logic separate from the Cobra command wiring
and from the `tabwriter` presentation so it can be tested directly.

Write tests with Go's standard `testing` package using govmomi's embedded
**`github.com/vmware/govmomi/simulator`** package (NOT the external `vcsim`
binary), so tests are hermetic and run with a plain `go test ./...`. The
`simulator.Test(...)` / `simulator.VPX().Run(...)` helpers give a connected
client against an in-process vCenter.

Provide at minimum:

1. **Port group → VMs** — configure the model so that **known** VMs are attached
   to a **known** port group, then assert the lookup returns **exactly that
   set** (by name). An empty or "any result is fine" assertion does not satisfy
   this — the test must pin the expected VM set and fail if the lookup returns
   the wrong VMs, extra VMs, or none. Exercise **both** a standard port group and
   a distributed port group (either in one test with two lookups, or two tests).

Also unit-test the pure helpers, where the strongest deterministic assertions
live (no simulator needed):

- **Config precedence** — set a config-file value, override with an env var,
  override with a flag, and assert the resolved value follows
  flag > env > file > default.
- **Byte formatting** — table-driven tests for the human-readable GiB/TiB
  formatter.

**Test integrity rules:** every test contains real assertions tied to expected
values. No `t.Skip`, no empty or tautological tests, no rigging a test to pass,
and no loosening an assertion just to get green. In particular, a port-group
lookup test that passes on an empty result set is **not acceptable** — it must
assert the specific expected VMs. `go test ./...` must pass with zero failures
and zero skips.

## Definition of done (acceptance criteria)

1. `go build ./...` produces a working binary that supports
   `vswitches --portgroup <name>`.
2. Viper precedence works: a flag overrides an env var, which overrides the
   config file, which overrides the default.
3. `vswitches --portgroup <name>` lists the connected VMs for **standard and
   distributed** port groups.
4. VM rows report consumed (committed) storage, not provisioned.
5. Errors are wrapped and surfaced; no panics; context timeout is honored.
6. `go test ./...` passes with zero failures and zero skips, with at least the
   meaningful port-group simulator test (asserting the exact VM set for both a
   standard and a distributed port group) plus pure-function tests for config
   precedence and byte formatting.

All of these are fully verifiable locally (unit tests + the `vcsim` loop).

## Self-verification loop (required — use the local `vcsim` simulator)

Do not consider the task complete until the program builds **and runs cleanly**
against govmomi's bundled vCenter simulator, `vcsim`. Work in a
build → run → diagnose → fix loop and repeat until the lookup executes without
error and produces well-formed output.

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

**Discovering a port-group name for the manual run.** Because the default
`vswitches` listing is out of scope here, discover a port-group name another way:
the default VPX model exposes a distributed port group (commonly `DC0_DVPG0`) and
a standard port group (commonly `VM Network`); names can vary by simulator
version. The **authoritative** proof of correctness is the hermetic unit test
above, which attaches known VMs to a known port group and asserts the exact set —
the manual run is a smoke check.

**Drive the code path** against the running simulator:

```
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

./<binary> vswitches --portgroup "DC0_DVPG0"     # distributed (name may vary)
./<binary> vswitches --portgroup "VM Network"    # standard (name may vary)
```

**Exit condition — what "achieves its goal" means against `vcsim`:**

- `go build ./...` and `go vet ./...` pass clean.
- `vswitches --portgroup <name>` runs against the simulator with a zero exit
  code, no panics, and no unhandled errors.
- For a port group that has VMs attached, the lookup returns those VMs, sorted by
  name, with aligned columns and a header; for one with none, it returns an empty
  table without crashing.
- The lookup works for both standard and distributed port groups (verified
  authoritatively by the unit test).

**Do not fabricate data.** Never hardcode, fake, stub, or special-case values
(including simulator port-group names or VM counts) to make the output look
complete, and never loosen the spec to force a pass. Report only what you can
truthfully derive from the API.

## Deliverables

- Complete source for every file.
- `go.mod` (and a note on running `go mod tidy`).
- Project layout (directory tree).
- Build and run instructions, including an example `config.yaml` and an example
  using environment variables.
- A script or `Makefile` target (e.g. `make verify`) that automates the full
  check: runs `go vet ./...` and `go test ./...`, then starts `vcsim` in the
  background, waits for it to be ready, runs a `vswitches --portgroup` invocation
  against it, and exits non-zero on any failure. Tear the simulator down when
  finished.
- A short note confirming the code was actually run: paste a sample
  `go test ./...` result and a sample `vcsim` run.
- The same code must also be deployable against a live vCenter.
