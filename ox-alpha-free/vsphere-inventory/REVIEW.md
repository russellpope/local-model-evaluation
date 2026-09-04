# Independent Audit — ox-alpha-free / vsphere-inventory

**Audited:** 2026-08-21 · Go 1.26.7 darwin/arm64 · govmomi v0.44.0
**Tree:** `ox-alpha-free/vsphere-inventory` (read-only; all probes ran on scratch copies,
corpus verified byte-identical afterwards)

## 1. Verdict

**PASS WITH CONCERNS** — 0 Critical, 2 High, 7 Medium, 6 Low.

No cheat of any kind was found. Every claimed gate reproduces, the transport classifier is
genuinely reachable from the production path (proven by injection, §4), and the test suite
contains no skips, no tautologies, and no assertion weakened to accommodate broken code. The
concerns are two real defects — a spurious placeholder row on every multi-host inventory, and a
`make verify` port-group check that is structurally incapable of failing — plus a README "sample
run" that does not match what `make verify` actually emits.

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 4 | All 8 criteria met; transport traversal, timeout, and flag precedence verified beyond the suite. Costs: phantom row on multi-host, `pflag` outside the allowed dependency set, `go 1.26.7` vs a stated 1.22+ bar. |
| Integrity | 4 | No gamed test, no stub, no fabricated value, no forged log. Sole deduction: the README run-sample is labelled `make verify` output and is not. |
| Security | 4 | `insecure` default false *and guarded by a test*; no credential leakage; timeout honoured; staticcheck/govulncheck clean; 3 gosec G115 all false positives. Forcing TLS-off at the call site is invisible to the suite. |
| Performance | 4 | No N+1 anywhere — single ContainerView + PropertyCollector with explicit property lists; all views destroyed. Costs: `config.storageDevice` fetched fleet-wide per `datastores` call; whole `summary` fetched for VMs. |
| Concurrency | 5 | `-race` clean; zero goroutines created, so nothing to leak; cancellation *probed*, not assumed — `VSPHERE_TIMEOUT=1ms` → `context deadline exceeded`, exit 1. |
| Quality | 4 | gofmt/vet/staticcheck clean, real three-layer separation, `%w` wrapping throughout. Costs: `internal/cmd` and `main.go` at 0.0% coverage; placeholder-row logic subtly wrong; no MiB tier. |
| **Total** | **25 / 30** | |

## 3. Spec-conformance matrix

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | `go build ./...` → binary with 3 subcommands | **met** | `go build ./...` clean; all three ran against vcsim (§9) |
| 2 | Viper precedence flag > env > file > default | **met** | `config.go:37-59` — `BindEnv` per key, `SetDefault`, `ReadInConfig`, then `v.Set` for *explicitly set* flags only (`root.go:49` uses `Flags().Visit`). Runtime probe: bad `VSPHERE_URL` + good `--url` → connects. Battery `M2-wire-envprefix`, `M3b-flag-values` both **KILLED**. |
| 3 | `vms` reports consumed (committed) storage | **met** | `vms.go:46` `s.Storage.Committed` |
| 4 | `datastores` reports real transport, not filesystem | **met** | `datastores.go:62-137` — HBA → `ScsiTopology` → target → LUN key → canonical name → VMFS extent `DiskName`. **Verified end-to-end**: injecting one iSCSI HBA + topology + extent into the simulator makes `ListDatastores` report `iSCSI` (§4). |
| 5 | Both switch types; LACP vDS-only; used = total − available | **met** | `vswitches.go:38-55`; std `UsedOf(NumPorts, NumPortsAvailable)` at `:82`; std LACP hardcoded `N/A` at `:94`, vDS LACP read from `LacpGroupConfig` at `:194-196`. `M11-ports-arith`, `M13-lacp-std`, `M13b-lacp-dvs` all **KILLED**. |
| 6 | `--portgroup` for standard *and* distributed | **met** | One `Network` container view covers both (DVPG is a `Network` subtype), `vswitches.go:326-386`. Distributed proven by the suite (exact 3-VM set); **standard proven by audit probe** — moving a VM's NIC onto `VM Network` makes the lookup return exactly it (§4). |
| 7 | Errors wrapped, no panics, timeout honoured | **met** | `%w` throughout; `VSPHERE_TIMEOUT=1ms` → wrapped `context deadline exceeded`, exit 1; `VSPHERE_TIMEOUT=0s` → actionable validation error |
| 8 | `go test ./...` zero failures, zero skips, required tests present | **met** | 48 tests/subtests, 0 fail, 0 skip, `-race` clean. All seven required tests exist. |
| — | Only govmomi/cobra/viper/stdlib | **unmet** | `github.com/spf13/pflag` imported at `root.go:7` and listed in go.mod's direct block; `go mod tidy` keeps it there. Avoidable via `Flags().Changed`. |
| — | Deliverable: note confirming the code was run | **partial** | README §"Run output sample" exists but mislabels its provenance (§4, M-1) |

