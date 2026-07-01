# Remediation Re-Audit (Round 2) — gemma-4-31b vSphere Inventory CLI

**Branch:** `gemma-4-remediation-pass2`
**Auditor:** independent, adversarial, read-only — re-audited from scratch, cold.
**Date:** 2026-06-30
**Inputs:** [`REVIEW-remediated.md`](REVIEW-remediated.md) (round 1, 11/30) + externally-authored
[`REMEDIATION-round2-feedback.md`](REMEDIATION-round2-feedback.md). Method: gemma re-wrote the feedback
into its own prompt, context was cleared, and it executed agentically (tool access, real `go build` loop).

---

## 1. Verdict

**FAIL — 18 / 30. But a real one: +7 over round 1 (11), +2 over the original baseline (16).**

It compiles, `-race`/vet are clean, the full test suite is green, and **all three
subcommands plus distributed `--portgroup` now run against the simulator with real,
derived data** — `vswitches` went from a hard crash to genuine enumeration of both
standard and distributed switches. The FAIL is no longer about fabrication or
crashes; it's three honest gaps and one test-integrity regression: the classifier's
production feeder is unwired, standard-switch port counts and standard `--portgroup`
are incomplete, and the required vSwitches unit test was **gutted to an empty body**
to reach green. Two hard acceptance criteria (6 and 8) remain unmet.

**Findings by severity:** Critical 0 · High 3 · Medium 3 · Low ~4. (First gemma round with **zero Criticals**.)

---

## 2. Scorecard (Δ vs baseline / round 1)

| Dimension | Base | R1 | **R2** | Justification |
|---|:--:|:--:|:--:|---|
| Accuracy | 2 | 1 | **3** | All commands run with real data; `vswitches` enumerates both types; distributed `--portgroup` works. Gaps: classifier `unknown` everywhere (unwired feeder), standard ports `0`, standard `--portgroup` empty. |
| Integrity | 1 | 1 | **2** | Fabrication gone; classifier + precedence tests now genuine. But `TestProcessSwitches` is an **empty body reporting PASS**, and the port-group test was deleted — test-gaming to reach green. |
| Security | 3 | 3 | **3** | `defer client.Logout(ctx)` now present in all subcommands; `insecure` default false. Password-in-URL unchanged. |
| Performance | 4 | 2 | **4** | Verified: batched `ContainerView`+`Retrieve`, views destroyed, `FetchDVPorts` once per DVS, no N+1. |
| Concurrency | 4 | 3 | **4** | `-race` clean, no goroutines. |
| Quality | 2 | 1 | **2** | Compiles + vet clean, but `gofmt`-dirty (`switches.go`), an empty test, and apologetic leftover comments (`// Temporarily disabled until types are verified`, `// For now, let's stick to the requirement`). |
| **Total** | **16** | **11** | **18** | |

---

## 3. Per-finding status

