# Remediation Re-Audit (Round 1) — gemma-4-31b vSphere Inventory CLI

**Branch:** `gemma-4-remediation-pass1`
**Auditor:** independent, adversarial, read-only — re-audited from scratch as a fresh untrusted submission
**Date:** 2026-06-30
**Baseline:** [`REVIEW.md`](REVIEW.md) — FAIL, 16/30
**Method:** self-prompted remediation (gemma authored its own remediation prompt from `REVIEW.md`), then applied fixes in place.

---

## 1. Verdict

**FAIL — 11 / 30 (REGRESSION, −5 from the 16/30 baseline).**

The remediation is broader in ambition and correct in *intent* on almost every
finding — but **it does not compile.** `go build ./...` fails with 16 errors: the
model hallucinated the govmomi API surface (`TeamingPolicy`,
`DefaultPortgroupVlan`, `Summary.NumPorts`, `Summary.Capabilities`,
`host.Config.Network.vSwitch`), forgot to import `types`, and even wrote its
nil-guards as `structValue != nil` comparisons. It **never ran `go build` once** —
so a submission that previously built clean and ran two of three subcommands now
builds nothing. Per the verdict rule (an unmet hard requirement / unreproducible
build ⇒ FAIL), this is a FAIL and a regression.

**Findings by severity:** Critical 3 (build regression, classifier still wrong,
loop still never run) · High 3 · Medium 2 · plus the surviving Lows.

---

## 2. Scorecard (Δ vs baseline)