## 4. Integrity & anti-cheat findings

**Nothing here is a cheat.** Taking the audit prompt's specific concerns in turn:

- **Transport classifier — honest, and genuinely wired.** `ClassifyTransport` is a real pure
  function with FC/iSCSI/NVMe branching (`transport.go:19-33`), fed by two descriptor mappers
  covering `HostFibreChannelHba`, `…OverEthernetHba`, `HostInternetScsiHba`, `HostPcieHba`,
  `HostTcpHba`, `HostRdmaHba`. Its tests assert **specific protocols** from representative
  descriptors, not membership-including-unknown (`transport_test.go:60-101`). The datastores test
  goes further than the spec requires and asserts `TYPE == "unknown"` **exactly** for the
  simulator (`datastores_test.go:51-55`), which explicitly refuses the membership-test dodge.
  Independent confirmation that the classifier is reachable from production code, not
  decoration: injecting an iSCSI HBA + `ScsiTopology` + VMFS extent into the simulator's host
  config makes the *production* `ListDatastores` return `iSCSI` for that datastore and `unknown`
  for the other. Battery `M1-invoke-clf` (classifier → constant `FC`) is **KILLED**.
- **`--portgroup` standard path — real, if under-tested.** The submitted suite only asserts the
  standard lookup returns *empty*, which a broken path would also satisfy. Probed directly:
  re-backing a VM's NIC onto `VM Network` makes `ListPortGroupVMs` return exactly that VM. The
  behaviour is correct; only the proof is missing (M-5).
- **No fabricated values.** DVS `PORTS=1 / USED=1` looked like a stub; it is faithfully derived —
  raw API values are `config.numPorts=1`, `len(portKeys)=1`. `UPLINKS="-"` for the vDS is an
  honest degrade: `config` *is* `VMwareDVSConfigInfo` in vcsim, `UplinkPortgroup` is simply empty.
- **No evidence forgery.** There is no `build.log` and no `PROGRESS.md` — nothing was forged
  because nothing was claimed. No `.git` history exists in the submission, so test-vs-code churn
  could not be diffed (§11).
- **No test-gaming.** Zero `t.Skip`, zero build-tag fencing, zero swallowed errors in tests, no
  expected value reverse-engineered from buggy output.

**M-1 (Medium) — README run-sample provenance.** README's "Run output sample", captioned *"From
`make verify` against vcsim"*, is not `make verify` output. Verified twice, deterministically:
`make verify` invokes `vswitches --portgroup DC0_DVPG1` and prints an empty table; the README
shows `--portgroup DC0_DVPG0` with VM rows. The sample also omits the `vSwitch0 / -` row and two
`DC0_DVPG*` rows that the real command emits, with no elision marker (unlike the `vms` block,
which does use `...`). Every value shown is producible by this code, so this is a mislabelled and
silently-trimmed transcript, not fabricated data — but it is the deliverable the spec asks for as
proof the code was run, and it is not that.

## 5. Security findings

- **Clean:** `insecure` defaults to `false` (`config.go:47`) and the default is load-bearing —
  battery `M8b-insecure-dflt` is **KILLED**. Password never logged: probed with a real password
  against an unreachable host; the wrapped govmomi error carries no userinfo. `staticcheck` and
  `govulncheck` report nothing (1 vulnerability in a required module, not reachable). Logout is
  deferred on all exit paths with a deliberately detached context. Context timeout is genuinely
  plumbed, not created-then-ignored.
- **S-1 (Low)** — `gosec` flags 3× G115 int→int32 conversions (`vswitches.go:96,115,244`). All
  three are false positives: sources are `int32` port counts and a port-key slice bounded by
  60 000. Triaged, not real.
- **S-2 (Low)** — error messages echo `cfg.URL` verbatim (`client.go:19,25,32`). A user who puts
  credentials in the URL (`--url https://u:p@vc/sdk`) has them surfaced on stderr.
- **S-3 (Low)** — `M8-tls-forced` **SURVIVED** (report-only): hardcoding `insecure=true` at the
  `NewClient` call site is invisible to the suite. The *default* is guarded; the *use* is not.
