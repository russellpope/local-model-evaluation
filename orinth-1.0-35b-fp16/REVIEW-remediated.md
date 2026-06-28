# Remediation Re-Audit — vSphere Inventory CLI (govmomi)

**Submission:** `orinth-1.0-35b-fp16/govmomi-cli` — **remediated** (branch `ornith-remediation-attempt`, working tree atop baseline `86036fa`)
**Auditor:** Claude Opus 4.8 (independent re-audit, read-only)
**Date:** 2026-06-27
**Baseline:** the original audit is preserved in [`REVIEW.md`](REVIEW.md) (FAIL, 16/30).
**Method:** treated the remediated tree as a fresh untrusted submission — re-ran the full
reproduce-everything pass (build/vet/gofmt/`-race`/staticcheck) and drove every
subcommand against a live `simulator` server, **with flags**, plus targeted probes
of the three predicted weak spots (G1/G2/G3). Scored against the **original
findings + the eval spec**, not the model's self-authored remediation prompt.

---

## 1. Verdict

### `FAIL` (improved) — score **16 → 20 / 30** (+4)

The remediation is **substantial and largely genuine**: 9 of the 16 findings are
fully fixed (including two of three Criticals — standard vSwitches now list, the
transport feeder is real), the linters are green, the missing deliverables exist,
and most tests were strengthened honestly (the `t.Skip` is gone, the dead code is
gone, a real port-group assertion replaced "zero is fine"). But it remains a
**FAIL** on two unmet hard acceptance criteria, and it introduced two new bugs:

- **Criterion 2 (flag overrides env) still UNMET (C1).** Flags now *parse* and
  appear in `--help`, but their **values are silently dropped** — `bindFlags`
  looks up the flag on the *subcommand's* empty `PersistentFlags()`, so the
  binding no-ops and env/default win. Verified live: `--url good` with a bad env
  connected to the **env** host. The model's new "real flag" test passes anyway
  because it binds its own flagset instead of the actual CLI wiring.
- **Criterion 5 (used = total − available) still UNMET for distributed (C2).**
  Standard switches now compute USED correctly (1536/6 live), but distributed
  rows still show **USED == TOTAL** (1/1 live) because `AvailablePorts` is never
  set for DVS — and the strengthened test *exempts* distributed from the check.

**Findings by severity (remediated):** Critical 1 · High 2 · Medium 2 · Low ~3
(down from Critical 3 · High 3 · Medium 4 · Low 5). Two of the High/Medium are
**newly introduced** by the remediation.

---

## 2. Scorecard — original → remediated

| Dimension     | Was | Now | Why it moved |
|---------------|:---:|:---:|--------------|
| Accuracy      | 2 | **3** | C3 (standard switches), H1 (transport plumbing), H2 (timeout) fixed; vms/datastores correct. Held back: criterion 2 (flags) and criterion-5-distributed still unmet. |
| Integrity     | 2 | **3** | `t.Skip` removed, dead-code gone, port-group test now requires a non-empty exact-ish result, standard used-ports asserted. Held back: two green tests still mask real bugs (C1 flag path, C2 distributed used) + one new latent bug shipped. |
| Security      | 4 | **4** | Unchanged — insecure default false, no password leak, deferred logout. (Minor new footgun: a silently-ignored `--url`/`--password` could send creds to the wrong/default endpoint.) |
| Performance   | 2 | **3** | N+1 killed on the primary paths (VMs, datastores, port-group, DVS all batched via `pc.Retrieve`). Held back: HBA enrichment re-walks all hosts per datastore (O(ds×hosts)); standard-switch path is per-host. |
| Concurrency   | 4 | **4** | Still `-race` clean, no goroutines/leaks. `context.TODO()` removed from the port-group path but a new one appeared in the HBA path — net wash. |
| Quality       | 2 | **3** | gofmt + staticcheck clean, dead/duplicate formatters removed, Makefile + README added, headers fixed. Held back: duplicate per-host standard-switch rows, the latent ordering bug, residual `context.TODO`. |
| **Total**     | **16** | **20** | +4 |

