---
name: qwen-3.8-max
created: 2026-09-01
model: qwen3.8-max — Alibaba **hosted API**, not local weights. Served via opencode provider `alibaba-token-plan`; serving precision **undisclosed**. Open-weight core is `Qwen/Qwen3.8-2.4T-A95B` (2.4T total / 95B active MoE; 512 experts, 10 routed + 1 shared; 92 layers, hidden 8192; Gated DeltaNet + Gated Attention layout; Multi-Token Prediction in training; BF16; native 262,144 ctx extensible to ~1,010,000; custom `qwen3.8-max` license; open-weighted 2026-08-12). Per that card, "Qwen3.8-Max is the official version based on Qwen3.8-2.4T-A95B with more features, such as vision input & non-thinking support, 1M context length by default, official built-in tools" — and the open weights are text-only with thinking non-disableable. Weight identity between endpoint and checkpoint is **not asserted**. Effort `variant = xhigh`, 56,949 reasoning tokens across 130 messages, 2026-09-01 12:40:52 → 13:26:50.
stage: audited
score: 25 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — qwen-3.8-max

## Wire

**Audit-only record.** Generation was driven by the operator; nothing was observed in flight.
No pre-registered predictions, no gate table, no throughput measurement.

Submission landed at `qwen-3.8-max/` (untracked at audit time), module `vsphere-inventory`, 28
files. Build verified at audit: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean on Go
1.27.0 darwin/arm64 against govmomi v0.56.0.

### Provenance — captured at audit time, not backfilled

| field | value |
|---|---|
| provider | `alibaba-token-plan` |
| model id | `qwen3.8-max` |
| variant (effort) | `xhigh` |
| messages | 130 |
| reasoning tokens | 56,949 |
| window | 2026-09-01 12:40:52 → 13:26:50 (46 min) |

**Naming catch, recorded because it costs a lookup otherwise.** The official open-weight repo
is `Qwen/Qwen3.8-2.4T-A95B`, **not** `Qwen/Qwen3.8-Max` — that path 401s, and the
"Qwen3.8-Max"-named repos on Hugging Face are third-party mirrors.

**Scale context.** ~13× the total parameters and ~16× the active parameters of
`qwen-3.8-flash` (180B/6B core). At 2.4T, no local deployment is plausible: ~4.8 TB at BF16,
~2.4 TB at FP8, ~1.2 TB even at 4-bit — against 256 GB for two pooled DGX Sparks or 512 GB for
an M5 Ultra. *(Those TB figures are arithmetic from the parameter count; the card does not
state a checkpoint size. The Flash comparison figures — 335.28 GiB BF16 / 172.78 GiB FP8 — are
published.)*

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context. Raw report:
[`qwen-3.8-max/REVIEW.md`](../../../../qwen-3.8-max/REVIEW.md).

**PASS WITH CONCERNS — 0 Critical, 0 High, 4 Medium, 0 Low.** The cleanest submission in this
eval to date.

No cheat of any kind: no `t.Skip`, no build-tag fencing, no `recover()`, no tautological
assertions, no stubbed or hardcoded values, no forged evidence. gofmt/vet/staticcheck/
govulncheck all clean; `go test -race -count=1 ./...` clean; `make verify` reproduces green.

### The first submission to get criterion 4 right — and provably so

The classifier performs **no string matching on device identifiers**. `FromTargetTransport`
and `FromHBA` type-switch on concrete govmomi types
(`*types.HostFibreChannelTargetTransport`, `*types.HostInternetScsiTargetTransport`), and
`scsiTransportForDevice` resolves extent → `ScsiLun` (by `DeviceName` or `CanonicalName`) →
`ScsiTopology` → `target.Transport`. Reading the **target transport** is the semantically
correct field and sidesteps entirely the adapter key/device confusion that made
`qwen-3.8-flash`'s traversal inert and the prefix heuristics that made
`deepseek-v4-flash-0731` fabricate.

Unit tests assert **specific** protocols with genuine negative controls —
`HostParallelScsiTargetTransport` ⇒ `unknown`, `HostBlockAdapterTargetTransport` ⇒ `unknown`,
`nil` ⇒ `unknown` — plus a composite `ClassifyDatastore` table covering NFS, an FC extent, an
iSCSI extent and an NVMe namespace with exact expected values.

**Reachability verified by injection, not inferred.** Injecting an iSCSI HBA, matching
`ScsiLun`, a `ScsiTopology` target carrying `HostInternetScsiTargetTransport`, and a VMFS
extent into vcsim, then calling production `ListDatastores`:

```
datastore "LocalDS_0" -> "iSCSI"     PROBE: traversal REAL and reachable
```

