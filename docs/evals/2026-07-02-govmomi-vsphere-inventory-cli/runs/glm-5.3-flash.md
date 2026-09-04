---
name: glm-5.3-flash
created: 2026-09-01
model: glm-5.3-flash (Z.ai / Zhipu **GLM-5.3-Flash**) — hosted endpoint via opencode provider `zai-coding-plan`; serving precision **undisclosed**. Nominal model is 320B total / 18B active MoE, hybrid sparse + linear attention with Manifold-Constrained Hyper-Connections (mHC), first natively multimodal member of the GLM-5 series, MIT, 1M context. **Second measurement of a model already in this field** — the same model was audited blind as `ox-alpha-free` on 2026-08-21, five days before Z.ai's 2026-08-26 reveal that Ox Alpha was GLM-5.3-Flash. Effort `variant = max`, 55,066 reasoning tokens across 132 messages, 2026-09-01 14:28:44 → 15:13:04.
stage: audited
score: 23 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — glm-5.3-flash

## Wire

**Audit-only record.** Generation driven by the operator; nothing observed in flight.

Submission at `glm-5.3-flash/vsphere-inventory/`, module `vsphere-inventory`, 27 files, govmomi
**v0.46.2** (older than the v0.56/v0.57 used by the sibling runs this week). Build verified at
audit: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean on Go 1.27.0 darwin/arm64.

### Provenance — captured at audit time

| field | value |
|---|---|
| provider | `zai-coding-plan` (Z.ai direct) |
| model id | `glm-5.3-flash` |
| variant (effort) | `max` |
| messages | 132 (cwd-scoped) |
| reasoning tokens | 55,066 |
| window | 2026-09-01 14:28:44 → 15:13:04 |

Two aborted starts preceded it on other providers — `zhipuai` (1 message, 13:57:25) and `zai`
(2 messages, variant `high`, 13:54:53 → 13:55:18) — before settling on `zai-coding-plan`.
Recorded so the 13:54–13:58 activity is not mistaken for part of the run.

**This run exists to be a repeat measurement.** `ox-alpha-free` is the same model under its
pre-reveal cloaked name. See Compare.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context. Raw report:
[`glm-5.3-flash/REVIEW.md`](../../../../glm-5.3-flash/REVIEW.md).

**PASS WITH CONCERNS — 0 Critical, 1 High, 5 Medium, 0 Low.**

No cheat of any kind: no `t.Skip`, no build-tag fencing, no `recover()`, no tautologies, no
stubbed values, no forged evidence. gofmt/vet/staticcheck/govulncheck clean; `-race` clean with
no goroutines, channels or `sync` primitives in non-test code; `make verify` reproduces green.

### The transport implementation is the strongest in this field

`HBADescriptor` type-switches on **nine** concrete govmomi HBA types — FC, FCoE, iSCSI, PCIe,
TCP, ParallelScsi, SerialAttached, RDMA, Block — broader coverage than any prior submission,
including NVMe/TCP and NVMe-over-PCIe. No string-prefix heuristics anywhere (the failure mode
that made `deepseek-v4-flash-0731` fabricate `FC`). The lookup chain is
`descriptorByAdapterKey[hba.Key]` resolved via `iface.Adapter` — **key matched against key**,
the correct VMware convention and exactly what `qwen-3.8-flash` got backwards. Volume matching
is by UUID **and** name, robust against the `vmfsUUID` parsing defect that made
`qwen3.8-27b-bf16`'s classifier unreachable.

**Reachability proven by injection.** Because this implementation reads host storage facts off
`HostStorageSystem` (`configManager.storageSystem`) rather than `host.Config.StorageDevice`,
the probe injects there:

```
inject HostVmfsVolume(extent naa.60014051…) + iSCSI HBA + ScsiLun
     + ScsiTopology(Adapter = HBA key)
  -> production ListDatastores: LocalDS_0 -> "iSCSI"      PROBE PASS
```

### H-1 — DVS LACP derived from the wrong field

`switches.go:191` sets `s.LACP = LACPEnabled` when `cfg.LacpApiVersion != ""`.
`VMwareDVSConfigInfo.LacpApiVersion` reports which LACP **API version/mode** the switch
supports — a capability field, not a statement that any link aggregation group exists. A vDS
with the LACP API enabled and zero `LacpGroupConfig` entries is reported as `LACP enabled`. The
correct signal is a non-empty `LacpGroupConfig`, which both sibling runs this week used.

**Latent under vcsim** — live output correctly shows `disabled` because vcsim leaves
`LacpApiVersion` empty — so neither the suite nor `make verify` can catch it. Found by reading,
not observation; charged the same class as `qwen-3.8-flash`'s H-1 (a real field read with the
wrong semantics).

### Notable strengths, recorded because they are field-firsts

- **`verify.sh` is the only gate in this field that asserts failure paths** — an unknown port
  group and a missing `VSPHERE_URL` must each exit non-zero.
- **Best port-group test coverage** — `TestVMsOnPortgroup` covers two distributed port groups,
  the standard `VM Network`, *and* an unknown-name error case.