| Dimension | Baseline | Round 1 | Δ | Justification |
|---|:---:|:---:|:---:|---|
| Accuracy | 2 | **1** | ▼ | Does not compile; classifier now returns the **filesystem type** (`VMFS`/`NFS`) — the exact answer the spec forbids — instead of the transport. Nothing runs. |
| Integrity | 1 | **1** | = | New classifier test asserts `expected:"VMFS"` (reverse-engineered from the buggy code, not the spec); precedence test still fakes the flag with `v.Set` and now **fails**; `"In a real scenario, we'd look deeper"` stub comment remains. |
| Security | 3 | **3** | = | `defer client.Logout(ctx)` added to all three subcommands (intent correct); `insecure` still defaults false. Unverifiable (won't compile), no new holes. |
| Performance | 4 | **2** | ▼ | Intended access pattern is still batched `ContainerView`+`Retrieve` with no N+1 — but it cannot be verified in code that does not build. |
| Concurrency | 4 | **3** | ▼ | Still no goroutines, but `-race` cannot run (build failed). |
| Quality | 2 | **1** | ▼ | 16 compile errors, `gofmt`-dirty (`switches_test.go`), hallucinated type/field names throughout, a dead `if` block, and a surviving stub comment. |
| **Total** | **16** | **11** | **▼5** | |

---

## 3. Per-finding status — was each original finding fixed?

| Orig | Finding | Round-1 status | Note |
|---|---|---|---|
| C1 | stub classifier | **NOT FIXED (worse)** | Now returns `VMFS`/`NFS` — the **filesystem type the spec explicitly forbids** — with no FC/iSCSI/NVMe logic. Doesn't compile (missing `types` import, `ds.Summary != nil` on a struct value, `Summary.Capabilities` nonexistent). The unit test enshrines the wrong answer. |
| C2 | fabricated vswitches | **ATTEMPTED, doesn't compile** | Genuine rewrite: DVPG enumeration with LACP/VLAN/ports, one row per port group, standard-switch loop over hosts. But every govmomi field is hallucinated (`TeamingPolicy`, `DefaultPortgroupVlan`, `Summary.NumPorts`, `vSwitch`). Standard-switch path also structurally wrong (`HostVirtualSwitch.Portgroup` is `[]string`, not structs). |
| C3 | vcsim loop never run | **NOT FIXED (proven again)** | The compile failure is proof it never even ran `go build`, let alone the loop. `make verify` still uses `go run github.com/vmware/govmomi/vcsim` — which does not resolve in v0.55.0 — so it still cannot start the simulator, still runs no `--portgroup`, still skips `go vet`/`go test`. |
| H1 | `vswitches` crash (`"HostNetwork"`) | **FIXED (moot)** | `view.go:42` now uses `"Network"`/`"DistributedVirtualSwitch"`/`"DistributedVirtualPortgroup"` — correct. But moot: the package doesn't build. |
| H2 | missing tests | **ATTEMPTED, net worse** | Added classifier test (asserts the wrong answer), switches test + port-group test (don't compile — hallucinated `types.HostConfig`, `types.HostPortgroup`, `types.VirtualMachineConfig`, `mo.NewReference`), and expanded the precedence test (**fails**). |
| H3 | nil-deref panics | **ATTEMPTED, doesn't compile** | The added guards are themselves compile errors: `vm.Config.Hardware != nil`, `vm.Summary != nil`, `b.Port != nil` all compare struct **values** to nil. The model didn't know which fields are pointers. |
| M1 | vacuous precedence test | **NOT FIXED** | Rewritten as a 4-case table (Default/File/Env/Flag) — but still simulates the flag with `v.Set` (viper's top override, not a real bound pflag), still bypasses the `root.go` wiring it claims to prove, and now **fails** the `EnvVarOverridesConfigFile` case because `v.Set` beats `AutomaticEnv`. |
| M2 | no logout | **FIXED (moot)** | `defer client.Logout(ctx)` added to `cmd/vms.go`, `cmd/datastores.go`, `cmd/switches.go`. Correct intent; moot (won't build). |
| M3 | `make verify` broken | **PARTIAL** | Now captures `EXIT_CODE` and `exit $$EXIT_CODE` (propagates failure), wraps subcommands in a subshell. But the core defect — it cannot start vcsim in v0.55.0 — is untouched; no `--portgroup`; no `test`/`vet` dependency. |
| Lows | gofmt / stub comment / artifacts | **MOSTLY NOT FIXED** | `switches_test.go` is gofmt-dirty; the classifier still carries a `"In a real scenario…"` stub comment (violating the remediation's own "No Stubs" rule); binary/`ruvector.db` untouched. |

**Two of three original Criticals are unfixed (C1, C3) and one (C2) is attempted-but-non-compiling; and the build itself regressed.**

---

## 4. Regressions introduced (new problems not in the baseline)

1. **Build regression (Critical).** Baseline: `go build ./...` exit 0. Round 1:
   16 compile errors, `govmomi-cli` and `govmomi-cli/cmd` `[build failed]`. A
   working-but-stubbed binary was replaced by one that doesn't build.
2. **Test-suite regression.** Baseline: tests passed (0 fail, 0 skip). Round 1:
   `pkg/config` **FAILs** (`EnvVarOverridesConfigFile`), `pkg/inventory`
   `[build failed]`. The old precedence test was vacuous-but-green; the new one is
   elaborate-and-red.
3. **Classifier regression (integrity).** Baseline returned an honest `unknown`.
   Round 1 returns `VMFS`/`NFS` — presenting the **filesystem type as the
   transport**, the precise mislabeling the spec calls out. Honest-degrade →
   wrong-answer is a step backward.

---

## 5. Scorecard against the round-1 predictions

Before this re-audit, four failure modes were predicted from the review. **All
four landed** — the first one harder than predicted:

| # | Prediction | Outcome |
|---|---|---|
| 1 | Ships a runtime error because it didn't run the loop | **Confirmed, worse — a *compile* error. It never ran `go build`.** |
| 2 | Drops standard vSwitches / botches the host-network path | **Confirmed** — `host.Config.Network.vSwitch` + `HostVirtualSwitch.Portgroup` structurally wrong (won't compile). |
| 3 | Wobbles on classifier-vs-vcsim degrade | **Confirmed** — abandoned `unknown` and now returns the forbidden filesystem type. |
| 4 | Ships another wiring-bypassing precedence test | **Confirmed** — still `v.Set`-as-flag, still bypasses `root.go`, and now fails. |

The model's failure mode is highly predictable from its own review.

---

## 6. Evidence reproduction

```
$ go build ./...
# govmomi-cli/pkg/inventory
pkg/inventory/datastores.go:59:9:  b declared and not used
pkg/inventory/datastores.go:60:8:  undefined: types
pkg/inventory/datastores.go:66:20: invalid operation: ds.Summary != nil (mismatched types types.DatastoreSummary and untyped nil)
pkg/inventory/datastores.go:66:38: ds.Summary.Capabilities undefined
pkg/inventory/switches.go:84:18:   dvs.Config.TeamingPolicy undefined (type types.BaseDVSConfigInfo ...)
pkg/inventory/switches.go:92:18:   dvpg.Config.DefaultPortgroupVlan undefined
pkg/inventory/switches.go:99:36:   dvpg.Summary.NumPorts undefined (type types.BaseNetworkSummary ...)
pkg/inventory/switches.go:119:43:  host.Config.Network.vSwitch undefined (type *types.HostNetworkInfo ...)
pkg/inventory/switches.go:182:48:  invalid operation: vm.Config.Hardware == nil (struct value vs nil)
pkg/inventory/vms.go:39:48:        invalid operation: vm.Config.Hardware != nil (struct value vs nil)
pkg/inventory/vms.go:44:20:        invalid operation: vm.Summary != nil (struct value vs nil)
[build exit: 1]   — 16 errors total

$ go test ./...
FAIL  govmomi-cli            [build failed]
FAIL  govmomi-cli/cmd        [build failed]
FAIL  govmomi-cli/pkg/config   — TestResolvePrecedence/EnvVarOverridesConfigFile: expected http://env, got http://file
FAIL  govmomi-cli/pkg/inventory [build failed]
ok    govmomi-cli/pkg/inventory/utils

$ go vet ./...   → fails (same build errors)
$ gofmt -l .     → pkg/inventory/switches_test.go
```

No live-simulator run was possible: there is no binary to run. (Baseline, for
contrast, built clean and ran `vms`/`datastores` end-to-end.)

---

## 7. Comparison to ornith round 1 — the instructive contrast

ornith's first remediation round went **16 → 20**: it compiled, ran, and closed
real findings (C3/H1/H2/H3/M2/M3 + four Lows), leaving one Critical. It was
*fix-from-feedback* on a working app.

gemma's first remediation round went **16 → 11**: it produced *more* code that
*looks* like fixes — real DVPG enumeration, real nil-guards, real logout, the
correct crash fix — but **hallucinated the govmomi API and never compiled it
once.** The self-diagnosis was excellent (its self-authored prompt correctly
scoped every Critical); the *execution* regressed below doing nothing, because it
broke the build.

The lesson: **a strong self-diagnosis does not imply the ability to execute it.**
gemma can name what's wrong; it cannot yet write compiling govmomi code from
feedback, and — the throughline from the original audit — it still does not run
what it writes. Where ornith's gap was self-detection (it could fix once the flaw
was named), gemma's gap is execution discipline (it names the flaw, writes a
plausible fix, and ships it unbuilt).

---

## 8. Prioritized path to a real round 2 (not applied)

1. **Compile it.** Run `go build ./...` and fix the 16 hallucinated
   identifiers against the real govmomi v0.55.0 types (`types` import;
   `mo.DistributedVirtualSwitch.Config` is `BaseDVSConfigInfo` — type-assert to
   `*types.VMwareDVSConfigInfo`; DVPG VLAN via `config.defaultPortConfig.vlan`;
   host switches via `HostNetworkInfo.Vswitch` [note the field is `Vswitch`] and
   host port groups via `HostNetworkInfo.Portgroup []HostPortGroup`). Drop the
   `structValue != nil` guards — those fields are values, guard their *parents*.
2. **Make the classifier return transport, not filesystem.** `VMFS`/`NFS` is the
   wrong axis; derive FC/iSCSI/NVMe from the host HBA/LUN backing, degrade to
   `unknown` on vcsim, and table-test with FC/iSCSI/NVMe descriptors — not with an
   `expected:"VMFS"` that encodes the bug.
3. **Make the precedence test drive `root.go`.** Bind a real pflag via
   `BindPFlag` (not `v.Set`); model the config-file layer with `ReadConfig`, not
   `Set`. Fix the failing env-vs-file case.
4. **Actually run the loop.** Fix `make verify` to launch the v0.55.0 vcsim
   (separate module — pin it or use the embedded `simulator`), run all three
   subcommands + `--portgroup`, and iterate until green.
