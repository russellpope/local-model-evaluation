# Remediation Re-Audit (Round 3) — vSphere Inventory CLI (govmomi)

**Submission:** `orinth-1.0-35b-fp16/govmomi-cli` — **remediated, round 3**
(branch `remediate-round3`, working tree atop the round-2 checkpoint `3bb9aa7`).
**Auditor:** Claude Opus 4.8 (independent re-audit, read-only)
**Date:** 2026-06-28
**Baseline chain:** [`REVIEW.md`](REVIEW.md) (FAIL, 16/30) →
[`REVIEW-remediated.md`](REVIEW-remediated.md) (FAIL, 20/30) →
[`REVIEW-remediated-r2.md`](REVIEW-remediated-r2.md) (FAIL, 22/30) → **this report**.
**Method:** treated the round-3 tree as a fresh untrusted submission. Re-ran the
full reproduce-everything pass (gofmt / build / vet / `-race` / staticcheck /
gosec / govulncheck) with a `-v` skip audit, and drove every subcommand against a
live embedded `simulator` server with flags. Scored against the **original
`REVIEW.md` findings + the eval spec**, not the model's self-authored prompt.

> Git note: round-2 is committed as checkpoint `3bb9aa7`, so this re-score has an
> exact **round-2 → round-3 delta** (`git diff 3bb9aa7`). Round 3 touched 5 files
> (`cmd/vswitches.go`, `internal/inventory/{datastores,portgroup_test,switches,vms}.go`),
> no new files, no dependency changes.

---

## 1. Verdict

### `PASS WITH CONCERNS` — score **16 → 20 → 22 → 25 / 30** (+3)

Round 3 clears the only verdict-blocker. The firing `t.Skip` is gone, **`go test
./...` now reports zero failures and zero skips** (reproduced with `-v`), and all
eight acceptance criteria are met and independently verified — including
criterion 2 (flag precedence, live), criterion 5 (used = total − available;
standard real, DVS honest `N/A`), and criterion 8 (zero skips). No Critical or
High findings remain, and **nothing previously fixed regressed** (C1/C2/C3/N1/N2/
H1/H2/H3 all re-verified, live where observable).

All five round-2 residuals were fixed, each genuinely (not gamed):

- **The `t.Skip` → two real tests.** `TestListVMsByPortGroup_DistributedPG_ExactSet`
  is a *true bidirectional* exact-set assertion (no-extras + no-missing + no-dupes
  + vCPU/RAM identity-consistency, with the expected set derived from `ListVMs` at
  runtime — no hardcoded count). `TestListVMsByPortGroup_StandardPG_Empty` asserts
  exactly 0 (falsifiable against over-matching). Neither skips.
- **N1-residual** — the three parallel-index loops now iterate `range allVms` /
  `range allDevices`; `listDVSPortGroupNames` keys by `pgs[i].Self.Value`.
- **H3-residual** — `buildHostStorageCache` batches every host's storage device in
  one `pc.Retrieve`; the per-datastore re-walk is gone (network O(ds×hosts) →
  O(hosts)).
- **UPLINKS** renders the bare NIC name (`vmnic0`); the dead `-1` sentinel is gone.

It is **PASS WITH CONCERNS**, not a clean sweep, because a handful of
sub-Critical residuals remain (one N+1, a latent nil-deref, minor dead code, and
the standard-PG positive path that vcsim cannot exercise) — see §4. None blocks an
acceptance criterion; all are fixable in a short polish pass.

**Findings by severity (round 3):** Critical **0** · High **0** · Medium **1** ·
Low **4**. (Round 2 was Critical 1 · High 0 · Medium 2 · Low 2.)

---

## 2. Scorecard — original → r1 → r2 → r3

| Dimension | Orig | R1 | R2 | **R3** | Why it moved (r2 → r3) |
|-----------|:---:|:---:|:---:|:------:|------------------------|
| Accuracy    | 2 | 3 | 4 | **5** | Criterion 8 (zero skips) now met; all 8 criteria pass and are verified live + by unit tests. Sim-unmodeled fields (transport, standard-PG positive) degrade honestly — the same limitation the passing reference submission had. |
| Integrity   | 2 | 3 | 3 | **4** | The firing `t.Skip` is gone; the new port-group test is a genuine, falsifiable, bidirectional exact-set assertion. No masking, no vacuous assertions, no gaming anywhere. Held at 4 (not 5) because the suite still cannot *positively* prove standard-PG matching (vcsim attaches no VMs to standard PGs) — a residual coverage gap, not a cheat. |
| Security    | 4 | 4 | 4 | **4** | Unchanged-good: insecure default false, no credential leak, deferred logout, gosec / govulncheck clean. |
| Performance | 2 | 3 | 3 | **4** | The datastore HBA walk is now batched (H3 fixed). One residual N+1 remains: `listStandardSwitches` still `RetrieveOne`s per host — bounded by host count, mild, but the same pattern just fixed elsewhere. |
| Concurrency | 4 | 4 | 4 | **4** | `-race` clean; context threaded throughout; views destroyed; logout deferred; no `context.TODO()`. |
| Quality     | 2 | 3 | 4 | **4** | gofmt / vet / staticcheck / gosec clean; UPLINKS cleaned, dead sentinel removed. Offset by minor new residuals (dead `ctx`/`c` params in `hostHBAsForDatastore`; a now-vacuous `i < len(pgs)` guard; a latent `hs.Config` nil-deref). |
| **Total**   | **16** | **20** | **22** | **25** | **+3** |