- The N+1 is **memoized**: `storageFactCache` caches per-host topology across datastores, so
  cost is O(hosts), not the O(datastores × hosts) that sank the DeepSeek run.

### The five Mediums

- **M-1** — datastore sim assertion accepts membership including `unknown`, so it cannot fail
  (the universal pattern in this field). Charged Medium because the pure-function table tests
  do prove specific protocols and the probe confirms the wiring.
- **M-2** — **standard vSwitch rows silently aggregate across hosts.** `switches.go:79-91` keys
  on `vs.Name` and accumulates `s.Ports += vs.NumPorts`. With four hosts each carrying a
  1536-port `vSwitch0`, live output reports `PORTS 6144 / USED 24` — honest arithmetic
  (4 × 1536, 4 × 6) but a figure matching no actual switch, with nothing signalling aggregation.
- **M-3** — DVS `UPLINKS` renders `unknown`; uplink policy read only from
  `DVSNameArrayUplinkPortPolicy`.
- **M-4** — `cmd/` at 0.0% coverage.
- **M-5** — cancellation plumbed (`cmd/connect.go:33`) but never probed by a test.
- **M-6** — still two `RetrieveOne` per host rather than one batched `Retrieve` over all hosts.
  Noted in fairness: reading `HostStorageSystem.fileSystemVolumeInfo` is the more correct API
  path for extent→volume mapping, so the extra hop buys real fidelity.

### Not run

**The behavioural mutation battery was not run**; `battery_*` fields left empty. `gosec` could
not be installed. No git history in the run directory.

## Score

**23 / 30 — PASS WITH CONCERNS.** 0 Critical, 1 High, 5 Medium.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 3 | Criterion 4 fully met and verified by injection — strongest in the field. Criterion 5 defective (H-1 LACP). Criterion 8 partial (`pflag`). Standard rows aggregate across hosts (M-2). |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value. Specific-protocol tests with real negatives; `TransportForExtents` table-tested including two must-return-`""` cases. Deduction: M-1. |
| Security | 4 | `insecure` default false at flag and Viper default; no credential leakage; timeout plumbed. staticcheck + govulncheck clean. `gosec` not run. |
| Performance | 4 | Per-host facts memoized across datastores — O(hosts), not O(datastores × hosts). Deduction: M-6. |
| Concurrency | 4 | `-race` clean; no goroutines, channels or `sync` in non-test code. Deduction: cancellation never probed. |
| Quality | 4 | gofmt/vet/staticcheck clean; best `verify.sh` and best port-group coverage in the field. Cost: `cmd/` 0.0% coverage. |

## Compare

Sits mid-field: below `ox-alpha-free` (25 — **the same model**), `qwen-3.8-max` (25) and
`qwen-3.8-flash` (24); level with `qwen-agentworld-35b-a3b` (23, after two remediation rounds)
and `qwen3.8-27b-bf16` (23); above `deepseek-v4-flash-0731` (18).

### The repeat measurement — this field's only test–retest pair

| | `ox-alpha-free` | `glm-5.3-flash` |
|---|---|---|
| audited | 2026-08-21 (blind, pre-reveal) | 2026-09-01 |
| provider | `opencode` / `x-preview-f-free` | `zai-coding-plan` / `glm-5.3-flash` |
| variant **label** | `max` | `max` |
| messages | 177 | 132 |
| **reasoning tokens** | **12,542** | **55,066** |
| **score** | **25 / 30** | **23 / 30** |

**The pair is less controlled than the matching labels suggest.** Both are labelled `max`, but
actual reasoning expenditure differs by **4.4×**. The variant label is evidently not portable
across providers — opencode's free stealth gateway and Z.ai's direct endpoint interpret `max`
very differently, or one caps reasoning independently. **Future comparisons must treat reasoning
token count as the measured quantity and the variant label as, at best, a request.**

*Attribution method, recorded because the naive query is wrong:* model id `x-preview-f-free` was
used across 11 sessions and five project directories over six days (436 messages). Scoping by
`$.path.cwd` **and** cutting at the audit date isolates the generation run at 177 messages. A
model-id-level count overstates it by 2.5×.

**What the pair supports:** reproducibility at the level of *"no cheat, transport genuinely
works, score in the low-to-mid 20s"*. Both runs independently produced a real, reachable
transport classifier and neither cheated. The 2-point spread comes from *different* defects —
ox-alpha's phantom port-group row and unfailable verify check versus this run's LACP field
error — not a consistent weakness.

**What it does not support:** a clean run-to-run variance estimate. Serving path, reasoning
expenditure and audit date all differ. Combined with the Max-vs-Flash result (25 vs 24 at 13×
the parameters), two weak signals now point the same way: **the instrument's resolution may be
coarser than the deltas being read off it.** That belongs in the v2 design discussion.

*Auditor caveat:* both audits were run by the same auditor against the same rubric eleven days
apart, and the probe methodology developed materially in between. Some of the spread may be
auditor drift rather than model variance. A blind re-audit of `ox-alpha-free` with the current
probe set is the cheapest test of whether the benchmark or the auditor is the noisy component.

## Remediate

## Rescore
