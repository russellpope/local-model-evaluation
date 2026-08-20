---
name: kat-coder-v2.5-dev-bf16
created: 2026-08-13
model: Kwaipilot KAT-Coder-V2.5-Dev (Apache 2.0 — hybrid attention/SSM MoE, arch `qwen35moe`; 40 blocks, `full_attention_interval = 4` → 10 full-attention + 30 SSM layers; 256 experts, 8 used; 262,144 native context. Fine-tune of **Qwen3.6 35B A3B**. Run at **BF16, native precision** from `bartowski/Kwaipilot_KAT-Coder-V2.5-Dev-GGUF` (2 shards, 69,376,637,248 B = 64.6 GiB). Local on Apple M5 Max 128 GiB via Homebrew llama.cpp `llama-server`; driven via opencode)
stage: audited
score: 14 / 30
---

# Run — kat-coder-v2.5-dev-bf16

## Wire

**Status: audited 2026-08-14 — FAIL 14/30.** Submission frozen at `08962c5` before the audit.
Directory `kat-coder-v2.5-dev-bf16/` was seeded with the eval prompt only; the audit prompt was
deliberately **not** present and went in at audit time, never before. The model ignored that
directory and wrote to the repo root as `govmomi-inventory/` — the prompt never specifies a working
directory, so this is recorded as a deviation and **not scored** (see requirements defect R6).
Everything above *Wired* is pre-registration, left as written before anything ran.

### Why this run exists

KAT-Coder V2.5 Dev declares `general.base_model.0.name = Qwen3.6 35B A3B` in its own GGUF
metadata. That makes it the **third independent fine-tune of one base model** in this field, and the
first time the field can hold a base constant and vary only the fine-tune:

| Run | Relationship to Qwen3.6 35B A3B | Baseline | Final |
|---|---|---|---|
| [`qwen3.6-35b-a3b-ud-mxfp8_k_xl-mlx`](qwen3.6-35b-a3b-ud-mxfp8_k_xl-mlx.md) | the base itself | 15 | 21 |
| [`qwen-agentworld-35b-a3b`](qwen-agentworld-35b-a3b.md) | AgentWorld agentic fine-tune | 16 | 23 |
| **`kat-coder-v2.5-dev-bf16`** | Kwaipilot coding/agentic fine-tune | — | — |

That comparison is worth more than the headline number. The base scored 15 and one agentic
fine-tune moved it to 16 baseline / 23 remediated. If a second, independently-produced agentic
fine-tune lands in the same band, the ceiling belongs to the **base**, not to any lab's post-training.
If KAT-Coder breaks out, post-training is the lever. Either result is informative, and neither is
available from a benchmark table.

One confound is *removed* relative to the base run: the base was scored from an `mxfp8_k_xl` MLX
build, so its 15 carries a quantization asterisk. This run is **BF16 weights with an F16 KV cache —
no quantization anywhere in the stack**, the second such run in the field after
[`muse-glimmer-30b-bf16`](muse-glimmer-30b-bf16.md). One confound *remains and is disclosed*: the
producer differs (bartowski here, unsloth for Muse), and AgentWorld was served through LM Studio.

### Backend decision — llama.cpp, not LM Studio (decided before the run)

The kickoff plan named LM Studio. It was changed to `llama-server` **before any prompt was
delivered**, on the operator's approval, and it cost nothing: llama.cpp reads the GGUF already on
disk at `~/.lmstudio/models/bartowski/…` directly, auto-loading shard 2 from `split.count = 2`.
**Nothing was re-downloaded.**

The reason is instrumentation, and it is not recoverable after the fact. LM Studio reports reasoning
as `usage.completion_tokens_details.reasoning_tokens` — a *count*. llama-server returns
`reasoning_content` — *text*, which opencode persists. The `laguna-s-2.1` baseline, run on LM Studio,
stored **one reasoning block of 122 characters across 232 tool calls**. Gate 5 below returned **4,530
characters of `reasoning_content` from a single request**. The reasoning-trace analysis in
[`artifacts/cross-run-reasoning-failure-modes.md`](../artifacts/cross-run-reasoning-failure-modes.md)
— the most interesting output of the previous run — is only possible on this backend.

### Wired — measured 2026-08-13, all gates green

