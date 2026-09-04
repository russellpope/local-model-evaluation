---
name: omp-qwen-3.8-flash-run2
created: 2026-09-02
model: qwen3.8-flash — Alibaba **hosted API**, not local weights. Served via **omp/18.1.2** provider `alibaba-token-plan`; serving precision **undisclosed**. Effort **recorded** as `thinkingLevel: xhigh`, `configured: xhigh` — explicitly set in-session, but **NOT wire-verified**; a config defect means the level may never have reached the API (see Wire). Session `01a060bf-872d-7647-a2c5-83cd107ac08e`, cwd `omp-qwen-3.8-flash-run2`, prompt *"read govmomi-cli-eval-prompt.md and execute"*, started 2026-09-01 23:12:39 PDT. **264,775 reasoning tokens / 361,955 output across 272 assistant turns**, 2026-09-01 23:12:39 → 2026-09-02 01:25 PDT (~2h12m), final `stopReason: stop`.
stage: audited
score: 22 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — omp-qwen-3.8-flash-run2

## Wire

**Audit-only record.** Generation driven by the operator. This run is a **controlled repeat of
`omp-qwen-3.8-flash` with effort explicitly set to `xhigh`**, to resolve the confound that made
the first omp↔opencode comparison undrawable.

### Effort as *recorded* — NOT wire-verified

> **Retraction.** An earlier version of this section claimed the effort level was "verified
> before dispatch". That claim was withdrawn the same day and is not supported. What follows is
> what omp *recorded*, which is config/UI state — not evidence of what the API request carried.
> See "The config issue" below.

The first omp run was scored at `thinkingLevel: null` against an opencode row at
`variant: xhigh`, so the harness comparison it was designed to make could not be drawn. Here
the level was set explicitly before the prompt landed:

| timestamp (UTC) | record | value |
|---|---|---|
| 06:12:39.879 | `model_change` | `zai/glm-5.3` (carried over from a prior session) |
| 06:12:46.885 | `model_change` | `alibaba-token-plan/qwen3.8-flash` |
| 06:12:46.886 | `thinking_level_change` | `thinkingLevel: medium`, `configured: auto` |
| 06:12:50.696 | `thinking_level_change` | **`thinkingLevel: xhigh`, `configured: xhigh`** |
| after | first user message | *"read govmomi-cli-eval-prompt.md and execute"* |

`configured: "xhigh"` (not `null`, not `"auto"`) distinguishes an explicit setting from an
inherited default. **It does not establish what was sent on the wire.**

### The config issue — the session record is intent, not evidence

The operator reports a configuration defect in which **omp sends no thinking level to the API
at all, while the system prompt it constructs asserts the effort is `xhigh`**. Independent
checks from this side are consistent with that and cannot refute it:

- **No request-level thinking parameter appears in any omp log.** Greps for `thinking`,
  `thinkingLevel`, `reasoning`, `effort`, `reasoningEffort` and `thinking_budget` across the
  run's logs return nothing. omp does not log outgoing request parameters, so the logs can
  neither confirm nor deny the level was sent.
- The session `.jsonl` records `thinking_level_change` events and assistant `thinking` content
  blocks carrying `thinkingSignature: "reasoning_content"`, but the latter only shows the
  endpoint *returned* reasoning — every run in this eval does that, at any setting.
- `~/.omp/agent/config.yml` at the time of this run: `modelRoles.default:
  alibaba-token-plan/qwen3.8-flash:xhigh`, `defaultThinkingLevel: auto`.

**Consequence: `thinking_level_change` is a record of operator intent, not of request content.**
That materially weakens the provenance correction made against
[`omp-qwen-3.8-flash`](omp-qwen-3.8-flash.md). It remains true that omp *records* an effort
level where the handoff claimed it recorded none — but the recorded value cannot be treated as
a controlled variable until the wire path is fixed and confirmed. **Any harness adapter for v2
must verify effort at the request, not at the config.** A recorded label that never reaches the
API is worse than no label, because it looks like a control.