---

## 3. Round-2 residuals — did round 3 fix them?

| ID | Round-2 status | **Round-3 status** | Evidence |
|----|----------------|--------------------|----------|
| **Crit — firing `t.Skip`** (criterion 8) | UNMET (deterministic skip) | **FIXED** | `portgroup_test.go` rewritten: `DistributedPG_ExactSet` (bidirectional set equality + identity-consistency, expected derived from `ListVMs`, `model.Machine=8`) + `StandardPG_Empty` (asserts exactly 0). `go test ./... -race -v` → **zero skips, zero failures**; `grep -rn t.Skip` → none. |
| **N1-residual** — parallel-index panic | open (Medium) | **FIXED** | `vms.go`: step 2 `for i := range vmRefs` → `range allDevices`; final join → `range allVms`; `listDVSPortGroupNames` → `out[pgs[i].Self.Value]`. No loop indexes a parallel slice by a foreign bound. |
| **H3-residual** — O(ds×hosts) HBA walk | open (Medium) | **FIXED** | `datastores.go`: new `buildHostStorageCache` does one batched `pc.Retrieve(hostRefs, ["config.storageDevice","datastore","name"])`; `hostHBAsForDatastore` filters the in-memory cache. Network round-trips O(ds×hosts) → O(hosts). Bonus: now fetches the *whole* `config.storageDevice` (round 2 under-fetched `.hostBusAdapter` yet read `ScsiTopology`/`ScsiLun`). |
| **Low — UPLINKS raw key** | open | **FIXED** | `switches.go`: `stripPnicKey` trims `key-vim.host.PhysicalNic-`. Live: UPLINKS = `vmnic0`. |
| **Low — dead `-1` sentinel** | open | **FIXED** | `vswitches.go`: passes `s.UsedPorts` directly to `formatUsedPorts`. |

---

## 4. Residual findings (none block a criterion)