---

## 3. Finding-by-finding: original → remediated

| ID | Original finding | Status | Evidence |
|----|------------------|--------|----------|
| **C1** | CLI flags entirely dead (`unknown flag`) | **STILL UNMET** (mechanism changed) | Registration moved to `init()` → flags now parse and show in `--help`. But `bindFlags` (`root.go:74`) does `v.BindPFlag("url", cmd.PersistentFlags().Lookup("url"))` where `cmd` is the **subcommand**, whose own `PersistentFlags()` has no `url` → `Lookup` returns nil → binding no-ops. Live: env=`http://wrong:1/sdk` + `--url <good>` → "connecting to vCenter at **wrong:1**". Flag silently ignored; **criterion 2 fails**. Now a *silent* failure (was loud). |
| **C2** | `vswitches` USED fabricated (== total) | **PARTIAL** (standard fixed, distributed not) | Standard: `Used = NumPorts − NumPortsAvailable` (`switches.go:96-101`); live `vSwitch0` shows TOTAL=1536 / **USED=6** ✓. Distributed: `AvailablePorts` never set → `Used = Total − 0 = Total`; live `DVS0` shows TOTAL=1 / **USED=1** ✗. `switches_test.go:72-81` only checks `used<total` for `SwitchType=="standard"`. |
| **C3** | Standard vSwitches silently dropped | **FIXED** | Now reads `config.network` off `HostSystem` and **returns/wraps** the error instead of `continue` (`switches.go:72-73`). Live: `vSwitch0 / Management Network` and `VM Network` rows present. |
| **H1** | Transport feeder stubbed (`return nil,nil`) | **FIXED (code)** | Real implementation: reads `config.storageDevice.hostBusAdapter`, builds LUN→HBA map from `scsiTopology`, classifies FC/iSCSI/NVMe (`datastores.go:108-180`); misleading comment removed. Degrades to `unknown` on vcsim (legitimate). Live-vCenter fidelity still unverifiable here. |
| **H2** | Timeout not plumbed into retrieval | **FIXED** (one residual) | Each RunE builds `context.WithTimeout(rootCtx, cfg.Timeout)` and threads it through connect **and** retrieval (`cmd/*.go:24-34`); `newClient` no longer cancels early; `matchingVM`'s `context.TODO()` gone. Residual: `dsInfoFromMo` still calls `hostHBAsForDatastore(context.TODO(), …)` (`datastores.go:83`). |
| **H3** | N+1 retrieval everywhere | **MOSTLY FIXED** | `ListVMs`, `ListVMsByPortGroup`, `ListDatastores`, DVS listing all batch via `pc.Retrieve`. Residual: `hostHBAsForDatastore` re-creates a host view and retrieves per host, **per datastore** (O(ds×hosts)); `listStandardSwitches` is per-host. |
| **M1** | Vacuous / weak tests | **PARTIAL** | `portgroup_test`: `t.Skip` removed, now **requires ≥1 VM** + `t.Errorf` on bad names ✓. `switches_test`: asserts standard `used<total` and requires a standard row ✓. `config_test`: added `TestBindPFlagEndToEnd` ✓ intent — but it binds its own flagset, so it **doesn't catch the real C1 bug**, and the distributed-used check is exempted. Two green tests still mask real failures. |
| **M2** | Dead code (SA4006) | **FIXED** | `staticcheck ./...` clean; the dead `totalPorts` block is gone. |
| **M3** | Missing deliverables | **FIXED** | `Makefile` with a real `verify:` target (lint/vet/staticcheck/test/sec/vulncheck) + `README.md` added. |
| **L1** | gofmt dirty (8 files) | **FIXED** | `gofmt -l .` empty. |
| **L2** | `FormatBytes` redundant branch | **FIXED** | Collapsed (`bytefmt.go:16-21`). |
| **L3** | Duplicate `FormatGB` | **FIXED** | Deleted. |
| **L4** | Header/units `RAM (GB)` vs `GiB` | **FIXED** | Headers now `RAM (GiB)`. |
| S-Low | timeout availability | **FIXED** | Subsumed by H2. |

