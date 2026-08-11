---
name: muse-glimmer-30b-bf16
created: 2026-08-11
model: Muse Glimmer 30B (Meta, released 2026-08-10, Apache 2.0 — dense causal transformer, ~29.6B total incl. ~1.8B ViT-G/14 perception encoder, ~27.9B language model; 52 layers, alternating SWA-2048 / global attention; 131,072+ context. Run at **BF16, native precision** from `unsloth/Muse-Glimmer-30B-GGUF` (2 shards, 55,725,511,168 B = 51.9 GiB). Local on Apple M5 Max 128 GiB via self-built llama.cpp llama-server; driven via opencode)
stage: staged
score:
---

# Run — muse-glimmer-30b-bf16

## Wire

**Status: staged, not yet wired.** Directory seeded with the eval prompt only. The audit prompt is
deliberately **not** present in the workspace — it is dropped in at audit time, never before.
No submission yet, no server yet, no gates run.

### Why this run exists

Every FAIL in this field carries the same unresolved asterisk: *was it the model, or was it the
quantization?* [`laguna-s-2.1`](laguna-s-2.1.md) states it outright — no F16 build was available
locally, so the confound could not be eliminated within the run. The follow-up
[`laguna-s-2.1-hf-Q4_K_M`](laguna-s-2.1-hf-Q4_K_M.md) was meant to close it and could not: five
variables moved at once (quantization, `--reasoning-preserve`, q8_0 KV, a different runtime, and a
mid-run cancel/restart), so its 16 → 20 arc against the 4-bit build's 18 → 22 proves nothing about
precision.

Muse Glimmer closes it, because at 51.9 GiB the native weights simply fit. **This is the first run
in the field with no quantization anywhere in the stack** — not the weights, not the KV cache. Any
weakness it shows is the model.

It also lands squarely on two models already audited under this rubric, which the model card names
as its own comparators:

| Comparator | In this field | Result |
|---|---|---|
| Gemma 4 31B | [`gemma-4-31b`](gemma-4-31b.md) | FAIL baseline, 3 Criticals — fabricated vswitches, vcsim loop never run |
| Qwen 3.6 27B | [`qwen-3.6-27b`](qwen-3.6-27b.md) | FAIL 16/30, 5 Criticals — binary cannot connect, dead-code classifier |

The card claims to beat both across 25+ benchmarks. That is a falsifiable prediction and this eval
tests it on a task none of those benchmarks cover.

### Pre-registered precision ladder

Run **sequentially, never concurrently** — the operator's constraint, and correct: three rungs
resident at once is ~94 GiB of weights before any context, which lands at 110–115 GiB with KV and
compute buffers on a 128 GiB machine.

| Rung | File | Weights | Status |
|---|---|---|---|
| 1 | `BF16/Muse-Glimmer-30B-BF16-0000{1,2}-of-00002.gguf` | 51.9 GiB | **scored baseline** — this record |
| 2 | `Muse-Glimmer-30B-Q8_0.gguf` | 27.6 GiB | pre-registered |
| 3 | `Muse-Glimmer-30B-UD-Q4_K_XL.gguf` | 14.8 GiB | pre-registered |

**All three rungs come from `unsloth/Muse-Glimmer-30B-GGUF` and nothing else.** Meta's own
`meta-models/Muse-Glimmer-30B-GGUF` publishes only 4-bit K-quants (`kquant-17gb` 16.76 GB,
`kquant-dynamic` 19.65 GB) and no full-precision GGUF. Mixing a Meta 4-bit rung under an unsloth
BF16 baseline would put a *producer* variable — different quantization method, different imatrix
pipeline — inside the one experiment whose entire purpose is a single moving variable. Do not do it.
If Meta's kquant builds are worth scoring, they are a separate run, not a rung.

Everything else is pinned across all three rungs: same server flags, same context, same sampler,
same opencode provider, same eval prompt, same auditor procedure. **F16 KV on every rung** — the
laguna arc was forced into `--cache-type-k/v q8_0` by a 89.4 GiB weight footprint; at 51.9 GiB there
is no such pressure, and quantized KV would reintroduce the exact confound this ladder exists to
remove.

Two vendor claims the ladder incidentally tests, neither measured on a multi-hour agentic task with
mutation-tested ground truth: Meta's ~0.2% degradation for its dynamic 4-bit build, and unsloth's
"UD 2.0 outperforms other leading quants."

### Wire gates — all four must pass before the eval prompt goes in

1. **Build ≥ `b10353`.** Muse Glimmer support merged 2026-08-10, llama.cpp
   [#26841](https://github.com/ggml-org/llama.cpp/pull/26841). The current local build predates it.
   Record the build tag actually run.
2. **`--jinja` present.** Without it the server errors `this custom template is not supported`.
3. **Stop-token round-trip.** End sequences are `<|end_of_text|>` and `<|eot|>`. **Do not stop on
   `<|eom|>`** — that is end-of-message, which terminates a tool call. Stopping there truncates the
   agentic loop mid-turn and would present as a model that cannot finish tasks. Gate is a real
   multi-call tool round-trip through opencode, not a single completion.
4. **Context is what it says.** With `-np` slots, per-request context is `-c / -np`. Assert the
   effective figure, do not assume it.

Plus the standing thinking gate (`check-thinking.sh`, backend-agnostic since `a0547c4`).

**Not loaded:** `mmproj-Muse-Glimmer-30B-BF16.gguf` (3.85 GB). The task is text-only Go; the
perception encoder never fires. The HF card's "60 GB" is shards + projector — resident weight here
is 51.9 GiB.

**DFlash is a second variable, not part of rung 1.** `dflash-kquant.gguf` (1.63 GB) is the native
block-diffusion drafter, 16 tokens per forward pass, claimed lossless — unlike the mismatched draft
model that took laguna from ~27 t/s to 3 t/s. It should help. Establish the BF16 baseline without
it, then A/B it on its own.

## Audit

## Score

## Compare

## Remediate

## Rescore