- **N+1 (Medium, perf) — per-host `RetrieveOne` in `listStandardSwitches`.**
  `switches.go:72` still loops over hosts calling `pc.RetrieveOne(ctx, ref,
  ["config.network","name"])` once per host — the exact RetrieveOne-in-a-loop the
  rubric flags, and the one `buildHostStorageCache` just eliminated for datastores.
  Bounded by host count (not VM count), so milder than the original N+1, but a
  single `pc.Retrieve(hostRefs, …, &hosts)` would remove it. Was present in round 2
  as well; round 3 did not address it (it was outside the prompt's item list).
- **Low (latent panic) — `hs.Config` nil-deref.** `buildHostStorageCache`
  (`datastores.go:162`) reads `hs.Config.StorageDevice` without an `hs.Config ==
  nil` guard. A disconnected/not-responding host on a real vCenter can return a
  nil `Config`, panicking. Sim-masked (vcsim hosts always have config); carried in
  spirit from round 2's `hs.Config.StorageDevice == nil` check.
- **Low (dead code) — unused params.** After the cache refactor,
  `hostHBAsForDatastore` no longer uses its `ctx` / `c` parameters; they should be
  dropped (or the function simplified). `listDVSPortGroupNames`'s `i < len(pgs)`
  guard (`vms.go:307`) is now vacuous (the loop is `for i := range pgs`).
- **Low (coverage) — standard-PG positive path unproven.** `StandardPG_Empty`
  asserts the correct (empty) result and is falsifiable against over-matching, but
  vcsim attaches no VMs to standard port groups, so the *positive* standard-PG
  match still has no test. This is a simulator limitation (the passing reference
  submission had the same gap), not a code defect.

---

## 5. Regression check — every prior fix held

`-race` clean, all green, verified live where observable. Round 3 touched only
`vswitches.go`, `datastores.go`, `portgroup_test.go`, `switches.go`, `vms.go`;
`root.go` and the command RunEs are untouched, so the flag/timeout wiring is the
round-2 state.

| ID | Fix | R3 | Evidence |
|----|-----|:--:|----------|
| C1 | `--url` overrides env | ✅ | `root.go` untouched; `TestBindFlags_FlagOverridesEnv` PASS; live `VSPHERE_URL=http://wrong:1/sdk … --url <good>` → 16 VMs from `<good>`. |
| C2 | std USED real, DVS USED `N/A` | ✅ | Live: `vSwitch0` `6/1536`; `DVS0` `USED=N/A`. |
| C3 | standard switches list | ✅ | Live: `vSwitch0 / Management Network` + `VM Network` rows present. |
| N1 | keyed by `.Self.Value` | ✅ | Live `--portgroup DC0_DVPG0` → 16 VMs; `DistributedPG_ExactSet` PASS. |
| N2 | HOST column | ✅ | Live: distinct hosts (`DC0_H0`, `DC0_C0_H0/H1/H2`). |
| H1 | real transport feeder | ✅ | `classifyTransport` path intact; `TestClassifyTransport` PASS; datastores degrade to `unknown` on vcsim (legit). The cache refactor preserved (improved) the feeder. |
| H2 | timeout threaded | ✅ | Command RunEs unchanged; no `context.TODO()`. |
| H3 | batched retrieval | ✅ | Now also batches the host storage walk (see §3). |
| M2/M3/L1–L4 | linters / deliverables / formatting | ✅ | gofmt/vet/staticcheck/gosec clean; Makefile + README present; `RAM (GiB)` header. |

---

## 6. Security

Unchanged and clean. `insecure` defaults false; password via `url.UserPassword` →
`Login` (never in a URL or log); logout deferred; no shell surface. **gosec → no
findings. govulncheck → 0 in called code** (1 in a required-but-uncalled module,
unreachable — identical to every prior round).

---

## 7. Evidence reproduction (round-3 tree)

```
$ gofmt -l .            # empty
$ go build ./...        # exit 0
$ go vet ./...          # exit 0
$ staticcheck ./...     # no findings
$ gosec -quiet ./...    # no findings
$ govulncheck ./...     # 0 in called code; 1 in an uncalled module
$ grep -rn 't\.Skip' --include='*.go' .   # none
$ go test ./... -race -count=1 -v
ok  govmomi-cli/cmd                 (TestBindFlags_FlagOverridesEnv PASS)
ok  govmomi-cli/internal/config     (3 pass)
ok  govmomi-cli/internal/inventory  (10 pass — incl. DistributedPG_ExactSet, StandardPG_Empty)
# zero failures, ZERO SKIPS
```

Live run against the embedded `simulator` (VPX; 8 vm / 3 ds / 3 pg), with flags:

```
# vswitches — HOST col, std USED<TOTAL, DVS USED=N/A, UPLINKS=vmnic0 (item 4):
SWITCH    HOST       SWITCH TYPE  PORTGROUP           VLAN    UPLINKS  LACP      TOTAL PORTS  USED
DVS0      N/A        distributed  DC0_DVPG0                   N/A      disabled  1            N/A
vSwitch0  DC0_H0     standard     Management Network          vmnic0   N/A       1536         6
vSwitch0  DC0_C0_H0  standard     VM Network                  vmnic0   N/A       1536         6
... (per-host rows, distinct HOST)

# C1 — env=wrong:1 + --url good → succeeds against good (flag beats env):
$ VSPHERE_URL=http://wrong:1/sdk gcli vms --url http://127.0.0.1:<port>/sdk … → 16 VMs
# --portgroup DC0_DVPG0 → 16 VMs ; datastores → USED/AVAILABLE correct, TYPE=unknown
```

---

## 8. Confidence & limitations

- **High confidence** on the verdict change: zero-skips reproduced with `-v`; the
  new exact-set test is bidirectional and falsifiable; all linters/scanners and the
  live sim run reproduced; no regression in C1/C2/C3/N1/N2/H1/H2/H3.
- **Standard-PG positive `--portgroup`** remains unverified-positive on vcsim (no
  VMs on standard PGs) — same limitation across all four audits and the passing
  reference submission.
- **H1 live fidelity** (real FC/iSCSI/NVMe) still needs a live vCenter; on vcsim it
  degrades to `unknown` and the production path is real code (now fed the full
  `config.storageDevice`).
- The §4 residuals (per-host N+1, `hs.Config` nil-deref) are code-read; the nil-deref
  is not reproducible on vcsim (hosts always have config).

---

## 9. Prioritized remediation (round 4 — optional polish, not blocking)

**Medium**
1. Batch `listStandardSwitches`: replace the per-host `RetrieveOne` loop with one
   `pc.Retrieve(hostRefs, ["config.network","name"], &hosts)` (mirror
   `buildHostStorageCache`).

**Low**
2. Guard `hs.Config == nil` in `buildHostStorageCache` before reading
   `.StorageDevice`.
3. Drop the now-unused `ctx`/`c` params from `hostHBAsForDatastore`; remove the
   vacuous `i < len(pgs)` guard in `listDVSPortGroupNames`.
4. If a live vCenter or a custom sim model becomes available, add a positive
   standard-PG `--portgroup` test (attach a VM to a standard PG, assert the exact set).