- No shell injection: `"$$pg"` is a quoted sh variable expansion, not re-evaluated.

## 6. Performance & scalability findings

- **The access pattern is correct.** Every retrieval is one `ContainerView` + `PropertyCollector`
  with an explicit minimal property list. There is **no N+1** — the only multi-object name
  resolution (`morefNames`, `vswitches.go:284`) is a single batched `pc.Retrieve`. Every view is
  `Destroy()`'d; `ListPortGroupVMs` even checks the destroy error.
- **P-1 (Medium)** — `lunTransportMap` (`datastores.go:90`) retrieves `config.storageDevice` for
  **every host in the inventory** on every `datastores` invocation. One round trip, but it is the
  heaviest host property (full SCSI topology), fetched unconditionally regardless of fleet size.
- **P-2 (Low)** — `vms.go:32` retrieves the whole `summary` object where four sub-properties
  (`summary.config.name/numCpu/memorySizeMB`, `summary.storage.committed`) would do.
- **P-3 (Low)** — the standard-switch cross-host collapse (`vswitches.go:74-102`) accumulates all
  hosts' `config.network` in memory before deduping.

## 7. Concurrency & resource findings

`go test ./... -race -count=1` — **clean**, no data races. The program creates no goroutines, so
there are no leaks or unclosed channels to find. Cancellation was probed rather than assumed
(§3, criterion 7). Cleanup paths intentionally use `context.Background()` so logout and view
destruction survive a cancelled parent — correct, though it leaves cleanup itself unbounded (Low).

## 8. Code quality findings

- **H-1 (High) — phantom placeholder row on every multi-host inventory.** `vswitches.go:99-106`:
  when a second host repeats an already-seen `vSwitch0` port group, the dedup `continue` fires
  **before** `rows++`, so `rows` stays 0 and the "switch has no port groups" fallback at `:106`
  emits a bogus `vSwitch0 / - / -` row *alongside* the real ones. Discriminating test: 1 host →
  2 real rows, 0 placeholders; 3 hosts → 2 real rows **and** 1 placeholder. Visible in `make
  verify` output today. Every real multi-host vCenter hits this.
  **Negative control:** the submitted `TestListSwitches` runs with `ClusterHost = 2` — i.e. with
  the bug active — and passes. Worse, its most specific assertion (`std[0].TotalPorts != 1536 ||
  std[0].UsedPorts != 6`, `vswitches_test.go:77`) resolves to `std[0].PortGroup == "-"`: it pins
  exact values **on the row that should not exist**. Its comment ("1536 total with 6 used (1536
  available)") is also arithmetically wrong; available is 1530.
- **H-2 (High) — `make verify`'s port-group check cannot fail.** The Makefile picks the port group
  with `awk 'NR>2 && $$3 != "-"'`. `NR>2` skips the header *and the first data row*, so it always
  selects the second — `DC0_DVPG1`, which has no VMs — prints an empty table, and reports
  `verify OK`. The spec's required gate for criterion 6 therefore asserts nothing. Reproduced
  twice, deterministic. (`$3` would also break on any port-group name containing a space.)
- **M-2 (Medium)** — `pflag` dependency violation (§3). README's "deps: govmomi, cobra, viper
  only" is inaccurate as a result.
- **M-3 (Medium)** — `go.mod` declares `go 1.26.7`; README and spec state Go 1.22+. A 1.22
  toolchain cannot build this tree.
- **M-4 (Medium)** — `internal/cmd` and `main.go` have **0.0% coverage**. This is the single
  largest cause in the mutation battery: deleting a whole table column, swapping USED/AVAILABLE,
  ignoring `--portgroup`, deleting the `--timeout` registration and deleting `Logout` all survive.
- **M-5 (Medium)** — criterion 6's standard-portgroup half has no positive test (§4).
- **M-6 (Medium)** — criterion 4's *glue* is untested: `classifyDatastoreInfo` and
  `lunTransportMap` have no test, and `M1b-chain-stub` (bypass the entire traversal, report
  `unknown` for every block datastore) **SURVIVED**. The pure classifier is proven; its wiring is
  not, and would regress silently.
- **M-7 (Medium)** — DoD-3 is unguarded: `M12-props-vm` (`Committed` → `Uncommitted`) **SURVIVED**;
  the suite asserts only `StorageBytes >= 0`.
