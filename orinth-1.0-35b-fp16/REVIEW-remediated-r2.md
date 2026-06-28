# Remediation Re-Audit (Round 2) — vSphere Inventory CLI (govmomi)

**Submission:** `orinth-1.0-35b-fp16/govmomi-cli` — **remediated, round 2**
(branch `remediate-round2`, working tree atop the round-1 checkpoint `166e1fa`).
**Auditor:** Claude Opus 4.8 (independent re-audit, read-only)
**Date:** 2026-06-28
**Baseline chain:** [`REVIEW.md`](REVIEW.md) (FAIL, 16/30) →
[`REVIEW-remediated.md`](REVIEW-remediated.md) (FAIL, 20/30) → **this report**.
**Method:** treated the round-2 tree as a fresh untrusted submission. Re-ran the
full reproduce-everything pass (gofmt / build / vet / `-race` / staticcheck /
gosec / govulncheck) and drove every subcommand against a live embedded
`simulator` server **with flags**, including the decisive C1 env-vs-flag probe.
Scored against the **original `REVIEW.md` findings + the eval spec**, not the
model's self-authored remediation prompt.

> Git note: round-1 is committed as checkpoint `166e1fa`, so unlike round 1 this
> re-score has an exact **round-1 → round-2 delta** (`git diff 166e1fa`). Round 2
> touched 7 files (`cmd/root.go`, `cmd/vswitches.go`, `internal/inventory/{datastores,switches,vms,switches_test,portgroup_test}.go`)
> and added one new file (`cmd/root_test.go`). Everything else is byte-identical
> to the 20/30 tree, which bounds the regression surface.

---

## 1. Verdict

### `FAIL` (improved) — score **16 → 20 → 22 / 30** (+2)

Round 2 is a genuine, well-targeted fix pass. It resolves **all four** remaining
round-1 blockers, each verified live against a running simulator:

- **C1 (flags) — FIXED.** `--url` now overrides `VSPHERE_URL`. Live probe:
  `VSPHERE_URL=http://wrong:1/sdk gcli vms --url <good-sim>` **succeeded against
  the sim and printed 16 VMs** (round 1 connected to `wrong:1` and failed). The
  one-line fix is correct: `bindFlags` now looks up flags on `cmd.Flags()` (the
  merged set incl. inherited persistent flags) instead of the subcommand's empty
  `cmd.PersistentFlags()`. And the new test (`cmd/root_test.go`) drives the
  **real** `rootCmd.Execute()` with `--url` vs a conflicting env var — the exact
  end-to-end coverage round 1 lacked.
- **C2-distributed (fabricated USED) — FIXED** via honest degrade. A
  `UsedPortsValid bool` was added; DVS rows set it `false` and the presenter
  renders **`N/A`** rather than `Total − 0`. Live `DVS0` rows now show `USED=N/A`;
  standard `vSwitch0` still shows a real `USED=6 / TOTAL=1536`.
- **N1 (positional VM↔NIC mis-association) — FIXED.** All four batch-result
  keying sites flipped from input-ref position (`vmRefs[i].Value`) to each
  **returned object's `.Self.Value`**. Live `--portgroup DC0_DVPG0` returns the
  correct 16-VM set.
- **N2 (duplicate standard-switch rows) — FIXED** via a **HOST column**. Live
  output now disambiguates the four per-host `vSwitch0` rows by host
  (`DC0_H0`, `DC0_C0_H0/H1/H2`).

The linters are clean (gofmt / vet / staticcheck / gosec all green; govulncheck
0-in-called-code), security is unchanged-good, and **no previously-fixed finding
regressed** (C3/H1/H2/H3/M2/M3/L1–L4 all re-verified intact).

