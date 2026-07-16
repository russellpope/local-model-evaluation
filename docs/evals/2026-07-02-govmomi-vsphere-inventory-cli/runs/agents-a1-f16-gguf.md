---
name: agents-a1-f16-gguf
created: 2026-07-15
model: Agents-A1 (InternScience, arch qwen35moe, F16 GGUF 69.38 GB, 256 experts / 8 used, hybrid SSM+attention; local on Apple M5 Max 128 GiB via LM Studio llama.cpp Metal 2.24.0; driven via opencode)
stage: wired
score:
---

# Run — agents-a1-f16-gguf

## Wire

**Model.** `InternScience/Agents-A1-F16-GGUF` — `Agents-A1-F16.gguf`, 69,376,636,992 bytes
(64.61 GiB), F16 (`general.file_type = 1`, no quantization confound), Apache-2.0, size label
`256x2.6B` (~34B total params). Architecture `qwen35moe`, but the GGUF metadata shows a **hybrid
SSM + attention** design, not a plain MoE transformer: 40 blocks, hidden 2048, 16 heads / **2 KV
heads**, K/V head-dim 256, rope freq_base 1e7, `expert_count = 256`, `expert_used_count = 8`,
`expert_feed_forward_length = 512`, plus `ssm.{conv_kernel 4, state_size 128, group_count 16,
time_step_rank 32, inner_size 4096}` and **`full_attention_interval = 4`** — i.e. only every 4th
layer (10 of 40) is full attention; the other 30 are SSM/linear-attention layers whose recurrent
state is constant per sequence. Estimated ~2–3B active params/token. Tokenizer: gpt2/BPE, pre
`qwen35`, **vocab 248,320**, EOS 248046, no BOS. `trainedForToolUse: true`. Ships a sibling
`Agents-A1-mmproj.gguf` (arch **clip**, 447M) — a vision projector, unused here.

**Load config (LM Studio).** Context **262144** (model max), GPU offload **40/40 layers** (all),
Flash Attention on, K/V cache quantization **off** (F16 cache — no quality confound), Unified KV
Cache on, KV offloaded to GPU, mmap on, Keep-model-in-memory on, experts **8** (the trained
`expert_used_count` — not altered), RoPE Auto (model declares 1e7), eval batch 2048 / physical
batch 512, **PARALLEL 1**, **Speculative decoding: OFF**.

*Speculative decoding was deliberately disabled.* LM Studio offered `Agents-A1-mmproj.gguf` in the
draft-model dropdown (it filters by folder, not capability) but that file is a **CLIP vision
projector with no LM head** and cannot propose tokens; the model's own shipped default is
`speculativeDecoding.draftModel: ""`. No valid draft exists locally in any case: the other
same-family GGUFs (`ornith-1.0-35b`, `qwen-agentworld-35b-a3b`) are 69 GB — as large as the target;
the Gemmas have an incompatible vocab; `qwen/qwen3.5-9b` and `qwen/qwen3.6-27b` are MLX/safetensors
(wrong runtime for a GGUF target). A future draft would need a small dense Qwen3.5-family **GGUF**
matching vocab 248,320 / pre `qwen35`, and would have to be cheaper than the target's ~2–3B active
path to pay off at all.

**Memory (measured, not estimated).** Loaded `llama-server` RSS **70.03 GiB** at 262144 ctx —
consistent with 64.61 GiB weights + ~5.0 GiB KV. The KV cost is ~20 KiB/token (10 attention layers
× 2 KV heads × (256+256) × 2 B), **not** the ~80 KiB/token a 40-layer-all-attention model would
imply; the hybrid SSM layout is what makes full 256k context affordable on 128 GiB. Note
`lms load --estimate-only` reports a flat 64.61 GiB at *every* context — it counts weights and
ignores KV entirely (`Confidence: LOW`), so the resource guardrail cannot be relied on here.

**Sampling (via opencode → LM Studio OpenAI-compatible API at :1234).** Cohort eval preset,
identical to the other local runs: `temperature 0.6, top_p 0.95, top_k 20`, `limit {context:
262144, output: 65536}`. Temperature 0.6 is the publisher's own shipped default (the only
inference field LM Studio's stored model config pins). `repeat_penalty` is **not** sent by
opencode and falls back to a server-side default — unpinned, but applied uniformly across every
model in this cohort. LM Studio's Inference-tab values (repeat penalty 1.1, top_k 40, min_p 0.05)
govern the GUI chat path, not this API path. Reasoning/thinking **enabled**.

**Harness.** opencode 1.18.2, provider `lmstudio` (`@ai-sdk/openai-compatible`, baseURL
`http://localhost:1234/v1`), model id `internscience/agents-a1-f16-gguf/agents-a1-f16.gguf`
(selected as `lmstudio/internscience/agents-a1-f16-gguf/agents-a1-f16.gguf`). Wire verified live
before the run: plain completion returned exactly `WIRED`; tool call returned
`finish_reason: tool_calls` with well-formed args `{"command":"ls /etc"}`. **Heavy reasoner** —
210 reasoning tokens to emit `WIRED`, 51 to decide on `ls`; expect reasoning to dominate token
spend and wall-clock (same profile as ornith-1.0-397B).

**Runtime incident — two reasoning-loop stalls requiring user restart (confirmed from transcript).**
The model **stalled twice mid-run, each time caught in a reasoning loop that produced zero output**,
and had to be manually restarted both times. Evidence from the opencode session store
(`~/.local/share/opencode/opencode.db`, session `ses_098713d18ffeS5WtbavHHdmZM8`, 13:53:57 →
21:21:02):

| Time | Duration | Output tokens | Reasoning tokens | `finish_reason` |
|---|---|---|---|---|
| 14:30:02 | 22 min | **0** | **32,000** | `length` |
| 15:45:43 | 26 min | **0** | **32,000** | `length` |

Each stall reasoned until it hit a hard 32,000-token ceiling and emitted **no content and no tool
call** — the signature of a reasoning loop, not a slow-but-progressing generation. User
interventions are recorded as the only non-kickoff user messages: `"go"` at 15:01:29 (restarting
after stall 1) and `"continue"` at 21:07:26 (stall 2 left the session idle for **4h 55m** —
the model never recovered on its own).

Aggregate for the session: **91,756 of 93,780 output tokens (97.8%) were reasoning tokens**, and
**64,000 of those (70% of all reasoning) were spent inside the two dead stalls**, yielding zero
output. 134 assistant messages, 3 with zero output. This corroborates the pre-run wire observation
that the model is a heavy reasoner (210 reasoning tokens to emit `WIRED`) — at eval scale that
tendency degenerated into non-termination twice.

**Unresolved — do not treat as settled:** the exact round number `32,000` suggests a *configured*
reasoning cap rather than a model-intrinsic limit; opencode's configured `limit.output` here is
65,536, so 32,000 comes from somewhere else (LM Studio server default or model config — not yet
identified). This matters for attribution: the *stall* (reasoning without emitting) is model
behavior regardless, but whether the 32,000 ceiling is the harness's or the model's must be
verified before the audit draws a conclusion from it. The restarts also mean this run is **not a
clean unaided baseline** — two operator interventions are in the transcript and must be disclosed
in the audit.

**Open (fill after the run):** submission contents, `go build` / `go vet` on arrival, git-history
forensic availability, driving-plan artifacts, wall-clock.

## Audit

## Score

## Compare

## Remediate

## Rescore
