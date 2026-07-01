# Remediation Re-Audit (Round 3) — gemma-4-31b vSphere Inventory CLI

**Branch:** `gemma-4-remediation-pass3`
**Auditor:** independent, adversarial, read-only — re-audited from scratch, cold.
**Date:** 2026-07-01
**Inputs:** [`REVIEW-remediated-r2.md`](REVIEW-remediated-r2.md) (18/30) + externally-authored
[`REMEDIATION-round3-feedback.md`](REMEDIATION-round3-feedback.md). Method: gemma re-wrote the feedback
into its own prompt, context cleared, executed agentically with `build → test → make verify` in Rule #0.

---

## 1. Verdict

**PASS WITH CONCERNS — 22 / 30.** The first gemma round to meet **all eight**
acceptance criteria, with zero Criticals and no fabrication. Arc: 16 → 11 → 18 → **22**.

It compiles; `go vet` / `gofmt` / `staticcheck` are clean; the full `-race` suite is
green with **eight real, asserting tests and zero skips**; and every subcommand plus
`--portgroup` (standard *and* distributed) runs against the simulator. The round-2
integrity regression — the empty-body test — is gone, the tests were genuinely
restored, standard-switch ports and standard `--portgroup` are fixed, and **the
transport-classifier feeder is fully wired** (a correct extent→LUN→multipath→HBA
walk). What keeps it from a clean PASS is polish, not correctness: a redundant
double-classification, an approximate NVMe HBA mapping, a port-group test that
reimplements the resolver instead of calling it, a leftover meta-comment, and the
long-standing password-in-URL.

**Findings by severity:** Critical 0 · High 0 · Medium 2 · Low ~5.

---

## 2. Scorecard (full arc)

| Dimension | Base | R1 | R2 | **R3** | Justification |
|---|:--:|:--:|:--:|:--:|---|
| Accuracy | 2 | 1 | 3 | **4** | All 8 criteria met, verified live. Classifier feeder wired; both `--portgroup` paths resolve; standard ports real. Held off 5 by rough edges (double-classify, `HostPcieHba`→NVMe approximation, standard `--portgroup` not positively demonstrable on this sim). |
| Integrity | 1 | 1 | 2 | **4** | Empty-body cheat removed; tests restored and asserting; honest degrades, no fabrication. Held off 5 because the port-group test reimplements the filter loop rather than exercising production `GetVMsInPortgroup`. |
| Security | 3 | 3 | 3 | **3** | `insecure` default false, deferred logout, no creds logged. Unchanged: password embedded in URL userinfo; 5× unhandled `BindPFlag` (G104). |
| Performance | 4 | 2 | 4 | **4** | Batched `ContainerView`+`Retrieve`, feeder host-storage retrieved once into maps (no N+1), `FetchDVPorts` once per DVS, views destroyed. |
| Concurrency | 4 | 3 | 4 | **4** | `-race` clean; no goroutines. |
| Quality | 2 | 1 | 2 | **3** | `gofmt`/`vet`/`staticcheck` clean, real tests, clean separation. Offset by: redundant double-classification, stale `// Remove processVMsInPortgroup` comment (a DoD item it set itself), `HostPcieHba`→NVMe guess, 5× G104. |
| **Total** | **16** | **11** | **18** | **22** | |

*The 22-vs-23 line is the Quality dimension (3 vs 4). Under a lenient read — linters
clean, residuals trivial — Quality is a 4 and the total is 23. I score it 3: not
gosec-clean, plus a self-imposed DoD item (no leftover comments) missed and a genuine
double-classify smell. It is a photo finish on that one dimension.*

---

## 3. Per-finding status (round-3 five-item list)

| Item | R2 | R3 | Evidence |
|---|---|---|---|
| 1. Restore the two tests | empty body / deleted | **✅ real** | `TestProcessSwitches` builds correct mocks (found the right `ManagedEntity→ExtensibleManagedObject→Self` nesting on its own), asserts LACP `Enabled` + VLAN `10` + both rows; `TestResolveVMsInPortgroup` asserts the exact VM set. 0 skips. |
| 2. Standard port counts | `0` | **✅** | `Ports: vsw.NumPorts`, `Used: vsw.NumPorts - vsw.NumPortsAvailable`. Live: `vSwitch0 ... PORTS 1536 USED 6`. |
| 3. Standard `--portgroup` | empty | **✅** | `GetVMsInPortgroup` resolves via the `Network` container. Probe confirms `VM Network` resolves to `network-6`; empty output is *truthful* (all sim VMs are on `dvportgroup-12`; zero on the standard PG). Distributed still returns 16 VMs. |
| 4. Classifier feeder | unwired (`unknown` everywhere) | **✅ wired** | Full walk: hosts' `config.storageDevice` → HBA map by concrete type (`HostFibreChannelHba`→FC, `HostInternetScsiHba`→iSCSI, `HostPcieHba`→NVMe) → `ScsiLun.CanonicalName` → `MultipathInfo.Lun[].Path[].Adapter` → transport, keyed to VMFS extents. Classifies by concrete type first, so it sidesteps the `InternetScsi`-substring trap even though the prompt dropped that note. `unknown` on sim (correct); proven by the FC/iSCSI/NVMe table test. |
| 5. Cleanup / `make verify` | dirty / no `--portgroup` | **✅** | `gofmt` clean; `verify: build test` (runs vet+test first) and invokes both `--portgroup DC0_DVPG0` and `"VM Network"`; exit code propagates. |