It nevertheless remains a **FAIL** on a single, verifiable hard-criterion
violation: a **newly-introduced `t.Skip` fires on every run**, so `go test ./...`
reports a `SKIP` — breaking **acceptance criterion 8** ("zero failures and zero
skips") and the spec's explicit **"No `t.Skip`"** test-integrity rule. This is an
*honest* skip — it does **not** mask a broken feature (the port-group lookup is
otherwise tested and passing) — but the eval's verdict rule maps an unmet hard
requirement to FAIL regardless of intent, exactly as the original audit applied
it. It is a finish-line miss: construct a VM-on-a-standard-PG scenario (or assert
the distributed exact-set without skipping) and this flips toward PASS.

**Findings by severity (round 2):** Critical **1** · High **0** · Medium **2** ·
Low **2**. (Round 1 was Critical 1 · High 2 · Medium 2 · Low ~3; original was
Critical 3 · High 3 · Medium 4 · Low 5.)

---

## 2. Scorecard — original → r1 → r2

| Dimension | Orig | R1 | **R2** | Why it moved (r1 → r2) |
|-----------|:---:|:---:|:------:|------------------------|
| Accuracy    | 2 | 3 | **4** | Criteria **2** (flag precedence) and **5** (`used = total − available`: standard real, DVS honest `N/A`) now **met and live-verified**. Held back only by criterion 8 (firing skip) and the standard-PG positive path being unverifiable on vcsim. |
| Integrity   | 2 | 3 | **3** | The two round-1 green-masks-broken tests are **gone** — the C1 flag path is now driven end-to-end through `rootCmd.Execute()` (passes), and DVS used is asserted as `N/A`-not-derivable. The N1 latent bug is fixed. **But** a new firing `t.Skip` (the spec's named anti-pattern) was introduced, and the standard-PG exact set is *skipped* rather than asserted — holding integrity flat. |
| Security    | 4 | 4 | **4** | Unchanged. The round-1 footgun ("silently-ignored `--url` could send creds to the wrong endpoint") is actually **resolved** now that flags work. gosec / govulncheck clean. |
| Performance | 2 | 3 | **3** | N1 corrected with no perf cost, but the round-1 perf residuals are unchanged: HBA enrichment re-walks **all** hosts **per datastore** (O(ds×hosts)); standard-switch listing is per-host `RetrieveOne`. |
| Concurrency | 4 | 4 | **4** | `-race` clean. The **last** `context.TODO()` (datastores HBA path) is gone, so the timeout context now threads through every retrieval path. |
| Quality     | 2 | 3 | **4** | gofmt / vet / staticcheck clean. The three things that held quality at 3 in round 1 — duplicate per-host rows, the N1 ordering bug, the residual `context.TODO()` — are **all resolved**. Only minor new residuals remain (latent parallel-index assumption in `vms.go`; raw Pnic key in UPLINKS; a dead `-1` sentinel in the presenter). |
| **Total**   | **16** | **20** | **22** | **+2** |

---

## 3. Round-1 survivors — did round 2 fix them?

| ID | Round-1 status | **Round-2 status** | Evidence |
|----|----------------|--------------------|----------|
| **C1** — flag values silently dropped | STILL UNMET (silent) | **FIXED** | `bindFlags` (`root.go:78-84`): `cmd.PersistentFlags().Lookup` → `cmd.Flags().Lookup`. Live: `VSPHERE_URL=http://wrong:1/sdk gcli vms --url <good>` → **prints 16 VMs from `<good>`** (flag beats env). `--help` lists all 6 flags. New `cmd/root_test.go` drives the real `rootCmd.Execute()` and **passes**. |
| **C2-distributed** — DVS `USED == TOTAL` | PARTIAL (std fixed, DVS not) | **FIXED** (honest degrade) | `switches.go`: DVS block drops the `Total − 0` computation, sets `UsedPortsValid: false`; presenter (`vswitches.go:96`) renders `N/A`. Live `DVS0` rows: `USED=N/A`. Standard `vSwitch0`: `USED=6 / TOTAL=1536`. |
| **N1** — positional batch association | NEW (High, latent) | **FIXED** | `vms.go`: `vmInfoByRef[vmRefs[i].Value]` → `[allVms[i].Self.Value]`; `nicMap` keyed by `allDevices[i].Self.Value`; `netNameByValue` keyed by `nets[i].Self.Value`; final join via `selfValue`. (Residual length-assumption — see §6.) Live `--portgroup DC0_DVPG0` → correct 16-VM set. |
| **N2** — duplicate standard rows, no host | NEW (Medium) | **FIXED** | `SwitchInfo.Host` added; `listStandardSwitches` sets `Host: hs.Name`; `vswitches.go` adds a **HOST** column. Live: four per-host `vSwitch0` rows now carry distinct hosts (`DC0_H0`, `DC0_C0_H0/H1/H2`). |
| **M1-residual** — port-group test should assert the exact set + cover a standard PG | open | **PARTIAL → introduced a regression** | The new `TestListVMsByPortGroup_StandardPG_ExactSet` adds real identity-consistency assertions (returned VMs exist in inventory; vCPU/RAM match) — good — **but** when no standard PG has VMs (always true on vcsim) it falls back to `t.Skip` (`portgroup_test.go:124`). See §4. |

---

## 4. The remaining blocker (Critical) — a firing `t.Skip` breaks criterion 8

`internal/inventory/portgroup_test.go:124`, inside the new
`TestListVMsByPortGroup_StandardPG_ExactSet`:

```go
if stdPG == "" {
    t.Skip("no standard port group with connected VMs in simulator; skipping exact-set test")
}
```

The test scans for a **standard** port group that has VMs attached. On the VPX
simulator, VMs are attached only to the **distributed** `DC0_DVPG0`; the standard
`VM Network` / `Management Network` have none (confirmed in the original audit and
again here). So `stdPG` is **always** `""` and the skip **always** fires:

```
--- SKIP: TestListVMsByPortGroup_StandardPG_ExactSet (1.07s)
```

Why this is the verdict-blocker:

- The spec states it twice — acceptance **criterion 8**: *"`go test ./...` passes
  with zero failures and **zero skips**"*; and the **test-integrity rules**:
  *"**No `t.Skip`**, no empty or tautological tests… `go test ./...` must pass with
  zero failures and zero skips."* A firing skip violates **both**.
- The spec's graceful-degrade allowance is for **field values** (`unknown`/`N/A`
  for sim-unmodeled transport/LACP), **not** for skipping a test. The spec
  explicitly tells the author to *configure the model so known VMs are attached to
  a known port group* — i.e. construct the scenario, don't skip it.
- This is a **regression**: round 1 had removed the submission's only `t.Skip` and
  achieved a clean zero-skip suite; round 2 re-introduced one.

**Important nuance (fairness):** unlike the original audit's findings, this is
*not* a disguised cheat. The skip is transparent, the feature it would exercise is
covered elsewhere (`TestListVMsByPortGroup_Simulator` passes against a real
distributed PG with 16 VMs), and the new test does real identity-consistency work
when a qualifying PG exists. The verdict is FAIL because criterion 8 is a hard
requirement that is *verifiably and deterministically* unmet — the same
intent-independent rule the original audit invoked — not because the model gamed
anything.

---

## 5. Regression check — every round-1 fix held

Treated as fresh; re-verified, not assumed. (Most are in files round 2 never
touched — `cmd/vms.go`, `cmd/datastores.go`, `bytefmt.go`, `transport.go`,
`config.go`, `main.go`, `Makefile`, `README.md` are byte-identical to the 20/30
tree.)

| ID | Round-1 fix | R2 | Evidence |
|----|-------------|:--:|----------|
| C3 | Standard switches via `config.network`, error wrapped not `continue`'d | ✅ | `switches.go:74` reads `["config.network","name"]` off `HostSystem`, returns wrapped error. Live: standard `vSwitch0` rows present. |
| H1 | Real HBA discovery (no `return nil,nil` stub) | ✅ | `datastores.go:112-180` reads `config.storageDevice.hostBusAdapter`, builds LUN→HBA map via `scsiTopology`, classifies FC/iSCSI/NVMe. Degrades to `unknown` on vcsim (legit). |
| H2 | Timeout threaded through retrieval; no early `cancel()` | ✅ | Each RunE builds `context.WithTimeout(cmd.Context(), cfg.Timeout)` and passes it to the inventory calls; `cmd/vms.go`/`cmd/datastores.go` unchanged from r1; `vswitches.go:31-33` intact. **Last** `context.TODO()` removed (`datastores.go:83` now passes `ctx`). |
| H3 | N+1 → batched `pc.Retrieve` | ✅ (residual) | VMs, port-group, datastores, DVS all batch. Residual O(ds×hosts) HBA walk + per-host standard switch listing unchanged (see §6). |
| M2 | Dead code (SA4006) | ✅ | `staticcheck ./...` clean. |
| M3 | Makefile + README deliverables | ✅ | both present and unchanged. |
| L1–L4 | gofmt, `FormatBytes` branch, `FormatGB`, `RAM (GiB)` header | ✅ | `gofmt -l .` empty; live output header is `RAM (GiB)`. |

---

## 6. New / residual findings (none verdict-blocking)

- **N1-residual (Medium, latent) — parallel-index length assumption.** After
  fixing the *keys*, the three loops in `ListVMsByPortGroup` still iterate
  `for i := range vmRefs` while indexing parallel slices `allVms[i]` /
  `allDevices[i]` (`vms.go:154-156, 185-186`). `property.Collector.Retrieve` is
  not guaranteed to return one object per requested ref (a VM deleted or
  permission-filtered between `Find` and `Retrieve` yields a shorter slice) → an
  index-out-of-range **panic** on a real vCenter. This is *milder* than the
  original mis-association bug (and sim-masked), but it should iterate
  `range allVms` / `range allDevices`, or guard the length.
- **H3-residual (Medium, perf) — O(ds×hosts) HBA enrichment.**
  `hostHBAsForDatastore` rebuilds a host `ContainerView` and `RetrieveOne`s every
  host **for each non-NFS datastore** (`datastores.go:112-180`); `listStandardSwitches`
  is per-host `RetrieveOne` (`switches.go:72-76`). Fine at sim scale, quadratic on
  a real fleet. Unchanged from round 1.
- **Low — UPLINKS shows the raw MO key.** `formatUplinks` returns `vs.Pnic`
  verbatim, so UPLINKS renders `key-vim.host.PhysicalNic-vmnic0` rather than a
  clean `vmnic0`. (Not a regression — `formatUplinks` is unchanged from round 1;
  the round-1 report simply abbreviated it.)
- **Low — dead sentinel in the presenter.** `runSwitchesMode`
  (`vswitches.go:67-72`) computes `used = -1` when `!UsedPortsValid`, but
  `formatUsedPorts` ignores `used` entirely when `!valid`. The `-1` branch is dead
  and mildly misleading.

---

## 7. Security

Unchanged and still the strongest dimension. `insecure` defaults `false`;
password flows via `url.UserPassword` → `session.Manager.Login` (never in a URL or
log); logout deferred on all paths; no shell surface. **gosec `./...` → no
findings. govulncheck → 0 vulnerabilities in called code** (1 in a
required-but-uncalled module, unreachable — identical to the original audit). The
round-1 *mild* footgun (a silently-ignored `--url`/`--password` could target the
wrong endpoint) is **resolved**, since flags now take effect as typed.

---

## 8. Evidence reproduction (round-2 tree)

```
$ gofmt -l .            # empty
$ go build ./...        # exit 0
$ go vet ./...          # exit 0
$ staticcheck ./...     # no findings
$ gosec -quiet ./...    # no findings
$ govulncheck ./...     # 0 in called code; 1 in an uncalled module
$ grep -rn 'context.TODO' --include='*.go' .   # none
$ go test ./... -race -count=1
ok  govmomi-cli/cmd                 (TestBindFlags_FlagOverridesEnv PASS)
ok  govmomi-cli/internal/config
ok  govmomi-cli/internal/inventory  (--- SKIP: TestListVMsByPortGroup_StandardPG_ExactSet)
# 12 pass, 1 SKIP, 0 fail   <-- the SKIP breaks criterion 8
```

Live run against the embedded `simulator` (VPX; 8 vm / 3 ds / 3 pg), driven
**with flags**:

```
# --help — all 6 flags registered:
Flags:
      --config string      ...   --insecure           ...   --password string  ...
      --timeout duration   ...   --url string         ...   --username string  ...

# C1 PROBE — env points to a bad host, --url to the good sim → FLAG WINS:
$ VSPHERE_URL=http://wrong:1/sdk gcli vms --url http://127.0.0.1:<port>/sdk \
      --username user --password pass --insecure
NAME            VCPU  RAM (GiB)  STORAGE
DC0_C0_RP0_VM0  1     0.0 GiB    0.0 GiB
... (16 VMs)                                  # connected to <good>, NOT wrong:1

# vswitches — HOST column (N2), standard USED<TOTAL, DVS USED=N/A (C2):
SWITCH    HOST       SWITCH TYPE  PORTGROUP           VLAN    UPLINKS  LACP      TOTAL PORTS  USED
DVS0      N/A        distributed  DC0_DVPG0                   N/A      disabled  1            N/A
vSwitch0  DC0_H0     standard     Management Network          ...      N/A       1536         6
vSwitch0  DC0_C0_H0  standard     Management Network          ...      N/A       1536         6
vSwitch0  DC0_C0_H1  standard     VM Network                  ...      N/A       1536         6
... (per-host rows, distinct HOST values)

# --portgroup DC0_DVPG0 → 16 VMs (N1 association correct)
# datastores — TYPE=unknown (legit vcsim degrade), USED/AVAILABLE correct
```

---

## 9. Confidence & limitations

- **High confidence** on C1 (env-beats-flag reproduced live *and* via a passing
  end-to-end unit test), C2 (DVS `USED=N/A`, standard `6/1536`, both live), N2
  (distinct HOST values live), the firing `t.Skip` (reproduced; deterministic),
  and all linter/build/test/scanner results (reproduced).
- **N1** is fixed by code reading (correct `.Self.Value` keying) and is consistent
  with the live 16-VM result; the *ordering* hazard it originally described is
  vcsim-masked, and the residual length-assumption panic is likewise not
  reproducible on vcsim (ordered, complete results) — flagged as latent.
- **Standard-PG `--portgroup`** remains **unverified-positive** (vcsim attaches no
  VMs to standard PGs) — the same limitation as both prior audits, and the direct
  cause of the §4 skip.
- **H1 live fidelity** (real FC/iSCSI/NVMe) still needs a live vCenter; on vcsim it
  correctly degrades to `unknown` and the production path is real code.
- govmomi v0.55 has no `vcsim` binary; the live run used an embedded
  `simulator.Model.Service.NewServer()` server (the hermetic unit tests use the
  same `simulator` package). Re-scored against `REVIEW.md` + the spec.

---

## 10. Prioritized remediation (round 3 — not applied)

**Critical**
1. **Remove the firing `t.Skip` (criterion 8).** In
   `TestListVMsByPortGroup_StandardPG_ExactSet`, *construct* the scenario instead
   of skipping: attach a known VM to a standard port group in the simulator model
   (or reconfigure an existing VM's NIC to `VM Network`) and assert the **exact**
   returned set — or, if a standard PG genuinely can't be populated, assert the
   exact set on the **distributed** PG (16 VMs) with no skip. The suite must report
   **zero skips**.

**Medium**
2. **N1-residual:** iterate `range allVms` / `range allDevices` (or length-guard)
   so a short `Retrieve` result can't panic on a real vCenter.
3. **H3-residual:** walk host storage devices **once** and cache LUN→HBA across
   datastores; batch the standard-switch host reads.

**Low**
4. Render UPLINKS as the bare NIC name (`vmnic0`), not the raw `key-vim.host.PhysicalNic-…`.
5. Drop the dead `used = -1` sentinel in `runSwitchesMode`.
