---
name: qwen3.8-27b-bf16
created: 2026-08-14
model: Qwen3.8-27B (Apache 2.0 — hybrid linear-attention/SSM **dense** model, arch `qwen35`; 64 blocks, `full_attention_interval = 4` → 16 full-attention + 48 linear/SSM layers; `head_count 24`, `head_count_kv 4`, `key/value_length 256`; vocab 248,320; 262,144 native context; natively multimodal `image-text-to-text`, run text-only. Run at **BF16, native precision** from `ggml-org/Qwen3.8-27B-GGUF` single file `Qwen3.8-27B-BF16.gguf`, 53,808,281,952 B = 50.1 GiB, 851 tensors. Local on Apple M5 Max 128 GiB via Homebrew llama.cpp `llama-server` build 10360; driven via opencode)
stage: wired
score:
---

# Run — qwen3.8-27b-bf16

## Wire

**Status: pre-registration.** Directory `qwen3.8-27b-bf16/` is seeded with the eval prompt only; the
audit prompt is deliberately **not** present and goes in at audit time, never before. Everything in
this section is written before the model has been loaded — no weights have been executed at the time
of writing, and no throughput number exists yet.

### Why this run exists

Qwen3.8-27B is the first **dense** hybrid-attention model in this field. Every prior Qwen3.x entry
has been either a sparse MoE (`qwen35moe` — qwen3.6-35b-a3b, qwen-agentworld, kat-coder) or a
conventional dense transformer (muse-glimmer, gemma-4-31b). This one is dense *and* hybrid: only 16
of 64 layers keep a growing KV cache, the other 48 carry constant-size SSM state.

That makes it the cleanest test yet of a question the field has repeatedly raised but never isolated:
**how much of the long-context degradation in these runs is attention cost rather than model
quality.** The task routinely reaches 200k+ tokens; KAT hit 229,841 and its tool-call format collapsed
there. A model whose KV grows at a quarter the usual rate should decay far less over the same run.

It also arrives with the two open questions from the KAT close still unanswered — whether 14 was that
model's ceiling or its run's, and whether the base or the post-training sets the ceiling on this task.

### Backend decision — llama.cpp, not LM Studio (carried forward, unchanged)

LM Studio reports reasoning as a token *count*; llama-server returns `reasoning_content` *text*, which
opencode persists. On KAT this captured 54,727 chars across 174 reasoning blocks against the LM
Studio-era laguna baseline of 122 chars — ~450× the density, and every honesty finding in that record
exists only because of it. Repeated here for the same reason. Provider entry is `llamacpp-local`,
never `lmstudio`, whose defaults are laguna-shaped (262,144 ctx, temp 0.6 / top_k 20) and would
silently misconfigure this model.

### Architecture support — resolved before download, not after

The prior handoff flagged an abort risk: build 10360 knows a fixed set of qwen arches and there is no
`qwen38`. Resolved by reading the GGUF header directly over an HTTP range request — 10.94 MB pulled,
not 53.8 GB — before committing to the download:

```
general.architecture = qwen35        ← present in build 10360's arch table
qwen35.block_count = 64              qwen35.context_length = 262144
qwen35.full_attention_interval = 4   qwen35.ssm.state_size = 128
```

Despite the "3.8" product name the model declares `model_type: qwen3_5`
(`Qwen3_5ForConditionalGeneration`), which llama.cpp maps to the existing `qwen35` arch. **No source
build required.** Recorded as necessary-but-not-sufficient: string presence in the arch table is not a
load, and Gate 1 below is the load.

Two adjacent risks checked and cleared in the same pass:

- **Upstream is 70 commits ahead** (brew 10360 vs b10430, released 2026-08-14). Nothing in that range
  touches the `qwen35` loader or the Metal backend. One commit is relevant only if Gate 3 misbehaves:
  `chat : tighten bare function parsing for Qwen models (#26793)`. Staying on 10360 keeps the harness
  identical to the twelve prior scored runs.