Server: Homebrew `llama-server` **build 10360 (48d22e295)** — the same upstream build that produced
every `muse-glimmer-30b-bf16` number, and *not* the `poolsideai/llama.cpp` fork at build 10010 that
the laguna runs came from.

```
--jinja -fa on -ngl 999 -np 1 -c 262144 -b 4096 -ub 2048
--temp 1.0 --top-p 0.95 --top-k 20 --log-timestamps
```

Live-loaded metadata: `n_ctx_train = 262144`, `n_expert = 256`, `n_expert_used = 8`,
`n_head = 16`, `n_head_kv = 2` (8:1 GQA), `key_length = value_length = 256`,
`rope.freq_base = 1e7`, SSM `state_size = 128` / `conv_kernel = 4` / `inner_size = 4096`.
Tokenizer `gpt2`/`qwen35`, EOS `248046`, `add_bos_token = false`. 733 tensors across 2 shards.

**Context is 262,144 — the native trained length, with no rope scaling and no YaRN.** Memory is why
that is affordable: only 10 of 40 layers hold a KV cache, so F16 KV at *full* context computes to
5.0 GiB (10 layers × 2 KV heads × 512 × 2 B × 262,144). Measured process RSS is **69.36 GiB** against
a 64.61 GiB weight file — a residual consistent with that figure. *Disclosure:* build 10360 did not
emit its own KV-buffer breakdown at this verbosity, so the split is computed and cross-checked
against RSS, not read from the loader.

**Sampler is the model's own baked-in GGUF metadata** — `general.sampling.temp = 1.0`,
`top_p = 0.95`, `top_k = 20`. This differs from the three sibling Qwen3.6-A3B entries in the field,
which run `temp 0.6`. That is not an inconsistency: every run in this field uses its own vendor's
documented sampler, and Kwaipilot ships 1.0. Recorded because it is a real comparability caveat.

**Deviation, recorded not corrected:** llama-server applies a default `min_p = 0.05` on top of the
vendor sampler — the same deviation disclosed for Muse, and unchanged here.

**`--reasoning-preserve` deliberately OFF**, though the server suggests it on load
(`chat template supports preserving reasoning`). It changes what is fed *back* to the model, not what
is captured — opencode persists each turn's `reasoning_content` either way — so it buys no trace
fidelity. What it *does* change was measured rather than assumed, by rendering a realistic agentic
history through llama-server's own `/apply-template` both ways
(`scratchpad/preserve-probe.py`):

| Reasoning block | preserve OFF | preserve ON |
|---|---|---|
| prior completed user turn | **DROPPED** | KEPT |
| current turn, before first tool call | **KEPT** | KEPT |
| current turn, after a tool result | **KEPT** | KEPT |

The template keys off the last *real* user message, and tool responses render as pseudo-user turns
that do not reset it. So reasoning chains forward intact across an arbitrarily long tool-call
sequence **with the flag off** — the failure mode the flag appears to guard against does not exist
for this workload, which is one large prompt plus a self-verification loop. The delta is exactly one
block per completed user turn.

Off therefore matches the template's own default (and so the post-training distribution), costs
nothing in within-task continuity, and avoids accumulating every prior turn's reasoning in context
over a multi-hour run.

**There is a real cost to off, and it is prompt-cache stability, not quality**
(`scratchpad/prefix-stability-probe.py`). History is not append-only with the flag off: a new user
message moves `last_query_index`, which strips the previous turn's thinking out of the *middle* of
the prompt. Rendering the same history before and after one follow-up:

| | turn N | after one follow-up | server KV cache |
|---|---|---|---|
| preserve OFF | 495 tok | **85 tok**, only 10.3% of the body prefix survives | **invalidated — full reprocess** |
| preserve ON | 495 tok | 515 tok, append-only | **survives** |

So off costs one full context reprocess per operator follow-up message — roughly 3 min on a
100k-token conversation at the 550–620 t/s prompt eval measured here. Accepted deliberately: this
eval is designed as one large prompt plus an autonomous self-verification loop (Muse: ~4 h 20 m
active, few interventions), and preserve ON would retain every completed turn's entire tool-chain
reasoning, a compounding context cost that slows every later prompt eval. **Revisit before the
prompt is delivered if the run is expected to be heavily babysat** — after that, flipping it is a
mid-run variable change and invalidates the gates. The `laguna-s-2.1-hf-Q4_K_M` run is the
cautionary case for flipping it casually — five variables moved at once, that flag among them, and
it proved nothing.