**Incidental finding: omp's default for this model is `auto` → `medium`, not `null`.** Selecting
flash produced `medium`/`auto` at 06:12:46. Run 1's `null` was therefore **not** omp's default
but a genuine unset state left by the `qwen3.8-max` → `qwen3.8-flash` switch, which reset the
level without re-resolving it. The most likely reading is that run 1 sent **no thinking
parameter at all**, leaving the endpoint's own default to apply. This is an omp state-machine
gotcha worth carrying into any v2 harness adapter: *changing the model silently clears the
thinking level, and the cleared state is not the same as the default state.*

### Pre-registered prediction (recorded before the result)

Run 1 spent **2.8× the reasoning tokens of the `xhigh` opencode run while carrying no effort
label at all** — 244,164 vs 88,467, and 1,021 vs 336 reasoning tokens *per turn*. Because the
per-turn figure differs 3×, the gap looks like a per-request property rather than run-to-run
variance. This run discriminates two explanations:

- **H1 — harness parameter mapping.** If run 2 at explicit `xhigh` lands near **~244k**
  reasoning tokens, then `null ≈ xhigh` at this endpoint, run 1 was never actually at low
  effort, and the variable separating omp from opencode is how each harness maps an effort
  label onto the request — not the effort the operator asked for.
- **H2 — run-to-run variance.** If run 2 lands near **~88k** (the opencode figure), run 1's
  spend was anomalous and variance on this task is large enough to swamp a 2-point score
  difference — which would also retire the omp↔opencode score comparison entirely.

Discriminating observation: total reasoning tokens and reasoning-per-assistant-turn, from
`/message/usage` in the session `.jsonl`.

**Amended after the config issue surfaced — the wire state of run 2 is unknown and under
diagnosis, so H1/H2 above are not cleanly separable by token count alone.** What the count can
and cannot settle:

| observation | what it supports | what it cannot distinguish |
|---|---|---|
| R2 ≈ 244k (≈ run 1) | Under omp, the *recorded* effort label has **no measurable effect on spend**. The omp↔opencode score gap is not effort-explained, and the run-1 comparison caveat can be lifted on those grounds. | Whether the label is **never sent** (config defect) or **sent and ignored**. Only request inspection separates these. |
| R2 ≈ 88k (≈ opencode) | `xhigh` did take effect *and lowered* spend relative to run 1's unset state — implying the endpoint's no-parameter default is *higher* than explicit `xhigh`. | Would need a third point to rule out plain variance. |
| R2 far from both | Run-to-run variance on this task is large enough to swamp the effect being measured. | Everything else. |

**The distinction between "not sent" and "sent but inert" is not recoverable from token counts
and must not be inferred from them.** It requires observing the outgoing request. Until that
exists, this record asserts only the weaker claim the numbers support.

### Concurrency caveat

An opencode run was dispatched **concurrently** with this one, against the same provider
account. Token accounting is unaffected — counts are per-response and provider-reported — so
reasoning/output comparisons stand. **Wall-clock, TTFT and throughput comparisons against
`omp-qwen-3.8-flash` (run serially) are confounded** by shared rate limits and local resource
contention, and are not drawn.

**Note on what this run does *not* answer.** Matched-effort repetition cannot also measure
variance. A third run at run 1's settings would be needed for that; until one exists, any
run-to-run variance claim in this eval is unsupported.

