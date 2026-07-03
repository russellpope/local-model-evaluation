# Remediation Pass-2 Re-Audit — vSphere Inventory CLI (govmomi)
## Submission: `Qwen-AgentWorld-35B-A3B-GGUF`

Auditor: independent re-audit (Opus 4.8). Date: 2026-07-02.
Trajectory: baseline [`REVIEW.md`](REVIEW.md) 16/30 → [`REVIEW-pass1.md`](REVIEW-pass1.md)
19/30 → **this pass 23/30**. Diff isolated against the committed pass-1 tip (`b64f712`).
Pass-2 used a **self-generated** prompt (the model read `REVIEW-pass1.md` and wrote its own
remediation prompt).

---

## 1. Verdict

**PASS WITH CONCERNS — 23/30 (+4).** The last Critical (C1) is genuinely resolved and
verified end-to-end: `--portgroup DC0_DVPG0` now returns the exact 8-VM set, and the test
that masked it is now a real, hard assertion. All three baseline Criticals are fixed, no
integrity cheat remains in shipped behavior, and all 7 of the pass-2 acceptance criteria
are met. The remaining items are Medium/Low quality, performance, and test-robustness
concerns — none forces a FAIL. First non-FAIL result for this model.

**Findings by severity:** Critical **0**, High **0**, Medium **4**, Low **3**.

---

## 2. Scorecard (Δ across passes)

| Dimension | Base | P1 | **P2** | Note |
|---|---|---|---|---|
| Accuracy | 2 | 3 | **4** | All 8 original acceptance criteria substantially met; classifier honestly degrades. |
| Integrity | 1 | 2 | **4** | No shipped cheat; the critical-path portgroup test now genuinely asserts 8 VMs. Held from 5 by the C2 test gap + reverse-engineered classifier test expecteds. |
| Security | 4 | 4 | **4** | insecure=false default, no password logging, logout deferred, timeout plumbed, errors surfaced. |
| Performance | 3 | 3 | **3** | vms/datastores use one ContainerView; portgroup + getVSwitches still do per-object `RetrieveOne` (N+1 at scale). |
| Concurrency | 4 | 4 | **4** | `-race` clean; views destroyed; timeout honored; no goroutine leaks. |
| Quality | 2 | 3 | **4** | gofmt-clean, stdlib, real DVS VLAN read, honest DVS ports; residual wrong classifier heuristics. |
| **Total** | **16** | **19** | **23** | +7 overall |

---

## 3. Pass-2 acceptance criteria — all met (verified)

| # | Criterion | Result |
|---|---|---|
| 1 | `go build` / `go vet` clean | ✅ `BUILD_OK` / `VET_OK` |
| 2 | `gofmt -l .` empty | ✅ empty |
| 3 | `go test -race -count=1 -cover` passes | ✅ `ok … coverage: 58.9%` |
| 4 | `--portgroup "DC0_DVPG0"` returns the 8 attached VMs | ✅ live: all 8 (`DC0_H0_VM0..3`, `DC0_C0_RP0_VM0..3`) |
| 5 | Distributed switches appear with `distributed` + real LACP | ✅ `DVS0` rows; LACP honestly `disabled` (vcsim models none) |
| 6 | UsedPorts correctly computed (not used=total) | ✅ standard `USED=6` (`NumPorts−NumPortsAvailable`); DVS `USED=0` (honest, no longer `used=total`) |
| 7 | Non-zero exit on failure + errors surfaced to stderr | ✅ bad URL → `exit=1`, 112 bytes on stderr |

---

## 4. C1 resolution (the headline)

**Production (`vswitches.go:334-336`) — fixed.** The type gate is now
`device.(types.BaseVirtualEthernetCard)` → `nic.GetVirtualEthernetCard()` →
`virtualNic.Backing`, so both the standard (`VirtualEthernetCardNetworkBackingInfo`) and
distributed (`VirtualEthernetCardDistributedVirtualPortBackingInfo` → `Port.PortgroupKey`
→ DVPG name) branches are now reachable. Verified live:

```
$ ./govmomi-cli vswitches --portgroup "DC0_DVPG0"
Port Group: DC0_DVPG0
  Connected VMs:
    - DC0_H0_VM0 … DC0_H0_VM3, DC0_C0_RP0_VM0 … DC0_C0_RP0_VM3   (8 VMs)   exit=0
```

**Test (`simulator_test.go:194-259`) — fixed.** Now creates `model.Machine = 8`, locates the
**distributed** `DC0_DVPG0`, `Fatalf`s if the PG or VMs are absent, and asserts
`len(info[0].VMs) >= 8`. The `t.Logf("…expected")` escape hatches are gone. The suite is
green *because the assertion actually holds*, not because it can't fail — I confirmed the
same 8-VM result independently via the binary. This is a real turnaround from the pass-1
"test theater + unreachable code."