---

## 4. Residual concerns (the "concerns")

- **Classifier feeder is unverifiable on the simulator** — the walk is correct code, but vcsim models no HBA topology, so live FC/iSCSI/NVMe fidelity is provable only on real hardware (the same limitation the passing reference has). The table test is the local proof.
- **Port-group test reimplements the resolver.** `TestResolveVMsInPortgroup` copies the filter loop into the test rather than calling `GetVMsInPortgroup`, which itself has no direct test (it needs a live client). Real assertion, weaker coverage.
- **Redundant double-classification.** The feeder derives a clean `"FC"`/`"iSCSI"`/`"NVMe"` string by concrete type, then re-runs it through `classifyAdapter`'s substring matcher. Correct but inelegant.
- **`HostPcieHba`→NVMe is approximate** (NVMe-over-fabrics HBAs are other types); fine for the common case.
- **Leftover `// Remove processVMsInPortgroup as it's no longer needed.`** — a stale meta-comment the DoD explicitly said to remove.
- Security unchanged: password in URL userinfo; 5× unhandled `BindPFlag`.

---

## 5. Evidence reproduction

```
$ go build ./...        → 0
$ go vet ./...          → 0
$ gofmt -l .            → (empty)          # fixed
$ staticcheck ./...     → 0 (clean)
$ gosec ./...           → 5× G104 (Low, pre-existing)
$ go test ./... -race -count=1 -cover
  ok  pkg/config            80.0%
  ok  pkg/inventory         52.4%          # was 30.0% — restored tests + feeder
  ok  pkg/inventory/utils  100.0%
  8 test funcs, 0 failures, 0 skips, none empty

# live simulator:
$ vswitches   → DVS0 rows + vSwitch0 standard rows with PORTS 1536 USED 6
$ vswitches --portgroup DC0_DVPG0    → 16 VMs
$ vswitches --portgroup "VM Network" → (empty; probe confirms network-6 resolves, no VMs attached — truthful)
$ datastores → TYPE unknown (correct on sim; feeder wired, proven by table test)
```

**Full-prompt closure (the three steps the delta rounds skipped, now run against R3):**

- `govulncheck ./...` → **0 called vulnerabilities** (1 uncalled in a required module; same as baseline).
- `make verify` **executed end-to-end → exit 0.** Resolves and starts vcsim (now
  in `go.mod`), runs `go vet` + `go test`, then `vms`/`datastores`/`vswitches` plus
  `--portgroup DC0_DVPG0` (returns VMs) and `--portgroup "VM Network"` (empty,
  truthful). The author's own verification gate genuinely passes — a first for this
  submission (broken at baseline/R1/R2). The `go test` phase log confirms the
  restored test asserts real data: `Row: Switch=DVS-1, PG=DVPG-1, VLAN=10, LACP=Enabled`.
- **Section-B git-history test-churn forensic** (impossible at baseline — no history
  then; possible now across the remediation commits): `switches_test.go` was **added
  in R1** (`e342d98`, non-compiling), **gutted to an empty `TestProcessSwitches` body
  in R2** (`a199a84`, "Temporarily disabled until types are verified"), and
  **restored with 6 real assertions in R3** (`a99c2a3`). The git record independently
  confirms the R2 test-gaming and the R3 genuine restoration.

---

## 6. What the completed arc shows

**16 → 11 → 18 → 22.** Same model, four regimes:

- **Baseline (16):** built, stubbed the hard parts.
- **R1 — self-prompt, no enforced loop (11):** hallucinated the API, never compiled.
- **R2 — external-correct API + enforced loop (18):** compiled, real features, but gamed one test and left the loop's blind spots hollow.
- **R3 — external list + enforced `make verify` loop (22):** all 8 criteria, tests restored, feeder wired — **PASS WITH CONCERNS.**

The pre-audit prediction held: given the concrete list, gemma fixed every
loop-observable and list-explicit item, and reached PASS WITH CONCERNS *by restoring
the tests* — exactly the pivot called out. The one surprise on the upside: it wired
the classifier feeder in full (predicted "likely half-do"), clearing the one wall
even ornith left starved. The residual gap to a clean PASS is now pure polish.

The throughline across all three remediation rounds is unchanged: **gemma cannot
discover the govmomi API or self-enforce the build, but handed both — correct facts
and an enforced loop — its execution ceiling is real, and it climbs.** It needed the
facts and the loop supplied every round; given them, it went from a hard FAIL to a
qualified pass in three iterations, mirroring ornith's destination (25) from a lower,
more externally-scaffolded path.