Submission status at registration: seeded only (`go.mod`, eval prompt) — run in flight.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context (no fresh-context subagent
dispatch, per the operator's standing instruction — same method as every run in this set). Raw
report: [`omp-qwen-3.8-flash-run2/REVIEW.md`](../../../../omp-qwen-3.8-flash-run2/REVIEW.md).

**PASS WITH CONCERNS — 1 Critical (accuracy), 1 High, 2 Medium, 2 Low.**

Submission: 11 Go files, module `github.com/example/govc-inventory`, govmomi pinned to an
**untagged master pseudo-version** `v0.0.0-20260902050809-f0d835f384b5`. No cheat:
`tskip` gate clean, no tautology, no stub, no forged evidence. `gofmt`/`vet`/`staticcheck`/
`govulncheck` clean; `-race` clean; `make verify` green.

**This is the cleanest tree in the comparison set everywhere except criterion 4.** It is the
only one of the four with **no N+1** (`spine gate go n-plus-one` → no findings), the only one
besides `omp-qwen-3.8-flash` that **probes cancellation** (`errors.Is(err, context.Canceled)`),
and it ships the most thorough `verify.sh` of any submission — asserting config/env/flag
precedence and clean wrapped errors, not merely exit codes. Criterion 4 handling of LACP is
also the most careful in the set, returning `N/A` when `LacpCapability.LacpSupported` is
absent rather than assuming.

### C-1 (Critical, accuracy) — `naa.` → FC, a **third** independent occurrence

`vsphere.go:343` maps `naa.`/`fc.` prefixes to FC; `transport_test.go:16,17,75` assert it as
the expected value. Verified by differential probe against the production path — 3 of 4 wrong:

| owning adapter | correct | reported |
|---|---|---|
| `HostFibreChannelHba` | FC | `FC` |
| `HostInternetScsiHba` | iSCSI | **`FC`** |
| `HostParallelScsiHba` | unknown | **`FC`** |
| `HostBlockHba` | unknown | **`FC`** |

Charged accuracy, not integrity, per the `deepseek-v4-flash-0731` precedent. **Overrulable —
this call is the difference between 22 and FAIL.**

### H-1 (High) — the correct signal is dead code

`deviceTransport` orders LUN table → NVMe table → **adapter token → HBA type** → name
fallback. The adapter step is the only structurally sound signal, and it never runs:
`indexHostStorage` inserts every LUN's canonical name into `lunTransport` *including* the value
`"unknown"`, and the step-1 guard is `ok && cls != ""` — `"unknown"` is non-empty, so it
returns and the adapter step is shadowed.

Differential probe, same iSCSI HBA and same `mpx.vmhba33:C0:T0:L0` LUN:

| host state | result |
|---|---|
| LUN present in `ScsiLun` (a real host) | **`unknown`** |
| LUN absent (adapter path reachable) | **`iSCSI`** |

Fixing C-1 alone would not help; the correct path would still never execute.

### Other findings

- **Medium** — `pflag` imported directly, outside the allowed dependency set (criterion 8
  partial). Mitigating: it is cobra's own flag library.
- **Medium** — sim datastore assertion accepts membership including `unknown`; the structural
  reason C-1 and H-1 shipped green.
- **Low** ×2 — gosec G115 ×2 on port counts (triaged not real); govmomi pinned to an untagged
  pseudo-version rather than a release tag.

### Not run

**The behavioural mutation battery was not run.** Disclosed rather than omitted; `battery_*`
front matter left empty, consistent with every run in this comparison set. No git history in
the run directory, so test-churn forensics could not be performed.

## Score

**22 / 30 — PASS WITH CONCERNS.** 1 Critical (accuracy), 1 High, 2 Medium, 2 Low.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 2 | 7 of 8 criteria met. Criterion 4 **unmet** — transport fabricated *and* the correct path unreachable. Criterion 8 partial (`pflag`). |
| Integrity | 3 | No cheat, skip, stub or forged log. Deductions: the unit test **certifies** `naa.`→FC as expected, and the sim test accepts `unknown` in the legal set. |
| Security | 4 | `insecure` default false; timeout plumbed; view destroyed. staticcheck + govulncheck clean; gosec 2 × G115 benign. |
| Performance | 4 | **No N+1** — the only tree in the set that is clean here. One ContainerView + one Retrieve with explicit fields, view destroyed. |
| Concurrency | 5 | `-race` clean; zero goroutines; cancellation probed with `errors.Is`. |
| Quality | 4 | gofmt/vet/staticcheck clean; the most thorough verify script in the set. Costs: the shadowed dead path, `pflag`, root package 0%. |

## Compare

### The pre-registered prediction resolves: R2 ≈ R1

| run | harness | effort (recorded) | reasoning | output | turns |
|---|---|---|--:|--:|--:|
| `omp-qwen-3.8-flash` | omp | `null` (unset) | 244,164 | 332,988 | 239 |
| **`omp-qwen-3.8-flash-run2`** | omp | **`xhigh`** | **264,775** | **361,955** | **272** |
| `qwen-3.8-flash` | opencode | `xhigh` | 88,467 | 75,374 | 263 |
| `opencode-qwen3.8-flash` | opencode | `xhigh` | 72,292 | 49,983 | 179 |

**Run 2 landed at 264,775 — 8.4% from run 1, and 3–3.7× above both opencode runs.** That is the
`R2 ≈ 244k` row of the amended prediction table. Per that pre-registration, the supported
conclusion is the weaker one:

> **Under omp, the recorded effort label has no measurable effect on reasoning spend.**

The two omp runs differ by 1.08× across a *recorded* `null` → `xhigh` change, while the two
opencode runs at matched `xhigh` differ by 1.22×. **The omp effort change moved spend less than
opencode's own run-to-run noise.** Given the operator's config finding, the most economical
reading is that no thinking level reached the API in either omp run — but **token counts cannot
distinguish "never sent" from "sent and ignored"**, and this record does not claim to. What is
ruled out is "sent and effective".

**The harness difference itself is real and survives.** The omp/opencode reasoning gap (3–3.7×)
is an order of magnitude larger than either harness's internal spread, so it is not variance.
Something structural differs — most likely the request omp constructs — and it is the one
cross-harness effect in this whole sequence whose size exceeds the measured noise.

### The score comparison remains dead

| run | harness | score |
|---|---|--:|
| `qwen-3.8-flash` | opencode | 24 |
| `omp-qwen-3.8-flash` | omp | 22 |
| **`omp-qwen-3.8-flash-run2`** | **omp** | **22** |
| `opencode-qwen3.8-flash` | opencode | 18 |

Same model, same task, four runs: **18, 22, 22, 24**. The two omp runs reproduce at 22; the two
opencode runs, at matched effort one day apart, span **18–24**. **opencode's own variance (6
points) exceeds the omp↔opencode difference entirely**, so no harness ordering can be claimed
from score. Only the token result separates them.