**Net: fixed C3, H1, H2, H3(mostly), M2, M3, L1–L4, S-Low; partial C1, C2, M1.**

## 4. New findings introduced by the remediation

The audit treats the remediated tree as fresh — these did not exist before.

- **N1 (High, latent / sim-masked) — positional result-association in
  `ListVMsByPortGroup`.** The batching keys results by input-ref index:
  `vmInfoByRef[vmRefs[i].Value]` driven by the *returned* slice position
  (`vms.go:135-141`), and likewise for device backings (`:154`) and network
  names (`:171`). `property.Collector.Retrieve` does **not** guarantee the
  returned objects match input-ref order or count, so on a real vCenter this can
  bind a VM's identity to **another VM's** NICs/storage. Masked on vcsim (ordered
  results). The correct key is each returned object's `.Self.Value`. This trades
  the old N+1 *perf* issue for a *correctness* landmine at the same fleet scale.

- **N2 (Medium) — duplicate standard-switch rows, no host disambiguation.** Now
  that standard switches list (C3), `listStandardSwitches` emits one row per
  *(host, port group)* with **no HOST column**, so a 4-host VPX prints four
  byte-identical `vSwitch0 / Management Network` rows and four `VM Network` rows
  (live output below). Each host genuinely has its own vSwitch, but with no host
  column the output reads as duplicate noise and explodes on real clusters.

## 5. Security

Unchanged and still the strongest dimension: `insecure` defaults false; password
via `url.UserPassword` → `Login` (never in a URL); logout deferred; no shell
surface. One **new mild footgun** from C1: because `--url`/`--password`/etc. are
silently ignored, a user who passes `--url prod-vc --password …` expecting those
to take effect will instead connect with whatever the env/config/default holds —
i.e., credentials/targets can silently differ from what was typed. Not a leak, but
a confusing-and-potentially-wrong-endpoint hazard. (`gosec`/`govulncheck` were
clean in the original audit; unchanged code paths.)

## 6. Evidence reproduction (remediated tree)

```
$ gofmt -l .            # empty
$ go build ./...        # exit 0
$ go vet ./...          # exit 0
$ staticcheck ./...     # no findings   (was: SA4006)
$ go test ./... -race -count=1 -cover
ok  govmomi-cli/internal/config      coverage: 95.2%
ok  govmomi-cli/internal/inventory   coverage: 71.7%
# PASS, zero failures, zero skips
```

Live run against the embedded `simulator` (VPX, 8 vm / 3 ds / 3 pg):

```
# C1 — flags now appear in --help (registration fixed):
Flags:
      --config string      ...
      --insecure           ...
      --password string    ...
      --timeout duration   ...
      --url string         ...
      --username string    ...

# C1 — but the VALUE is dropped. env=bad, flag=good → env wins:
$ VSPHERE_URL=http://wrong:1/sdk gcli vms --url <good> --username user --password pass --insecure
connecting to vCenter at wrong:1: ...   # <-- flag ignored; criterion 2 FAILS

# vswitches (env path) — C3 fixed (standard rows present); C2 split:
SWITCH    SWITCH TYPE  PORTGROUP           VLAN    UPLINKS   LACP      TOTAL PORTS  USED
DVS0      distributed  DC0_DVPG0                   N/A       disabled  1            1     # USED==TOTAL (C2 unfixed)
DVS0      distributed  DC0_DVPG1                   N/A       disabled  1            1
vSwitch0  standard     Management Network          vmnic0    N/A       1536         6     # USED real (C2 fixed)
vSwitch0  standard     Management Network          vmnic0    N/A       1536         6     # duplicate row (N2)
vSwitch0  standard     VM Network                  vmnic0    N/A       1536         6
vSwitch0  standard     VM Network                  vmnic0    N/A       1536         6
...

# vms — consumed storage, sorted (correct):
NAME            VCPU  RAM (GiB)  STORAGE
DC0_C0_RP0_VM0  1     0.0 GiB    0.0 GiB ...

# datastores — TYPE=unknown (legit vcsim degrade), USED/AVAILABLE correct:
LocalDS_0  unknown  160.0 GiB  3.8 TiB ...

# --portgroup DC0_DVPG0 → 16 VMs (distributed lookup works); "VM Network" → 0 (standard, plausibly correct-empty)
```