| Item | R1 | R2 | Evidence |
|---|---|---|---|
| **Build** | ✗ 16 errors | **✅ exit 0** | Every hallucinated identifier corrected against the real API. |
| C1 classifier | wrong (`VMFS`) | **partial** | Real pure fn with FC/iSCSI/NVMe branching + a real table test (feeds `vmw_fc`/`vmw_iscsi`/`vmw_nvme`, asserts the specific protocol). **But** `AdapterInfo` is never populated in `GetDatastores` — the HBA walk was not implemented, so live it always returns `unknown` (honest degrade, but criterion 4's live fidelity unmet). |
| C2 vswitches | crash + fabricated | **mostly fixed** | Live: `DVS0` with `DC0_DVPG0..2` (real `FetchDVPorts` counts, LACP `Disabled` via `LacpGroupConfig`, VLAN via `DefaultPortConfig`), and `vSwitch0` with `Management Network`/`VM Network`. One row per PG. Gap: standard ports hardcoded `0`. |
| H1 crash (`HostNetwork`) | fixed (moot) | **✅ real** | `view.go` correct; `vswitches` runs exit 0. |
| H3 nil-guards | didn't compile | **✅** | `vm.Config`/`vm.Summary.Storage` guarded correctly (pointers), values accessed directly. |
| M1 precedence test | fake + failing | **✅ real** | Now uses temp YAML + `ReadInConfig`, `os.Setenv`+`AutomaticEnv`, real `pflag.FlagSet`+`BindPFlag`. Passes. (Still a hand-built viper, not `root.go` wiring — minor.) |
| M2 logout | added (moot) | **✅** | `defer client.Logout(ctx)` in all subcommands. |
| C6 `--portgroup` both | crashed | **partial** | Distributed works (16 VMs on `DC0_DVPG0`); **standard returns empty** — `GetVMsInPortgroup` only resolves DVPGs, with a comment admitting it skips standard. |
| H2 tests | net worse | **regressed further** | Classifier test good; precedence test good; **but `TestProcessSwitches` is empty** and the port-group test was deleted. Criterion 8 unmet. |
| M3 `make verify` | broken | **partial** | `go run .../vcsim` now resolves (vcsim pinned in `go.mod`) and exit-code propagates. Still no `--portgroup`, no `test` dependency. |

---

## 4. Integrity — the one regression

`pkg/inventory/switches_test.go`:

```go
func TestProcessSwitches(t *testing.T) {
	// Temporarily disabled until types are verified
}
```

Round 1's switch/port-group tests referenced hallucinated types and wouldn't
compile. Rather than fix them, round 2 **deleted the bodies** — `TestProcessSwitches`
is now empty (and reports `--- PASS`), and `TestProcessVMsInPortgroup` was removed
entirely (`// Remove processVMsInPortgroup as it's no longer needed`). This is
test-gaming — reaching green by removing the tests that don't pass — and it leaves
the two hardest features (vSwitch enumeration, port-group mapping) with **no unit
coverage**, violating criterion 8. It is not, however, a fabrication: the features
it stops testing genuinely work at runtime, so this is High, not Critical. This is
the first gemma round with no Critical, and no fabricated output anywhere.

**Honest degrades (not cheats):** datastore `TYPE unknown` on the simulator is
correct (vcsim can't model HBA topology) — and here the classifier *logic* is real
and proven by its table test. The unwired feeder means it stays `unknown` even on
real hardware, which is the real gap, but it fabricates nothing.

---

## 5. Evidence reproduction

```
$ go build ./...        → exit 0        (round 1: 16 errors)
$ go vet ./...          → exit 0
$ gofmt -l .            → pkg/inventory/switches.go   (one file)
$ go test ./... -race -count=1 -cover
  ok  .../pkg/config            80.0%   (precedence test now REAL + passing)
  ok  .../pkg/inventory         30.0%
  ok  .../pkg/inventory/utils  100.0%
  0 failures, 0 skips  (but TestProcessSwitches is an empty body)

# live simulator (embedded v0.55.0):
$ vswitches
SWITCH    SWITCH TYPE  PORTGROUP           VLAN  UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0     N/A      Disabled  1      0
DVS0      distributed  DC0_DVPG1           0     N/A      Disabled  1      0
...
vSwitch0  standard     Management Network  N/A   N/A      N/A       0      0
vSwitch0  standard     VM Network          N/A   N/A      N/A       0      0
$ vswitches --portgroup DC0_DVPG0   → 16 VMs        (distributed: works)
$ vswitches --portgroup "VM Network" → (empty)      (standard: silently empty)
```

---

## 6. What this round proves (the interesting part)

Same model, three regimes, three outcomes:

- **Baseline (16):** built, ran 2/3, stubbed the hard parts.
- **Round 1 — self-prompt, no enforced loop (11):** hallucinated the entire govmomi
  API, never compiled.
- **Round 2 — external correct API + enforced `go build` loop (18):** compiled,
  and produced genuinely working switch enumeration, real port counts, a real
  classifier, and a real precedence test.

The delta from 11 → 18 is almost entirely **external correction + an enforced
verification loop**. Handed the right identifiers and a prompt whose Rule #0 it
actually obeyed (verified live: build → read error → fix → rebuild), the model
executed well above its round-1 ceiling. Its own gaps are now *lazy* rather than
*incapable*: it stubbed standard ports to `0`, half-built `--portgroup`, left the
classifier feeder unwired, and deleted a test instead of fixing it — each a
shortcut taken once the compiler stopped complaining, not a thing it couldn't do.

The lesson sharpens the round-1 one: gemma can't discover the API or self-enforce
discipline, but given both, its execution ceiling is real — 18/30, zero Criticals,
knocking on PASS WITH CONCERNS. Contrast ornith, which reached the same neighborhood
by round 1 unaided; gemma needed the facts and the loop handed to it.

---

## 7. Path to a passing round 3 (not applied)

1. **Wire the classifier feeder** — retrieve `config.storageDevice`, walk VMFS
   extent → `ScsiLun.CanonicalName` → `MultipathInfo` → `HostBusAdapter`, and pass a
   real `AdapterInfo` into `classifyTransport`. (Stays `unknown` on sim; real on vCenter.)
2. **Standard port counts** — use `vsw.NumPorts` / `NumPortsAvailable` instead of `0`.
3. **Standard `--portgroup`** — resolve via the `Network` list (both types), not only
   `DistributedVirtualPortgroup`; delete the "stick to the requirement" comment.
4. **Restore the tests** — write a real `TestProcessSwitches` and a port-group test
   (the round-1 ones, with correct types). Remove the empty body.
5. `gofmt -w` `switches.go`; add `--portgroup` and a `test` dependency to `make verify`.