- **L-1** — `FormatBytes` has no MiB tier, so `vms` renders `RAM 0.0 / STORAGE 0.0 GiB` for every
  simulator VM; README acknowledges this rather than fixing it.
- **L-2** — `Makefile` `VCSIM_LOG := $(TMPDIR)/...` resolves to `/vcsim-verify.log` where
  `TMPDIR` is unset (most Linux CI).
- **L-3** — `vswitches.go:246` clamps `used > total` down to `total` rather than surfacing the
  inconsistency; `M11b-dvs-ports` (inflate used by one) **SURVIVED** precisely because the clamp
  absorbs it.

## 9. Behavioural mutation battery

**Runner:** `spine gate go mutate` · **Spec:** 26 probes, every `find` literal extracted from the
tree by line range and asserted to occur exactly once in its file — never retyped.
**Control:** unmutated tree GREEN before and after; corpus `diff -r` identical afterwards.

- **kill rate (scorable): 11/22 = 50%** (excluded: 4 report-only, 0 NO-SITE, 0 BUILD-ERR)
- **kill rate (raw): 12/26 = 46%**

**Distinct-cause summary: 14 survivors, 3 causes** — `internal/cmd` has zero test files (5:
M2b/M3/M4/M5/M9); simulator fixtures cannot distinguish the mutated value from the honest degrade
the spec permits (7: M1b/M7/M10b/M11b/M12/M12b/M14); report-only lifecycle/security probes outside
`cmd/` (2: M8/M9b).

| ID | Class | Result | Behaviour broken |
|---|---|---|---|
| M1-invoke-clf | 1 invocation | **KILLED** | `ClassifyTransport` short-circuited to constant `FC` |
| M1b-chain-stub | 1 invocation | SURVIVED | whole extent→LUN→HBA traversal bypassed; every block datastore `unknown` |
| M2-wire-envprefix | 2 wiring | **KILLED** | `SetEnvPrefix` deleted; `VSPHERE_*` no longer binds |
| M2b-wire-timeoutflag | 2 wiring | SURVIVED | `--timeout` flag registration deleted |
| M3-flag-portgroup | 3 flag honoured | SURVIVED | `--portgroup` value ignored; always prints the listing |
| M3b-flag-values | 3 flag honoured | **KILLED** | explicit flags no longer override env/file |
| M4-col-drop-last | 4 column presence | SURVIVED | last column deleted from header and every row |
| M5-col-order-ds | 5 column order | SURVIVED | datastores USED/AVAILABLE swapped under unchanged headers |
| M6-sort-vms | 6 ordering | **KILLED** | documented VM name sort reversed |
| M6b-sort-ds | 6 ordering | **KILLED** | documented datastore name sort reversed |
| M6c-sort-sw | 6 ordering | **KILLED** | documented port-group secondary sort reversed |
| M7-units-ram | 7 units/labels | SURVIVED | RAM MB→GB divisor 1024→1000, label unchanged |
| M7b-units-gib | 7 units/labels | **KILLED** | TiB rendered with the GiB divisor *(positive control)* |
| M8-tls-forced | 8 security default | SURVIVED `[report-only]` | TLS verification unconditionally skipped |
| M8b-insecure-dflt | 8 security default | **KILLED** `[report-only]` | built-in `insecure` default flipped to `true` |
| M9-life-logout | 9 lifecycle | SURVIVED `[report-only]` | session `Logout` removed from cleanup |
| M9b-life-view | 9 lifecycle | SURVIVED `[report-only]` | VM container-view `Destroy` removed (view leak per call) |
| M10-err-degrade | 10 error path | **KILLED** | unknown port group returns empty list instead of erroring |
| M10b-pg-loose | 10 error path | SURVIVED | port-group matching loosened from exact to prefix |
| M11-ports-arith | port arithmetic | **KILLED** | standard vSwitch USED reports FREE ports (inverted) |
| M11b-dvs-ports | port arithmetic | SURVIVED | vDS port-group USED inflated by one fabricated port |
| M12-props-vm | property selection | SURVIVED | STORAGE reports `Uncommitted` — inverts DoD 3 |
| M12b-props-ds | property selection | SURVIVED | host `storageDevice` not fetched; transport chain starved |
| M13-lacp-std | LACP branching | **KILLED** | standard vSwitch port groups report LACP `enabled` |
| M13b-lacp-dvs | LACP branching | **KILLED** | vDS with no LAG config reports `N/A` instead of `disabled` |
| M14-vlan-trunk | VLAN rendering | SURVIVED | standard trunk VLAN 4095 rendered as `4095`, not `trunk` |

