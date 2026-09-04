---
name: omp-qwen-3.8-flash
created: 2026-09-01
model: qwen3.8-flash — Alibaba **hosted API**, not local weights. Served via **omp/18.1.2** provider `alibaba-token-plan`; serving precision **undisclosed**. Same endpoint and model id as the `qwen-3.8-flash` run, which was driven through **opencode** — this row exists to compare the two harnesses. Open-weight core is `Qwen/Qwen3.8-Flash-Next` (180B total / 6B active MoE); per Qwen's card the hosted Flash is *"based on"* Flash-Next *"with more production features, e.g. 1M context length by default, official built-in tools"* — weight identity between endpoint and checkpoint is **not asserted**. Effort **`thinkingLevel: null` (unset/default)**, 244,164 reasoning tokens / 332,988 output across 239 assistant turns, 2026-09-01 16:15:08 → 17:41:16 PDT (86 min).
stage: audited
score: 22 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — omp-qwen-3.8-flash

## Wire

**Audit-only record.** Generation was driven by the operator; the audit side observed nothing
in flight beyond confirming the run had produced a submission. No pre-registered predictions,
no gate table, no throughput measurement.

Submission landed at `omp-qwen-3.8-flash/`, 26 Go files + `Makefile`, `README.md`,
`scripts/verify.sh` and a vendored `tools/vcsim` driver; module
`github.com/ldh/vsphere-inventory`. Build verified at audit: `go build ./...` and
`go vet ./...` exit 0 on Go 1.27.0 darwin/arm64 against govmomi **v0.56.0**. No
`PROGRESS.md`, no `build.log`.

**The run was audited only after the operator confirmed completion.** At the first check
(20 min in) the tree held `go.mod`/`go.sum` only, and auditing an empty tree was refused;
the run went on to write its first source file ~78 min after dispatch.

### Provenance — omp, and a correction to the documented recipe

The 2026-09-01 handoff's omp provenance table is **wrong in two rows**. Both were corrected
against a completed, inert session before being relied on here.

| field | value | where it actually lives |
|---|---|---|
| provider / model | `alibaba-token-plan/qwen3.8-flash` | session `.jsonl` `model_change` records (run-scoped); `agent.db → model_usage` is last-used-only |
| effort | **`thinkingLevel: null`** | session `.jsonl` `thinking_level_change` — **not** "unrecorded" |
| reasoning / output tokens | 244,164 / 332,988 | session `.jsonl` `/message/usage` — **not** `~/.omp/logs/` |
| assistant turns | 239 (237 `toolUse`, 1 `error`, 1 `stop`) | same |
| cwd / session id / prompt / start | `omp-qwen-3.8-flash`, `01a05f41-…a891c1a0e7c4`, *"read govmomi-cli-eval-prompt.md and execute"*, 16:15:45 | `history.db → history` (`created_at` is epoch **seconds**, not ms) |

1. **omp *does* record effort.** The handoff's headline — *"EFFORT/VARIANT: NOT RECORDED
   ANYWHERE… It cannot be reconstructed after the fact"* — is false. `thinking_level_change`
   records carry `thinkingLevel` plus a `configured` field distinguishing an explicit setting
   from an inherited default, timestamped and parent-linked to the adjacent `model_change`.
   That is *richer* than opencode's `$.variant`. **The stated ladderbench-v2 conclusion that a
   harness adapter must capture effort at dispatch time because omp cannot is withdrawn.**
2. **Token accounting is not in the logs.** Across all 24 omp log files there are exactly two
   `usage` objects, both `title-generator: success` — the session-naming sidecar, totalling
   1,440 tokens. Following the documented recipe would report the sidecar's spend as the run's.
   Agent-turn usage lives in the session `.jsonl`. `model_perf` carries cumulative
   `output_tokens` only — no input, no reasoning — and is not a substitute.

**Model contamination checked and cleared.** The session opens on `qwen3.8-max` at 23:15:08Z —
carried over from an earlier session — and switches to `qwen3.8-flash` at 23:15:30Z, *15
seconds before* the eval prompt was submitted. All **239** assistant turns are
`alibaba-token-plan/qwen3.8-flash`; no `max` turn exists. The label is correct.

**Effort is NOT matched to the comparison run** — see Compare. The `xhigh` level in this
session belonged to the pre-prompt `max` selection; switching to flash reset it to `null`.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context (no fresh-context subagent
dispatch, per the operator's standing instruction — the same method as the `qwen-3.8-flash`
comparison run, so the two are method-matched). Raw report:
[`omp-qwen-3.8-flash/REVIEW.md`](../../../../omp-qwen-3.8-flash/REVIEW.md).