## 7. Did it fix what its own remediation prompt targeted?

Mostly yes — but the two items the self-authored prompt got *technically wrong*
are exactly the two that survived, as predicted before the run:
- The prompt told it to "leave AvailablePorts zero if unavailable" for
  distributed switches → it did → **USED==TOTAL persists** (G1). ✔ predicted.
- The prompt added a flag test but the real wiring bug (subcommand flagset
  lookup) is untested → **C1 still ships green-and-broken** (G2/the C1 binding). ✔ predicted.
- Its H2 verification (`VSPHERE_TIMEOUT=1ns`) is unfalsifiable, but the H2 *code*
  fix is correct regardless (verified by reading + threading). G3 was a
  proof-quality issue, not a fix issue.

This is the headline lesson, intact through the remediation: a self-authored
scaffold/test that *looks* right can still validate the wrong thing — the model
fixed what it could see and tested what it fixed, leaving the same green-masks-
broken pattern on a narrower surface.

## 8. Prioritized remediation (round 2 — not applied)

**Critical**
1. **C1:** in `bindFlags`, look up flags on `cmd.Flags()` (merged/inherited) or
   `rootCmd.PersistentFlags()`, not `cmd.PersistentFlags()` (the subcommand's
   empty set). Then add a test that drives the **actual** `rootCmd`/subcommand
   through `Execute()` with `--url` and asserts it beats env — not a hand-built
   flagset.

**High**
2. **C2-distributed:** derive DVS used ports from real port state (e.g.
   `FetchDVPorts`/portgroup port usage) or render **N/A** when underivable —
   never `Total − 0`. Extend the test's `used<total`/exact-value check to
   distributed rows.
3. **N1:** key batch results by each returned object's `.Self.Value`, not by
   input-ref position.

**Medium**
4. **N2:** add a HOST column (or dedup cluster-wide port groups) for standard
   switches.
5. **M1-residual:** make the port-group test assert the **exact** VM set and
   cover a **standard** PG; gate nothing behind the type that's still broken.

**Low**
6. Replace the `context.TODO()` in `dsInfoFromMo`/`hostHBAsForDatastore` with the
   caller ctx; cache the host storage-device walk across datastores.

## 9. Confidence & limitations

- **High confidence** on C1 (env-beats-flag reproduced live), C2 (USED==TOTAL on
  DVS, USED=6/1536 on standard, both live), C3 (standard rows present live), and
  all linter/build/test results (reproduced). N1 is **code-read, not reproduced**
  (vcsim returns ordered results, masking it) — flagged as latent, consistent
  with how H1 was handled originally.
- **H1 live-fidelity** (real FC/iSCSI/NVMe) still requires a live vCenter; on
  vcsim it correctly degrades to `unknown`, and the production path is now real
  code rather than a stub.
- **Standard-PG `--portgroup`** remains unverified-positive (vcsim attaches no
  VMs to the standard `VM Network`), same limitation as the original audit.
- Re-scored against `REVIEW.md` + the spec. The remediation lives in the
  `ornith-remediation-attempt` working tree (uncommitted) atop `86036fa`.