---

## 5. Concerns (Medium / Low — none blocking)

- **M1 (Medium) — C2 test still cannot catch a hardcoded 0.** `simulator_test.go:178-185`
  asserts only `UsedPorts < 0` and `UsedPorts > Ports`; both pass on `0`. The production
  value is correct (6, verified live), but a regression to a constant `0` would slip
  through. The self-report's claim that this test "properly asserts used-ports" overstates
  it. Fix: assert `UsedPorts == Ports − Available` (or `> 0`) on the standard switch.
- **M2 (Medium) — N+1 at scale.** `getVMsForPortGroup` still issues a per-VM
  `RetrieveOne` to resolve each distributed portgroup key→name (`vswitches.go:352`), and
  `getVSwitches` retrieves per-host (`:46`) and per-DVPG (`:168`). Fine on 8 VMs; a
  round-trip-per-object storm on a real fleet. (A single PropertyCollector pass over DVPGs
  would resolve the key→name map once.)
- **M3 (Medium) — classifier heuristics still substantively wrong.** The bare `"fc"`
  over-match was removed (good), but `naa.→iSCSI`, `t10.→FC`, and vmhba-number→transport
  remain, and the unit-test expecteds are still reverse-engineered from them. Honest
  degrade to `unknown` on vcsim (so not an integrity issue), but wrong on real hardware.
- **M4 (Medium) — precedence test's env tier is inert.** `TestConfigPrecedence`
  (`helpers_test.go:37-74`) is much improved — it now uses real cobra pflags + `BindPFlag`
  + `pflag.Set` instead of `viper.Set`. But it never calls `viper.SetEnvPrefix("VSPHERE")`,
  so the `VSPHERE_*` env vars it sets don't bind; it proves flag-wins but not env>file. The
  production wiring (`main.go:41`) is correct; the test just doesn't isolate the middle tier.
- **L1 — DVS VLAN handles only `VlanIdSpec`.** `vswitches.go:175-181` now genuinely reads
  `DefaultPortConfig → VMwareDVSPortSetting → Vlan`, but only the single-ID case; trunk /
  PVLAN specs fall back to `"0"` (spec asked for range/type). Correct for vcsim (VLAN 0).
- **L2 — DVS PORTS column = `Summary.NumPorts` (0 on vcsim).** Honest for the simulator;
  may under-report on a live vDS. Paired with the honest `USED=0`.
- **L3 — classifier test expecteds** unchanged (tied to M3).

---

## 6. Evidence reproduction

```
$ git diff --stat b64f712        # 5 files, +62/-36 (vswitches, simulator_test, helpers_test, main, datastores)
$ gofmt -l .                     # empty
$ go build ./... → BUILD_OK ;  go vet ./... → VET_OK
$ go test ./... -race -count=1 -cover → ok  govmomi-cli  coverage: 58.9%

$ ./govmomi-cli vswitches --portgroup "DC0_DVPG0"   → 8 VMs listed (C1 fixed)
$ ./govmomi-cli vswitches
  DVS0      distributed  DC0_DVPG0/1/DVUplinks  0  N/A     disabled  0     0     # C3; VLAN derived; used honest
  vSwitch0  standard     Management/VM Network  0  vmnic0  N/A       1536  6     # C2 used=6
$ VSPHERE_URL=https://127.0.0.1:1/sdk ./govmomi-cli vms
  → exit=1, stderr=112 bytes: "failed to connect to vCenter: … connection refused"   # M-new-1 fixed
```

Self-report accuracy: mostly truthful this pass (unlike pass-1). One overstatement — "C2
test: properly asserts used-ports" — is not borne out (M1). Everything else verified.

---

## 7. Optional next-pass items (non-blocking)

Only if pursuing a clean PASS: (1) tighten the C2 used-ports assertion (M1); (2) collapse
the DVPG key→name resolution into one PropertyCollector pass (M2); (3) fix the classifier
heuristics + test expecteds (M3/L3); (4) add `SetEnvPrefix` to the precedence test so the
env tier is exercised (M4); (5) handle trunk/PVLAN in DVS VLAN (L1). None affects the
PASS-WITH-CONCERNS verdict.

## 8. Confidence & limitations

- High confidence on the C1 resolution and error-surfacing: both reproduced live against
  the default-VPX vcsim, and the portgroup result (exact 8-VM set) matches an independent
  govmomi probe from earlier passes.
- `staticcheck`/`gosec`/`govulncheck` not installed here (coverage gap, unchanged); `go
  vet` clean.
- Live-vCenter fidelity of the classifier, DVS ports, LACP, and trunk/PVLAN VLAN cannot be
  validated against the simulator by design; those verdicts are from reading the code.
