---
name: kat-coder-v2.5-dev-bf16
created: 2026-08-13
model: Kwaipilot KAT-Coder-V2.5-Dev (Apache 2.0 — hybrid attention/SSM MoE, arch `qwen35moe`; 40 blocks, `full_attention_interval = 4` → 10 full-attention + 30 SSM layers; 256 experts, 8 used; 262,144 native context. Fine-tune of **Qwen3.6 35B A3B**. Run at **BF16, native precision** from `bartowski/Kwaipilot_KAT-Coder-V2.5-Dev-GGUF` (2 shards, 69,376,637,248 B = 64.6 GiB). Local on Apple M5 Max 128 GiB via Homebrew llama.cpp `llama-server`; driven via opencode)
stage: wired
score:
---

# Run — kat-coder-v2.5-dev-bf16

## Wire

**Status: wired 2026-08-13, all gates green. Eval prompt not yet delivered.** Directory
`kat-coder-v2.5-dev-bf16/` was seeded with the eval prompt only; the audit prompt is deliberately
**not** present and goes in at audit time, never before. Everything above *Wired* is
pre-registration, left as written before anything ran.

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
(`chat template supports preserving reasoning`). It changes what is fed *back* to the model across
turns, not what is captured: opencode persists each turn's `reasoning_content` either way, so it buys
no trace fidelity while changing behaviour away from the template's own default. The
`laguna-s-2.1-hf-Q4_K_M` run is the cautionary case — five variables moved at once, that flag among
them, and it proved nothing.

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
| baseline ≥ 25 | retire qwen-agentworld and orinth-1.0-35b |

Field baselines: laguna-s-2.1 **18** (best local), qwen-3.6-27b 16, qwen-agentworld 16,
orinth-1.0-35b 16, qwen3.6-35b-mlx 15, muse-glimmer-30b-bf16 14, qwen3-coder-next 13.
Ornith-1.0-397B (22 → 28) is cloud-hosted and is not a local-disk decision.

## Audit

## Score

## Compare

## Remediate

## Rescore