- **Open issue #26916** — a `qwen3_5` model failing to load with `tensor 'blk.32.attn_norm.weight'
  not found` — **does not apply.** That model declared `nextn_predict_layers = 1` while shipping no
  MTP tensors. All 39 KV pairs of this GGUF were dumped: no `nextn` key, 851 tensors, 64 blocks.
  Worth recording because the upstream workaround (`--no-mtp`) **does not exist in build 10360**; had
  it applied, the source build would have been mandatory after all.

### Sampler — the vendor's documented thinking-mode preset

`temperature 1.0, top_p 0.95, top_k 20, min_p 0.0`. Three independent sources agree: the model card's
Best Practices, `generation_config.json`, and the GGUF's own baked `general.sampling.*`. The field
rule is each model's own vendor sampler, which is why this run differs from the `temp 0.6` cohort
(laguna, agents-a1, orinth, the Qwen3.6 family) — the same recorded caveat as KAT, which ran 1.0
because Kwaipilot ships 1.0.

**One deviation retired.** llama-server applies a default `min_p = 0.05` on top of the vendor sampler;
this was disclosed as an uncorrected deviation on both Muse and KAT. Qwen documents `min_p = 0.0`
explicitly for thinking mode, so `--min-p 0.0` is passed and the footnote does not carry forward.

### Speculative decoding — deliberately OFF for this run

`mtp-Qwen3.8-27B-BF16.gguf` (5.95 GB) exists and build 10360 supports it via
`--spec-type draft-mtp`. Not downloaded, not used. The baseline must stay comparable to twelve runs
that had no draft model, and MTP carries a documented prompt-processing regression (D2H embedding
transfers, per the merging PR) that would be actively harmful on a prefill-heavy 200k-token run.
Pre-registered as a **separate throughput arm** after the baseline scores, not as a ladder variable.

DFlash is not an option here regardless: no Qwen3.8 drafter exists (z-lab ships 3.5-9B, 3.6-27B,
3.6-35B-A3B only), and two open bugs land on this platform — #26967 (corrupted `predicted_ms` on
Q4/Metal) and #26894 (drafter fails to bind on the Muse-Glimmer official GGUF, which is this repo's
own pre-registered DFlash arm).

### Throughput — no measurement exists yet

Recorded explicitly: at the time of writing, the model has not been loaded and **no throughput number
of any kind has been observed.** The KAT run had to disclose that gate requests leaked a shallow
decode figure before predictions were registered. Prediction 1 below is therefore a genuine forecast,
and if the gates emit `print_timing` before it is committed, that ordering will be recorded as a
failure of this run's discipline rather than quietly absorbed.

### Pre-registered predictions (recorded now, before the model is loaded)

Falsifiable, none of them measured:

1. **Shallow-context decode 8–11 t/s.** This is dense: every token reads all 50.1 GiB, exactly the
   regime where Muse (dense BF16, ~55.7 GB) measured 8–9 t/s on this machine. The hybrid layers
   change *KV growth*, not weight bandwidth, so they should buy nothing at shallow depth. Falsified
   below 8 or above 11 t/s. **This is the prediction that separates "hybrid" from "fast" — KAT's
   58.8–64.8 t/s came from MoE sparsity (8 of 256 experts), which this model does not have.**
2. **Decode at ≥ 100k depth retains ≥ 75% of shallow decode.** Only 16 of 64 layers grow a KV cache
   (64 KB/token, ~16.8 GB at the full 262,144 window), so the usual attention decay should be muted
   to roughly a quarter of its normal slope. Falsified by < 75% retention. **This is the run's
   central architectural claim.**
3. **No tool-call format collapse at depth.** KAT emitted 5,887 chars of Claude-dialect
   `<tool_calls><invoke name=` into the text channel at 229,841 tokens, losing three deliverables.
   That was attributed to training contamination, not harness. Predicting it does *not* recur here;
   falsified by any `invoke name=` hit in a `$.type='text'` part.
4. **Baseline score ≥ 16.** Above both unquantized runs currently sitting at 14 (muse, kat) and at
   least matching the best Qwen3.6-family result. A dense 27B at native precision scoring below a
   4-bit 118B MoE would be a repeat of the field's most durable finding, not a new one.
5. **The dominant failure mode will be fabricated-mechanism, not broken-build** — carried forward
   unchanged from KAT, where it held. Falsified by a submission that fails `go build`.

### Cull thresholds — carried forward unchanged, judged on baseline

Pre-registered before the Muse run and still unmet by every local model. Reproduced so this run is
judged against a bar set before it, not after.

| Threshold | Meaning |
|---|---|
| baseline > 18 | best local baseline ever recorded here — earns its slot |
| baseline ≥ 22 | retire gemma-4-31b and the remaining qwens |
| baseline ≥ 25 | retire qwen-agentworld and orinth-1.0-35b |

Field baselines: laguna-s-2.1 **18** (best local), qwen-3.6-27b 16, qwen-agentworld 16,
orinth-1.0-35b 16, qwen3.6-35b-mlx 15, muse-glimmer-30b-bf16 14, **kat-coder-v2.5-dev-bf16 14**,
qwen3-coder-next 13. Ornith-1.0-397B (22 → 28) is cloud-hosted and is not a local-disk decision.

### Wired — measured 2026-08-14, all five gates green

Everything above this line was committed at `2f9decb` before the model was loaded. Everything below
is measurement.

| Gate | Result |
|---|---|
| 1. Arch loads **live** | **PASS** — `qwen35` loaded in 14.4 s. Not a version comparison, not a strings grep. |
| 2. Chat template accepted | **PASS** — tool schema rendered, no template exception |
| 3. Real two-turn tool round-trip | **PASS** — see below |
| 4. Effective context asserted | **PASS** — 262,144 from `/props.default_generation_settings` *and* `/slots` |
| 5. Thinking on | **PASS** — `reasoning_content` returned as **text**, 185 + 152 chars over two turns |

**Gate 3 in full.** Turn 1 returned `finish_reason: tool_calls` with a correctly parsed
`get_datastore_capacity{"datastore":"LocalDS_0"}`; turn 2 consumed an injected tool result and
answered *"3,221,225,472 bytes, which equals 3 GiB"*. Recorded because `/props` reported
`chat_format: None` on this build — the same unreliable field that advertised `"Content-only"` on KAT
and would have produced a false abort if read alone. **Only the round-trip is evidence.**

Resident set 65.6 GB (50.1 GiB weights + KV). Vision reports `false` across all modalities, as
intended — the text-only GGUF was loaded without `mmproj`.

**Prediction 1 — HELD.** Pre-registered 8–11 t/s; measured **10.0 t/s** decode, 346 t/s prompt eval
at shallow depth. The dense-reads-everything model is ~6× slower than KAT's 58.8–64.8 t/s on the same
machine, which is the sparsity difference (8 of 256 experts) and not a quality difference. The
ordering discipline this run was built around held: the forecast was committed with zero weights
executed.

### Template probes — re-run for this model, and the conclusion is NOT the same as KAT's

Both probes are read-only (`/apply-template` + `/tokenize`). Script committed at
[`artifacts/tooling/template_probes.py`](../artifacts/tooling/template_probes.py).

**Probe A — reasoning preserved within a turn:** no difference. `preserve_thinking` true, false and
template-default all render **3 think-blocks / 206 tokens**, identical. Tool responses do not close
the turn, so a reasoning chain carries forward across an arbitrarily long tool-call sequence with the
flag off. Same shape as KAT.

**Probe B — prompt-cache stability across a new user turn:** this is where the flag actually acts.

| | before | after | shared prefix | verdict |
|---|---|---|---|---|
| preserve OFF | 206 | **179** | 62 (**30.1%**) | prefix broken — full reprocess |
| preserve ON | 206 | 223 | 206 (**100%**) | append-only |

With it off, history is **not** append-only: a new user message strips the prior turn's thinking out
of the *middle* of the prompt and the rendered prompt gets **shorter**. Every operator follow-up costs
one full context reprocess.

**The difference from KAT is quantitative, and it is the reason re-running the probes was not
ceremony.** KAT retained 10.3% of its prefix; this template retains 30.1%. Both are broken, so the
*decision* is unchanged — **`--reasoning-preserve` stays OFF** for a one-big-prompt autonomous run —
but the carried-over number would have been wrong by 3×. The server suggests the flag on load
(`chat template supports preserving reasoning`) and this model's card promotes `preserve_thinking` as
a first-class feature; neither is a reason to enable it here.

### Speculative decoding — confirmed off at runtime

`/slots` reports `speculative: false`. No draft model loaded, matching the twelve prior runs.

## Audit

## Score

## Compare

## Remediate

## Rescore