#### Gates

| Gate | Result |
|---|---|
| 1 — arch loads **live** | PASS. `qwen35moe` served under alias `kat-coder-v2.5-dev-bf16`. Presence of the arch string in `libllama.0.0.10360.dylib` was treated as necessary, not sufficient; the model was loaded. |
| 2 — `--jinja` / template accepted | PASS. Completion returned, no `custom template is not supported`. |
| 3 — **real two-turn tool round-trip** | PASS. Turn 1 emitted a `tool_call`; turn 2 consumed a fed tool result and answered coherently, `finish_reason=stop`. |
| 4 — effective context | PASS. `total_slots = 1`, `n_ctx` per slot `262144` — with `-np 1`, `-c` *is* the per-request context. |
| 5 — thinking on | PASS. **4,530 chars `reasoning_content`**; `reasoning_tokens: 0` is the llama-server accounting artifact `a0547c4` exists for. |

Gate 3 was the live risk and it is worth recording why. This template emits tool calls as **XML**
— `<tool_call><function=name><parameter=x>value</parameter></function></tool_call>` — not JSON. A
`/props` read shows `"chat_format": "Content-only"`, which reads like a parser fallback that would
silently strip every tool call and kill the agentic loop. It is not: that field reports the
*no-tools* default generation settings. With `tools` actually supplied, build 10360 routes the
template through its `autoparser` (`analyze_tool_call_format_non_json`, the same machinery behind the
Kimi K2 and Qwen3-Coder parsers) and returns proper OpenAI `tool_calls`. **A `/props` read alone
would have produced a false abort here** — only the two-turn round-trip settles it.

Thinking is *template-forced*: the generation prompt ends `<|im_start|>assistant\n<think>\n`, so
generation begins inside the reasoning block and the model emits the closing `</think>`. It cannot be
disabled except via `enable_thinking=false` through `--chat-template-kwargs`, which is not passed.

Harness: opencode provider `llamacpp-local` → `http://localhost:1234/v1`, model id
`kat-coder-v2.5-dev-bf16` (matches `--alias`), limits `context 262144 / output 65536` — matched to
the server's `-c`, not inherited. Added under the existing llama.cpp provider rather than the
`lmstudio` one, whose laguna-shaped defaults (262144 context, temp 0.6 / top_k 20) would have
silently misconfigured the sampler.

### Throughput — measured, NOT pre-registered

**Recorded honestly against the auditor:** the gate requests emitted `print_timing` lines, so
shallow-context throughput was measured *before* any prediction was registered. It is reported as
measured. No prediction is retro-fitted to it.

Across four gate requests at near-zero context depth: **decode 58.8 – 64.8 t/s**, prompt eval
**281 – 622 t/s**. That is ~7× Muse's measured 8–9 t/s decode, and the explanation is sparsity, not
quality: Muse is dense and reads all 55.7 GB per token, while this model activates 8 of 256 experts
(~3 B active) and reads a small fraction of its 64.6 GiB.

### Pre-registered predictions (recorded now, before the eval prompt is delivered)

Falsifiable, and none of them measured yet:

1. **Deep-context decode ≥ 40 t/s sustained at ≥ 100k context depth.** Shallow decode is a weak
   predictor; the hybrid SSM layers carry constant-size state while only 10 layers grow a KV cache,
   so the usual quadratic attention decay should be muted. Falsified by < 40 t/s.
2. **Reasoning capture ≥ 100× the laguna baseline** — ≥ 12,200 chars of persisted `reasoning_content`
   over the run, against laguna's 122 chars / 232 tool calls. This tests the backend decision itself.
3. **Baseline score ≥ 16** — i.e. at least matching `qwen-agentworld-35b-a3b`, the stronger of the two
   sibling fine-tunes. A coding-specialised fine-tune of the same base, unquantized, scoring *below*
   its own base's 15 would be the surprising result.
4. **The dominant failure mode will be fabricated-mechanism, not broken-build.** Every Qwen3.6-A3B
   descendant in this field produced a binary that compiles and output that looks right over logic
   that is not there. Falsified by a submission that fails `go build`.

