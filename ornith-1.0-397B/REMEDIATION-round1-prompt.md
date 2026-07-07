<!--
Round-1 remediation prompt — SELF-PROMPTED.
Authored by the model under test (Ornith-1.0-397B) after being asked to read its own
independent REVIEW.md. Captured here verbatim as the prompt driving remediation pass 1.
Not edited by the auditor (eval-prompt-as-instrument: gaps are signal).
-->

## Context

An independent audit of the vSphere inventory CLI produced a **PASS WITH CONCERNS** (22/30).
No critical integrity issues, but several high and medium findings need fixing.
Address all items below. The code must continue to pass `go build ./...`, `go vet ./...`,
`gofmt -l .` (clean), and `go test ./...` (0 failures, 0 skips) after each change.

---

## High Priority

### H1 — Replace per-object N+1 with ContainerView + PropertyCollector

Every retrieval function (`GetVMs`, `GetDatastores`, `getStandardVSwitches`,
`getDistributedVSwitches`) lists objects with `find.Finder` then calls
`Properties()` / `RetrieveOne()` in a loop — one network round-trip per object.

**Fix:** For each feature, create a single `view.CreateContainerView` over the
appropriate managed object type with an explicit minimal property list, call
`RetrieveProperties` once, then `defer view.Destroy(ctx)`. This applies to:
- `GetVMs` — view over `VirtualMachine`, props: `name`, `config.hardware.numCPU`,
  `config.hardware.memoryMB`, `summary.storage.committed`
- `GetDatastores` — view over `Datastore`, props: `name`, `summary.capacity`,
  `summary.freeSpace`, `summary.type`, `info`
- `getStandardVSwitches` — view over `HostSystem`, props: `name`,
  `config.network.vswitch`, `config.network.portgroup`
- `getDistributedVSwitches` — view over `DistributedVirtualSwitch`, props:
  `name`, `config`, `portgroup`

### H2 — Rewrite `GetVMsByPortGroup` to eliminate O(VMs×NICs) inner loop

Current code (`inventory.go:345-403`): distributed path does per-DVS `RetrieveOne`,
then per-pgRef `RetrieveOne`, then unconditionally also lists all VMs and for each
resolves every attached network by `RetrieveOne("name")` — cost ∝ (VMs × NICs).
The two paths are also redundant (a distributed PG is matched by both).

**Fix:** Build one VM→networks map from a single batched retrieve. Resolve the
target PG's moref once. Match against the map. Single scan, no inner loop, no
redundant dual path.

### H3 — Fix `TestGetVMsByPortGroup` — remove t.Skip, add real assertions

Current test (`inventory_test.go:149-188`) has two `t.Skip` calls (lines 160, 173)
which violate the spec rule "No t.Skip". The body only checks `vm.Name != ""`
inside a loop — if `GetVMsByPortGroup` returns `nil`, the loop never runs and the
test passes. It does not assert "returns exactly that set".

**Fix:** Remove both `t.Skip`. Configure the simulator model so known VMs are
attached to a known port group, then assert the lookup returns exactly that set
(by name or count). An empty/broken lookup must fail the test.

---

## Medium Priority

### M1 — Stop swallowing errors

Six real-error paths return empty results with `nil` error instead of propagating:
- `inventory.go:196,202` (std vSwitches host list / properties)
- `inventory.go:256,262` (distributed vSwitch list / properties)
- `inventory.go:350,406` (port-group paths)
- Plus numerous `continue`-on-error inside retrieval loops

A genuine API/permission/context failure becomes indistinguishable from "no data".

**Fix:** Wrap errors with `%w` and propagate. At minimum, the top-level retrieval
failures (host list, DVS list, VM list) must return errors. Inner loop `continue`s
may stay but should log or accumulate a multi-error.

### M2 — Fix `make verify` portgroup extraction

`Makefile:62-74` extracts portgroup name with:
```
awk 'NR>1 && $3!="-" {print $3; exit}'
```
Tabwriter output `vSwitch0\tstandard\tVM Network\t...` makes `$3="VM"` (truncated).
The `--portgroup` check then passes on the graceful no-match path — theater.

Also: `vcsim-stop` records the PID of the `go run` wrapper, not the vcsim child,
so `make verify` can't reap the server and hangs.

**Fix:** Parse the portgroup name correctly (e.g., use `cut` with tab delimiter,
or pick a known DVS portgroup name like `DC0_DVPG0`). Fix `vcsim-stop` to kill
the actual vcsim process (use `pkill -f "govmomi/vcsim"` or record the child PID).

### M3 — Add README

Spec deliverables require build/run instructions, example config, env-var example,
and a sample-run note.

**Fix:** Create a `README.md` with: project overview, prerequisites, build
instructions, config file example, env-var example, usage for all three
subcommands, `make verify` description, and a sample output section.

---

## Low Priority

### L1 — Logout with a fresh context

`defer client.Logout(ctx)` reuses the operation context which may already be
`Done()` near the deadline, causing silent logout failure and orphaned sessions.

**Fix:** Create a fresh short-timeout context for cleanup:
```go
defer func() {
    cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    client.Logout(cleanupCtx)
}()
```

### L2 — `--insecure=false` cannot override a true env/config

`main.go:58-60` only injects `insecure` into flag overrides when `true`:
```go
if flagInsecure {
    flagOverrides["insecure"] = "true"
}
```
If `VSPHERE_INSECURE=true`, there's no way to flag-override back to false.

**Fix:** Always include `insecure` in flag overrides (use a sentinel or pointer
to distinguish "not set" from "explicitly false").

### L3 — Remove dead code, populate HBA types

- `formatter.FormatBytesFloat` is exported but never called — remove it.
- `extractBackingInfo` declares `hbaTypes` as a named return but never populates
  it (only fills `deviceNames`). The classifier's HBA branch is unreachable from
  real data.

**Fix:** Remove `FormatBytesFloat`. For `extractBackingInfo`, walk the datastore's
host mount to find HBA/LUN backing info and populate `hbaTypes`. At minimum,
inspect `ds.Host` references to find `HostMountInfo` and extract HBA adapter
types from the host's storage system.

### L4 — Address gosec/govulncheck findings

- `gosec`: 5× G104 on `v.BindEnv` returns (false positives — `BindEnv` only
  errors on empty key list). Assign or explicitly ignore: `_ = v.BindEnv(...)`.
- `govulncheck`: GO-2026-5024 in `golang.org/x/sys@v0.29.0` (Windows-only,
  unreachable on darwin). Informational — note in README or upgrade if possible.

### L5 — Extract vswitches presentation logic

The multi-row continuation formatting for vswitches is inlined in the RunE
closure. Extract to a presentation helper function for readability.

---

## Acceptance Criteria After Remediation

1. `go build ./...` — clean
2. `go vet ./...` — clean
3. `gofmt -l .` — empty (no files need formatting)
4. `go test ./... -race -count=1` — all pass, 0 failures, 0 skips, no data races
5. `staticcheck ./...` — clean
6. `make verify` — completes successfully (vcsim starts, all subcommands run,
   `--portgroup` returns real VMs, vcsim is reaped on exit)
7. All retrieval functions use `ContainerView` + `PropertyCollector` (no per-object
   `Properties()` loops)
8. `TestGetVMsByPortGroup` has real assertions, no `t.Skip`
9. Error propagation: no silent empty-table-on-error paths
10. README present with all required sections