### Criterion 4 is now the confirmed failure mode of this eval

Across the four runs, criterion 4 fails four different ways and drives essentially the whole
spread:

| run | criterion-4 defect | degrades to |
|---|---|---|
| `qwen-3.8-flash` | HBA key vs device name | `unknown` (legal) |
| `omp-qwen-3.8-flash` | same, plus zero-value guard; multipath clobbers topology | **`""`** (illegal) |
| `opencode-qwen3.8-flash` | `naa.` → FC | **`FC`** (fabricated) |
| `omp-qwen-3.8-flash-run2` | `naa.` → FC, plus correct path shadowed | **`FC`** (fabricated) |

**Four of four runs got criterion 4 wrong, and three of the last five landed on the identical
`naa.` → FC fabrication** — across two harnesses and two model families (`deepseek-v4-flash`
and `qwen3.8-flash` twice). That is no longer a model property; it is a property of the task.
The identifier-space trap is what this rubric actually measures, and it is the strongest
ladderbench-v2 input the whole pilot sequence produced.

**For v2:** criterion 4 needs either a fixture that makes the transport path reachable — vcsim
populates no `ScsiTopology`, no `MultipathInfo` and no `naa.` extents, so every submission's
suite passes green while the feature is broken — or removal as a scored criterion. As it
stands it contributes 2–3 points of Accuracy plus 1 of Integrity on a signal no submission's
tests can see.

## Remediate

## Rescore