### Cull thresholds — carried forward unchanged, judged on baseline

Pre-registered before the Muse run and still unmet by any local model. Reproduced here so this run
is judged against a bar set before it, not after.

| Threshold | Meaning |
|---|---|
| baseline > 18 | best local baseline ever recorded here — earns its slot |
| baseline ≥ 22 | retire gemma-4-31b and the remaining qwens |
| baseline ≥ 25 | retire qwen-agentworld and ornith-1.0-35b |

Field baselines: laguna-s-2.1 **18** (best local), qwen-3.6-27b 16, qwen-agentworld 16,
ornith-1.0-35b 16, qwen3.6-35b-mlx 15, muse-glimmer-30b-bf16 14, qwen3-coder-next 13.
Ornith-1.0-397B (22 → 28) is cloud-hosted and is not a local-disk decision.

## Audit

**Audited 2026-08-14 — FAIL 14/30.** Submission frozen at `08962c5` **before** the audit began, extracted
via `git archive $(git write-tree)` and built from *that*, not the working tree.

Reports: [`REVIEW-baseline`](../artifacts/kat-coder-v2.5-dev-bf16-REVIEW-baseline.md) ·
[`CLAIMS-REVIEW`](../artifacts/kat-coder-v2.5-dev-bf16-CLAIMS-REVIEW.md) ·
[`GROUND-TRUTH-0.50.0`](../artifacts/kat-coder-v2.5-dev-bf16-GROUND-TRUTH-0.50.0.md) ·
[`MUTATION-BATTERY`](../artifacts/kat-coder-v2.5-dev-bf16-MUTATION-BATTERY.md)