This is the same probe that returned `unknown` on `qwen-3.8-flash` and produced a fabricated
`FC` on `deepseek-v4-flash-0731`.

### No N+1 — the best performance result in this field

All retrieval funnels through one helper (`view.go:11-19`): `CreateContainerView` →
`defer v.Destroy(ctx)` → `v.Retrieve` with an explicit property list. Host storage devices are
fetched **once** for all hosts into a `map[ManagedObjectReference]*HostStorageDeviceInfo`
(`datastores.go:50-63`) and looked up per datastore — precisely the pattern both prior runs got
wrong.

### Other verified strengths

- Criterion 6 handled three ways (`Network`, `DistributedVirtualPortgroup`, host
  `Config.Network.Portgroup`), with **both** paths tested against exact name sets
  (`reflect.DeepEqual` on the distributed set; a dedicated `TestVMsOnStandardPortgroup`).
- UPLINKS renders friendly NIC names (`vmnic0`), preferring
  `Spec.Policy.NicTeaming.NicOrder.ActiveNic` — verified in live output.
- Logout runs under `context.WithoutCancel(...)` with its own 10s budget, so cleanup completes
  even when the operation context has already expired. No other submission here did this.

### The four Mediums

- **M-1** — `datastores_test.go:19,40` still asserts transport membership over a set including
  `unknown`, so it cannot fail. Charged Medium rather than High (as on both prior runs) because
  the composite `ClassifyDatastore` table *does* prove specific-protocol behaviour on realistic
  `HostStorageDeviceInfo` structures; only ~12 lines of `classifyDatastoreTransport` wiring are
  unproven by the suite, and the auditor probe confirms that wiring is correct today. Regression
  risk, not an unproven feature.
- **M-2** — direct `pflag` import (`config.go:9`), criterion 8 partial.
- **M-3** — `cmd/` and `internal/vclient` at 0.0% coverage.
- **M-4** — cancellation plumbed (`root.go:49`) but never probed by a test.

### Not run

**The behavioural mutation battery was not run**; `battery_*` fields left empty. `gosec` could
not be installed. No git history in the run directory, so test-churn forensics were not
possible.

## Score

**25 / 30 — PASS WITH CONCERNS.** 0 Critical, 0 High, 4 Medium.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 4 | 7 of 8 criteria met and verified, including the two that broke every prior run. Sole deduction: `pflag` outside the allowed dependency set. |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value; honest `unknown` degrade. Unit tests assert specific protocols with real negative cases. Deduction: M-1. |
| Security | 4 | `insecure` default false; no credential leakage; timeout plumbed at root; logout survives a main-context deadline. staticcheck + govulncheck clean. `gosec` not run. |
| Performance | 5 | No N+1 anywhere; single view helper with explicit property lists and `defer Destroy`; host storage devices batched into a map. |
| Concurrency | 4 | `-race` clean; zero goroutines; views destroyed via `defer`. Deduction: cancellation never probed (M-4). |
| Quality | 4 | gofmt/vet/staticcheck clean; cleanest layering in the series (`internal/transport` pure, `internal/vclient` isolated). Costs: two packages at 0.0% coverage. |

## Compare

Ties `ornith-1.0-35b-fp16` and `ox-alpha-free` at 25, but reaches it on a **first audit with
zero High findings** — ornith needed three remediation rounds, and ox-alpha carried 2 High.
Above `qwen-3.8-flash` (24) and `qwen3.8-27b-bf16` (23); below `ornith-1.0-397B` (28),
`gpt-5.5` (29 after r1) and the `claude-code-opus-4.7` reference (30).

**The result worth reading carefully — and it is a finding about the instrument, not the
model.** `qwen-3.8-max` carries ~13× the total and ~16× the active parameters of
`qwen-3.8-flash`, ran at matched `xhigh` effort on the same provider one day apart, and scored
**one point higher** (25 vs 24). Its four residual findings are all mundane — a dependency
violation, two coverage gaps, one weak assertion — and none is a capability failure. The most
economical reading is that **this task saturates near the top of the rubric**, so the
instrument cannot resolve differences between frontier-class models. That argues for harder
discriminators in benchmark v2 rather than for a Max-over-Flash capability claim.

**Comparability caveats:**

1. **Hosted, not local**; serving precision undisclosed.
2. **The artifact is the API product, not the open checkpoint.** The card documents vision
   input, non-thinking support, 1M default context and **official built-in tools** as
   endpoint-only features, while the open weights are text-only with thinking forced on.
   Built-in tools are material for an agentic build task.
3. **Effort `xhigh`** — directly comparable to `qwen-3.8-flash` only; **not** to
   `deepseek-v4-flash-0731` (`high`), and not to any local run (no variant at all).

## Remediate

## Rescore