**PASS WITH CONCERNS — 1 Critical (accuracy), 2 High, 3 Medium, 1 Low.**

No cheat of any kind. `t.Skip`/`SkipNow`/`//go:build ignore`/`recover()` return nothing and
`spine gate go tskip` reports no findings; no tautological assertion, no stubbed value, no
forged evidence — the tree ships no `build.log` or `PROGRESS.md`, so nothing was claimed that
could be forged. `gofmt -l`, `go vet`, `staticcheck` and `govulncheck` clean;
`go test ./... -race -count=1` clean; `make verify` reproduces green end-to-end against a live
vcsim, exercising both port-group paths. `gosec` **was** installable here and ran (2 × G115,
both triaged not real).

### C-1 (Critical, accuracy) — the transport column emits an empty string on real hosts

The same key-vs-device root cause as the opencode run, with a materially worse manifestation.
`adapterProto` is keyed by HBA **device name** (`datastores.go:130`) but looked up by
`iface.Adapter` (`:161`) and `path.Adapter` (`:180`), which carry the HBA **key** — ground
truth in govmomi's canned ESX data at
`simulator/esx/host_storage_device_info.go:17-18,166,220`.

The mismatch alone would be the comparison run's High. What lifts it to Critical is the
**zero value**: a map miss yields `""`, but the guards compare against `TransportUnknown`
(`"unknown"`), so `""` passes straight through into the rendered TYPE cell. `""` is not one of
the five legal values and is not the spec's graceful degrade — it is invalid output.

**Verified by differential probe against the production `Datastores()` path**, in a scratch
copy; the tree was not modified.

| scenario | `Adapter` = HBA key (real convention) | `Adapter` = device name |
|---|---|---|
| ScsiTopology + iSCSI target transport | `iSCSI` | `iSCSI` |
| ScsiTopology + generic target transport | **`""`** | `iSCSI` |
| MultipathInfo only | **`""`** | `iSCSI` |

The flip is the negative control: the traversal is real and reachable; the identifier space is
wrong.

**And section 3 clobbers section 2.** A real host populates both structures. Topology can
classify correctly via target transport; multipath then re-walks the same LUNs and overwrites
with the zero value:

| input | result |
|---|---|
| ScsiTopology only | `iSCSI` |
| ScsiTopology + MultipathInfo (a real host) | **`""`** |

Multipathing is standard on exactly the FC/iSCSI SANs criterion 4 targets, so a correctly
derived protocol is destroyed on the *common* configuration. End-to-end against govmomi's
canned ESX host data: `datastore LocalDS_0 TYPE=""`.

**Charged Critical accuracy, not Critical integrity** — so the auto-FAIL rule does not fire,
following the `deepseek-v4-flash-0731` precedent (an openly-stated wrong rule, held
consistently, is an unmet hard requirement rather than a cheat). **This is the score-relevant
severity call and is overrulable**: charging it High instead — on the 2026-08-16
transport-unreachable precedent — would put Accuracy at 3 and the total at 23.

**Why the suite misses it:** vcsim populates no `ScsiTopology` and no `MultipathInfo`, so the
defective path never executes under `make verify`. Note the membership assertion
(`sim_datastores_test.go:26-37`) **would** catch `""` — it is not in the legal set. The blind
spot is the **fixture**, not the assertion. That is a real improvement over the comparison
run, whose assertion could not fail at all.

### H-1 (High) — local block adapters classified as NVMe

`targetTransportKind` maps `*types.HostBlockAdapterTargetTransport` → `"nvme"`
(`transport.go:111-112`). Confirmed against canned ESX data: `key-vim.host.BlockHba-vmhba1`
→ `NVMe`. A local disk reported as NVMe-attached — same family as deepseek's `naa.`→FC
fabrication, narrower blast radius.

To the model's credit the *opposite* error is explicitly avoided: `transport.go:41-43`
documents that `naa.` alone is ambiguous across FC/iSCSI/SAS and never classifies on it —
exactly what deepseek got wrong.

### H-2 (High) — N+1 in the distributed-switch path

Two round trips **per DVS port group**: `retrieveOneAll` (`vswitches.go:215`) and
`dvsUsedPorts`→`FetchDVPorts` (`:231`), both inside the portgroup loop. Mechanically
confirmed by `spine gate go n-plus-one` (2 findings, both sites). Everything else is correct —
single `ContainerView` + `PropertyCollector`, explicit minimal property lists, views destroyed.

### Other confirmed findings