Four independent passes: a blind adversarial auditor (walled off from the scratchpad, all run records,
every `REVIEW*`/`FINDINGS*`/`EVIDENCE*` file and every other model's submission), a claims reviewer
working from the opencode transcript, a 38-probe mutation battery with negative controls at both ends,
and a ground-truth pass at the submission's pinned govmomi version.

### Ground truth first — and it cleared the version suspicion entirely

The submission pins **govmomi v0.50.0**, a third version in this field (Muse v0.52.0, everything earlier
v0.55.1). Probed identically against both v0.50.0 and v0.55.1 from a shared source: **exactly one
difference across the entire surface** — VM `storage.perDatastoreUsage.committed` is `0` vs `234` bytes,
and **both render `0.0 GiB`**. Same-version rerun controls came back identical, so the delta is real and
not noise. The pin was also the model's own choice at **23:20:22, 36 seconds into the run**, with zero
`go mod`/`go get` activity after the compaction — not a post-compaction artifact. **There is nothing to
charge on the version.**

Ground truth then prevented a repeat of the Muse near-miss, and more. These outputs look wrong and are
**correct**: `RAM 0.0 GB` (`memoryMB = 32`), `STORAGE 0.0 GiB`, `VCPU 1`, datastore `TYPE unknown`
(`summary.type` genuinely `"OTHER"` — `VMFS` would be the fabrication), **DVS `PORTS 0`** (`NumPorts`
really is 0; a *non-zero* DVS port count is the fabrication signature), `LACP disabled` on DVS, `LACP N/A`
on standard vSwitches, and the transport classifier's `unknown` (no FC/iSCSI/NVMe HBAs exist in vcsim).

### Cross-pass conflict, resolved rather than averaged

The blind auditor's **C6** bundled two hardcodes: `vlan := "0"` and vDS used-ports `0`. Ground truth
contradicts half of it. Resolution, recorded because the redundancy exists to surface exactly this:

- **The VLAN hardcode stands as Critical.** `DC0_DVPG0` is genuinely VLAN 0, so it matches *by accident*,
  but `DVS0-DVUplinks-8` is a **trunk, 0–4094**, which the prompt explicitly requires be shown as a
  range. The VLAN spec was fetched and then discarded. Demonstrably wrong output.
- **The used-ports `0` is downgraded and partly charged to the auditor.** `DVSConfigInfo` exposes
  `NumPorts` with no available-ports counterpart, so `used = total − available` is **not derivable for a
  vDS**; ground truth independently could not determine whether any such figure is obtainable. Charging
  `0` as fabrication when `0` is also the true value, against a DoD that cannot be satisfied honestly, is
  the **auditor-induced fabrication** pattern already recorded against `qwen3.6-35b-a3b` (whose
  impossible "non-zero PORTS" DoD induced a fabricated 6144). Charged Medium as "should have reported
  `N/A`", and the DoD defect charged to the instrument.

This does not move the total: C6 remains Critical on the VLAN alone.

### The dominant defect — the binary does not run

Reached independently by ground truth, the blind auditor, and the claims reviewer. All three subcommands
exit 1 against vcsim at **both** govmomi versions, via three stacked defects:

1. `cmd/root.go:88` — `getDatacenter` passes the **ContainerView's own MoRef** to `pc.Retrieve` asking for
   `name` into `[]mo.Datacenter` → `InvalidProperty`. This is the *identical* bug the model had just
   fixed in the three inventory files.
2. `cmd/root.go:205` + `config/config.go:44-49` — every config flag is *declared inside* `RunE`, after
   cobra has parsed argv, so `--url` is rejected as an unknown flag. Criterion 2 unmet, and `--config` is
   unreachable, killing the file tier of the precedence chain too.
3. A URL carrying userinfo double-logs-in (`NewClient` then `connectToVCenter`'s `Login`) →
   `ServerFaultCode: Login failure`.

**Negative control:** `govc ls /` against the same simulator with the same credentials returns `/DC0`,
proving server and credentials are fine; patching the three `getDatacenter` lines to `v.View` makes all
three subcommands render correct tables. One line stood between "never runs" and "runs".

**vcsim was never started — zero occurrences of `vcsim` or `8989` across all 730 tool calls.** The
prompt's mandatory self-verification loop was never executed, and running it once would have found this.

### Mutation battery — 38 probes

`kill rate (scorable) 8/31 = 26%` · `kill rate (raw) 9/36 = 25%` · 0 NO-SITE · 2 BUILD-ERR (disclosed:
`FAB-dvs-ports`/`FAB-dvs-total`, the same fabricated-port-count intent defeated twice by Go's
unused-variable rule; the third attempt compiled and **SURVIVED**). Report-only rows: `SEC-insecure-def`
KILLED\*, `SEC-tls-forced`/`SEC-cred-override`/`LIFE-logout`/`LIFE-view-destroy` SURVIVED\*. Negative
controls both landed as predicted — `OUT-units-fmt` (must-kill) KILLED, `OUT-hdr-ds` (must-survive)
SURVIVED. Full matrix in the linked report.

**27 survivors, 8 distinct causes, 19 of them from three:** `cmd/` has zero test files, so the entire CLI
layer is unobserved; **`TestListVMs`/`TestListDatastores` never call `ListVMs`/`ListDatastores`** — they
re-implement the retrieval inline and assert on their own copy; and `ListSwitches` is called but asserted
only for plausibility (set membership, non-negativity, `used <= total`), all satisfiable by a constant.
The suite is green and is not testing the production code.

### Honesty — the series pattern holds, with a real exception

Three privately-flagged shortcuts, none disclosed: DVS VLAN (*"just show \"0\" if we can't parse it"*,
23:54); `t.Skip` (*"The prompt says \"no t.Skip\" but I can write tests that work with the simulator's
actual capabilities"*, 00:44:37 — then ships `t.Skip` at `vms_test.go:180` and reports "All tests pass");
and the port-group type assertions it privately called *"interface type assertions that won't work
cleanly"* before shipping them. It also ran `cat Makefile` → "No such file or directory" and said nothing.

**But it fabricated no sample output** — the pasted `go test` block is byte-exact against the real
07:24:18 run — and it did **not** claim credit for compaction-erased work; its final message is correctly
scoped to what it could still see. Across seven models that is a first. Its failure mode is *silence
about known gaps*, not invented evidence.

### A wire-level finding: tool-call format collapse at peak context

At **00:45:23, at 229,841 tokens — the deepest context of the run (87.7% of the window)** — the model
emitted **5,887 characters of Claude-dialect tool XML** (`<tool_calls><invoke name="write">
<parameter name="filePath">`) into the **text channel**, instead of the
`<tool_call><function=…><parameter=…>` format its own chat template declares. Those writes never
executed. The payload contained **`Makefile`, `vswitches_test.go`, and `datastores_test.go` — all three
absent from the submission**, which is the mechanical origin of the missing deliverables *and* of the
untested state of the two largest modules.

It happened **once in ~731 tool calls**, and llama.cpp was correct not to parse a dialect the template
never declares — this is a model defect (training contamination), not a harness one. Gate 3 could not
have caught it: a two-turn round-trip at zero depth exercises nothing about format integrity at 230k.
**Recorded as a new wire risk for every future long-context run in this field.**

## Score

**FAIL — 14 / 30.** Accuracy 1 · Integrity 1 · Security 3 · Performance 3 · Concurrency 4 · Quality 2.

7 Critical, 4 High, 6 Medium, 4 Low. The Criticals: non-functional `getDatacenter` (C1); config flags
declared after argv parsing so `--url` is rejected (C2); `--portgroup` returning 0 VMs against a ground
truth of 8, for standard *and* distributed, via three stacked bugs (C3); datastore TYPE keyed off
`summary.Type` — the filesystem type the prompt explicitly says is *not* the answer — with the real
classifier `ClassifyTransportFromDevice` sitting as dead code with zero callers (C4); six mutants
including `ListVMs`→nil passing the full suite (C5); the vDS VLAN hardcode (C6, resolved above); and
vcsim's `user`/`pass` hardcoded as a silent fallback in the production connection path (C7).

**Genuinely good, and worth not losing:** criterion 3 is correctly met — committed, not provisioned,
storage read from `storage.perDatastoreUsage[].Committed`. The retrieval pattern is textbook single
`ContainerView` + explicit property list at constant round-trips. `-race` clean, all views destroyed,
logout deferred (Concurrency 4, the highest dimension score). TLS-verify and `--timeout` are genuinely
enforced. And the distributed `--portgroup` path is otherwise correct — fixing the one type assertion
(`BaseVirtualEthernetCard`, not `*types.VirtualEthernetCard`) returns the right 8 VMs.

**The pattern is the same one this field keeps finding, in a new place.** Muse's output looked right over
logic that wasn't there. Here the *logic is largely there* — the retrieval pattern is the best in the
local cohort — and it is never connected to a working entry point. The model built good components and
never ran the program once.

### Run conditions — the score reflects ~1h28m of work

Recorded because it bounds the interpretation, **not** to adjust the score, which measures the artifact:

| | |
|---|---|
| Wall clock | 8h42m (23:30 → 08:12) |
| **Active** | **1h28m** |
| Active before the 01:01 compaction | 79 min |
| **Blocked on reauthorization** | **6h09m** (01:05:23 → 07:14:29) |
| Active after reauthorization | 11 min |

The compaction erased the model's memory of already holding authorization; it re-requested access and
the prompt sat unanswered overnight. **Charged entirely to the operator** — the second consecutive run
lost this way (2h22m on Muse, 6h09m here). "Allow always" is scoped only until OpenCode restarts, which
is why it recurs; a durable permission allowlist is the fix, and it belongs before the next wire, not
inside an audit.

### Pre-registered predictions — judged

| # | Prediction | Result |
|---|---|---|
| 1 | deep-context decode ≥ 40 t/s at ≥ 100k | **FALSIFIED** — 32–34 t/s at ~100k; median 31.4 across the run, min 19.8 |
| 2 | reasoning capture ≥ 12,200 chars | **CONFIRMED, 4.5×** — 54,727 chars / 174 blocks / 730 tool calls |
| 3 | baseline ≥ 16 | **FALSIFIED** — 14 |
| 4 | failure mode fabricated-mechanism, not broken-build | **Confirmed on its literal terms** (`go build` passes) but the framing missed the dominant defect: a building binary that does not *run* |

Prediction 1's *reasoning* was wrong, not just its number: I argued the 30 SSM layers carrying
constant-size state would mute attention decay. Decode fell 58.8–64.8 t/s at zero depth to ~34 at 100k, a
~45% drop — the 10 full-attention layers dominate. Recorded as a fact about hybrid-SSM serving, not a
missed guess.

Prediction 2 is the one that mattered: **every honesty finding in this record exists because llama.cpp
returns reasoning as text.** On LM Studio there would have been a token count and nothing to read.

### Requirements defects — charged to the auditor, not the model

- **R3, substantive.** The prompt's `go run github.com/vmware/govmomi/vcsim` **does not work at v0.50.0**:
  vcsim is a separate nested module with `replace` directives and no published tags, so both `go run` and
  `go install …@ver` fail. The prompt's claim that it "ships inside the govmomi module you already depend
  on … no extra dependency" is **false** for that pin. Proposed resolution: pin `@latest` or point at the
  embedded `simulator` package. *It does not excuse this submission* — the in-process simulator the model
  already imports reproduces C1 immediately.
- **R2.** The prompt pins neither library nor simulator version, which is why this field now spans three
  govmomi versions and why ground truth must be rebuilt every run. **Must not be fixed mid-field** —
  editing the eval prompt breaks comparability with all eleven scored runs. Logged as a v2-instrument change.
- **R5.** `used = total − available` is not derivable for a vDS (see the conflict resolution above).
- **R4.** "only govmomi/cobra/viper/stdlib" is unsatisfiable as written — cobra's own API returns
  `*pflag.FlagSet`. Not charged.
- **R1.** RAM "GB" vs "GiB/TiB" contradiction, and "human-readable (GiB/TiB)" vs "consistent units" are
  opposite demands. Scored against the adaptive-suffix reading, charged Medium not Critical.
- **R6.** The prompt never specifies a **working directory**; the staged-workspace expectation lived only
  in the harness cwd. The model wrote to the repo root instead. **Deviation noted, not scored.**

## Compare

**14 / 30 — tied with `muse-glimmer-30b-bf16` for the lowest baseline of any unquantized run, and below
its own base model's remediated arc.**

| Run | Relationship to Qwen3.6 35B A3B | Baseline | Final |
|---|---|---|---|
| [`qwen3.6-35b-a3b-...-mlx`](qwen3.6-35b-a3b-ud-mxfp8_k_xl-mlx.md) | the base itself (mxfp8) | 15 | 21 |
| [`qwen-agentworld-35b-a3b`](qwen-agentworld-35b-a3b.md) | AgentWorld agentic fine-tune | 16 | 23 |
| **`kat-coder-v2.5-dev-bf16`** | Kwaipilot coding fine-tune (BF16) | **14** | — |

**The natural experiment this run existed for returns a clear answer, and it is not the flattering one.**
Holding the base constant and varying only post-training: the base scores 15, an agentic fine-tune 16,
and a coding-specialised fine-tune **14** — at *higher* precision than either sibling, with the
quantization asterisk removed. All three land in a 3-point band. **On this task the ceiling belongs to the
base, not to any lab's fine-tune.** Kwaipilot's coding specialisation bought nothing measurable here, and
`qwen-agentworld`'s remediated 23 remains the family's high-water mark.

That said, the comparison carries a caveat the siblings do not: **this run got ~1h28m of compute** against
Muse's ~4h20m, and it lost `Makefile`, `vswitches_test.go` and `datastores_test.go` to a tool-format
collapse. A rerun without the permission block is the honest way to test whether 14 is the model's ceiling
or the run's. It is *not* a remediation round — it is the same prompt under working conditions.

**Second unquantized data point, same conclusion as the first.** Muse was the field's first run with no
quantization anywhere and scored 14. This is the second, and also scores 14. Both sit below a 4-bit 118B
MoE at 18. Quantization continues not to be the explanation for anything.

**Pre-registered cull thresholds — none met.** baseline > 18 (earns its slot), ≥ 22, ≥ 25 all unmet at 14.
Field baselines unchanged: laguna-s-2.1 **18** (still the best local baseline), qwen-3.6-27b 16,
qwen-agentworld 16, ornith-1.0-35b 16, qwen3.6-35b-mlx 15, **muse-glimmer-30b-bf16 14**,
**kat-coder-v2.5-dev-bf16 14**, qwen3-coder-next 13.

**What this run contributes beyond its score** is the tool-call format collapse at 229,841 tokens. That is
a failure mode no prior run in this field could have observed, because no prior run captured enough
reasoning and transcript detail to catch a single malformed emission among 731, and it cost three
deliverables. It is now a standing wire risk for long-context agentic runs.

## Remediate

## Rescore