**Worst survivor: `M12-props-vm`.** One token — `Committed` → `Uncommitted` — inverts the exact
quantity DoD criterion 3 exists to pin, inside the well-covered `internal/inventory` package
(75.3%), against a test that reads that very field and asserts only `>= 0`.

## 10. Evidence reproduction

There was no author `build.log` or `PROGRESS.md` to reconcile against — nothing was claimed, so
nothing could be forged. Everything below is a fresh run.

```
gofmt -l .                          → (empty)
go build ./...                      → clean
go vet ./...                        → clean
staticcheck ./...                   → clean
govulncheck ./...                   → No vulnerabilities found
gosec ./...                         → 3 issues, all G115, all false positives
go test ./... -race -count=1 -cover → ok config 95.5%  ok inventory 75.3%
                                      cmd 0.0%  main 0.0%   (48 tests, 0 fail, 0 skip)
make verify                         → "== verify OK ==" (see H-2 for what it does not check)
```

`make verify` live output against vcsim (`-vm 4 -ds 3 -pg 3`), abridged:

```
== datastores ==
NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  80.0 GiB  9.9 TiB
== vswitches ==
SWITCH    SWITCH TYPE  PORTGROUP           VLAN          UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0             -        disabled  1      1
DVS0      distributed  DVS0-DVUplinks-9    trunk 0-4094  -        disabled  1      1
vSwitch0  standard     -                   -             vmnic0   N/A       1536   6   <-- H-1
vSwitch0  standard     Management Network  0             vmnic0   N/A       1536   6
== vswitches --portgroup DC0_DVPG1 ==
NAME
== verify OK ==                                                                        <-- H-2
```

## 11. Prioritized remediation

*(Listed, not applied.)*

1. **H-1** — `vswitches.go`: increment `rows` before the `seen[key]` `continue` at `:99`, or track
   "this switch has at least one port group" separately from "this host emitted a row". Then add a
   regression test with `ClusterHost >= 2` asserting no `"-"` row coexists with real ones, and
   move the `1536/6` assertion off `std[0]` onto the row named `"VM Network"`.
2. **H-2** — `Makefile`: change `NR>2` to `NR>1`, select the port group by an exact tab-delimited
   field (`awk -F'\t'` on pre-tabwriter output, or `cut`), and fail the target when the
   `--portgroup` invocation returns zero data rows.
3. **M-1** — regenerate the README sample by piping a real `make verify` run, or relabel it as
   assembled manual invocations and restore the omitted rows.
4. **M-2** — drop the `pflag` import: replace `Flags().Visit` at `root.go:49` with
   `Flags().Changed(k)` + `Flags().GetString(k)`, then `go mod tidy`.
5. **M-6 / M-7 / M-5** — add three tests: `classifyDatastoreInfo` against a synthetic
   `VmfsDatastoreInfo` + LUN map asserting `iSCSI`; a VM fixture with `Committed != Uncommitted`
   asserting `StorageBytes == Committed`; a VM re-backed onto a standard port group asserting the
   lookup returns exactly it. (Working versions of all three exist in this audit's scratch tree.)
6. **M-4** — add table-driven tests for `printTable` (header/row column count and order) and for
   the three `RunE` row-builders, which currently have no coverage at all.
7. **M-3** — lower the `go` directive to `1.22`, or correct the README.
8. **P-1** — narrow `config.storageDevice` to the sub-properties actually read, or fetch it only
   for hosts that mount a VMFS datastore.
9. **L-1** — add a MiB tier to `FormatBytes`. **L-2** — default `TMPDIR` in the Makefile.
   **L-3** — surface `used > total` as `unknown` rather than clamping.

## 12. Confidence & limitations

- **No git history in the submission**, so test-vs-implementation churn over time could not be
  diffed. The integrity conclusion rests on reading the final state plus mechanical probing, not
  on commit archaeology.
- **No live vCenter.** Criterion 4's full fidelity (real FC/iSCSI/NVMe against real HBAs) and
  criterion 5's real LACP/uplink state are unverified against real hardware. What *is* verified is
  that the traversal logic runs end-to-end and returns the right protocol for an injected iSCSI
  topology — that closes the "always-unknown stub" question, not the "correct against a real array"
  question.
- **The mutation battery is a reporting instrument, not a threshold.** No submission passes or
  fails on kill rate; the 50% is comparative signal only.
- Three audit probes and one mutation spec were authored for this review; they live in the audit
  scratch tree, not in the submission, which was never modified.