- **Medium** — `vswitches.go:231-234` swallows a `FetchDVPorts` error and reports `used = 0`,
  presenting a transport failure as a real count.
- **Medium** — `internal/client` at 18.5% coverage; the connection/auth path is largely
  untested.
- **Medium** — no fixture injects storage topology, so criterion 4 has no reachable test.
- **Low** — `gosec` G115 ×2 (`vswitches.go:99,325`), int→int32 on port counts; triaged not
  real. `verify.sh` asserts only `ROWS >= 1` on `--portgroup`.

### Not run

**The behavioural mutation battery was not run.** Disclosed rather than omitted; `battery_*`
front matter left empty. The comparison run has no battery data either, so omitting it keeps
the harness comparison method-matched. No git history exists in the run directory, so the
test-churn forensics the rubric suggests could not be performed; anti-cheat conclusions rest
on the final tree state.

## Score

**22 / 30 — PASS WITH CONCERNS.** 1 Critical (accuracy), 2 High, 3 Medium, 1 Low.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 2 | 7 of 8 criteria met and verified. Criterion 4 **unmet**: production emits `""` — not a legal value — and misclassifies local block adapters as NVMe. Deps clean (`pflag` indirect only, unlike the comparison run). |
| Integrity | 4 | No cheat, skip, tautology, stub or forged log. Exact-count and disjointness assertions; `tskip` gate clean. Deductions: the fixture cannot reach the transport path; one silent error-swallow to `0`. |
| Security | 4 | `insecure` default false; URL userinfo stripped; password never logged; timeout plumbed; logout deferred. staticcheck + govulncheck + **gosec** all run and clean bar 2 benign G115. |
| Performance | 3 | Correct bulk retrieval everywhere except the DVS path, which carries a mechanically-confirmed N+1 at two sites. |
| Concurrency | 5 | `-race` clean; zero goroutines created; cancellation **and** deadline probed with `errors.Is`, not assumed. |
| Quality | 4 | gofmt/vet/staticcheck clean, real three-layer separation, `%w` throughout, `cmd/` at **92.4%** coverage. Costs: the key/device confusion and the zero-value guard bug. |

## Compare

**22 vs 24 for the same model on the same task through a different harness — but the two runs
are NOT method-matched on effort, and the comparison the pilot was designed to make cannot be
drawn.**

| | `qwen-3.8-flash` (opencode) | `omp-qwen-3.8-flash` (omp) |
|---|---|---|
| effort | `variant: xhigh` | **`thinkingLevel: null`** (unset) |
| reasoning tokens | 88,467 | **244,164** (2.8×) |
| output tokens | 75,374 | **332,988** (4.4×) |
| assistant turns | 263 messages | 239 turns |
| wall clock | 77 min | 86 min |
| score | 24 / 30 | 22 / 30 |

**The effort confound is fatal to the headline comparison and was discovered only at
provenance capture.** The `xhigh` in the omp session belonged to the pre-prompt `qwen3.8-max`
selection; switching to flash reset the level to `null`. So this is *default-effort omp* vs
*xhigh opencode* — a 2-point deficit that cannot be attributed to the harness.

**It also inverts last session's calibration finding rather than confirming it.** That session
established reasoning-token count as the measured quantity and the variant label as merely a
request, on evidence that two runs labelled `max` differed 4.4× in spend. Here the run with
**no** effort label spent **2.8× more reasoning** than the `xhigh` one — the label and the
spend point in opposite directions. Requested effort is not just an unreliable proxy for
spend; on this pair it is anti-correlated.

**What can be said, harness-independently:** the two trees fail in *disjoint* places, which is
the same signal the six-run session recorded. omp's tree is better on integrity surface
(exact-count and disjointness assertions, `cmd/` at 92.4% vs 0.0%, clean dependency set, gosec
runnable) and worse on the two things the rubric weights hardest — it ships an N+1 the
opencode tree does not have, and its criterion-4 defect degrades to an **illegal** value where
the opencode tree's degrades to a legal `unknown`. Both trees got the HBA key/device
distinction wrong, independently, from the same model. That recurrence across harnesses is
better evidence about the *task* than about either harness: **criterion 4's identifier-space
trap is the reproducible failure mode**, and it is the strongest ladderbench-v2 input this
pilot produced.

**Operational finding for v2:** under omp the run wrote no source file for ~78 minutes while
generating continuously (65 model turns, ~68k output tokens by the 20-minute mark), then
produced the entire tree in a burst. Any v2 harness adapter that infers liveness or progress
from worktree mtime would have declared this run dead.
