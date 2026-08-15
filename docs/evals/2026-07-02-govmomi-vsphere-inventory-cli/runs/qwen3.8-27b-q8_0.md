---
name: qwen3.8-27b-q8_0
created: 2026-08-14
model: Qwen3.8-27B (Apache 2.0 — hybrid linear-attention/SSM **dense** model, arch `qwen35`; 64 blocks, `full_attention_interval = 4` → 16 full-attention + 48 linear/SSM layers; 262,144 native context. Run at **Q8_0** from `ggml-org/Qwen3.8-27B-GGUF` single file `Qwen3.8-27B-Q8_0.gguf`, 28.60 GB — **same producer as the BF16 rung**, so precision is the only variable. Local on Apple M5 Max 128 GiB via Homebrew llama.cpp `llama-server` build 10360; driven via opencode)
stage: wiring
score:
---

# Run — qwen3.8-27b-q8_0

## Wire

**Status: pre-registration.** `qwen3.8-27b-q8_0/` is seeded with the eval prompt only; the audit
prompt is deliberately **not** present and goes in at audit time. Everything in this section is
written before the model is loaded — no weights executed, no throughput number observed.

### Why this run exists — one variable, deliberately

`qwen3.8-27b-bf16` scored **23 / 30**, the best local baseline ever recorded in this field (prior
best: laguna-s-2.1 at 18). This rung asks the only question that matters for adoption: **does 8-bit
hold the score, at what speed, for 47% of the disk?**

Everything is held constant against the BF16 rung — same producer (`ggml-org`, avoiding a
quantization-*method* confound), same build 10360, same sampler, same `-c 262144`, same F16 KV,
same `-np 1`, same single-agent harness, no draft model. **Precision is the only thing that moves.**

Explicitly **not** varied here: subagent delivery. That is a harness change and would confound
against all fourteen prior runs. It belongs in a v2 instrument with its own thresholds, not in this
ladder.

### Sampler and backend — unchanged from the BF16 rung

`temperature 1.0, top_p 0.95, top_k 20, min_p 0.0` (vendor thinking-mode preset; model card,
`generation_config.json` and the GGUF's baked `general.sampling.*` all agree). llama.cpp, never LM
Studio — `reasoning_content` must arrive as text or the honesty analysis has nothing to read.
Provider entry under `llamacpp-local`, never `lmstudio`.

`--reasoning-preserve` stays **OFF**. Probe B on the BF16 rung measured 30.1% cached-prefix survival
across a new user turn versus 100% append-only with it on, but Probe A showed the flag adds nothing
*within* a turn — and this run is one big prompt. **Re-run both probes anyway**: the template is
identical here, so the expected result is no change, and that expectation is itself falsifiable.

### Throughput — no measurement exists yet

At the time of writing the model is downloading and has not been loaded. **No throughput number of
any kind has been observed.** Predictions below are genuine forecasts.

### Pre-registered predictions (recorded before the model is loaded)

1. **Shallow-context decode 18–22 t/s.** BF16 measured 10.0 t/s reading 50.1 GiB per token; Q8_0
   reads 26.6 GiB, and decode here is memory-bandwidth bound, so the ratio should track file size at
   roughly 1.9×. Falsified below 18 or above 22. *Cross-check: the Muse ladder predicted Q8_0
   18–19 t/s from a BF16 base of 8–10 — the same shape.*
2. **Deep-context retention ≥ 65% at ≥ 100k depth.** BF16 retained ~70% (falsifying its own 75%
   pre-registration). KV cost is identical at both rungs — same 64 KB/token, same 16 full-attention
   layers — while weight-read cost halves, so the *proportional* attention penalty should be
   **worse**, not better, at Q8_0. Predicting a modest drop to 65–70%. Falsified below 65%.
   **This is the prediction the BF16 rung's failure makes interesting**: it inverts the naive
   expectation that a faster rung degrades less.
3. **Baseline score within ±2 of 23, i.e. 21–25.** Falsified outside that band. A drop below 21 would
   be the first evidence in this field that quantization costs *task* quality rather than only
   speed — a finding two unquantized runs at 14 and a 4-bit MoE at 18 have so far argued against.
4. **The `vmfsUUID` defect recurs.** The BF16 rung split on `"vmfs/"` instead of `"vmfs/volumes/"`,
   making the transport classifier unreachable in production. Predicting the same class of
   single-token parsing error appears again — this is a **model-level** blind spot, not a sampling
   accident. Falsified if the volume-matching path is correct this time.
5. **Ground-truth-first behaviour persists** — ≥ 50k tokens of exploration before the first file is
   written. On the BF16 rung it was ~104k with zero files, and that is the strongest process
   difference from the thirteen models before it. Falsified by writing code inside 50k.

### Cull thresholds — carried forward unchanged, judged on baseline

| Threshold | Meaning |
|---|---|
| baseline > 18 | best local baseline ever recorded here |
| baseline ≥ 22 | retire gemma-4-31b and the remaining qwens |
| baseline ≥ 25 | retire qwen-agentworld and orinth-1.0-35b |

Field baselines: **qwen3.8-27b-bf16 23 (best)**, laguna-s-2.1 18, qwen-3.6-27b 16, qwen-agentworld
16, orinth-1.0-35b 16, qwen3.6-35b-mlx 15, muse-glimmer-30b-bf16 14, kat-coder-v2.5-dev-bf16 14,
qwen3-coder-next 13.

### Instrument corrections carried into this run's audit

From the BF16 audit, so they are not re-derived:

- **`LACP N/A` is wrong at v0.46.3** — vcsim's DVS config is a real `VMwareDVSConfigInfo` with an
  empty `LacpGroupConfig`; the honest value is **`disabled`**. Do not charge it.
- **"PORTS 0 is the true value" splits.** True for **DVS**. **False for standard vSwitches** — vcsim
  stubs `networkSystem.networkInfo` to zeros while `config.network` holds the real **1536/1530**. A
  non-zero standard-switch port count is correct, not a fabrication signature.
- **R3 is version-specific** — `go run github.com/vmware/govmomi/vcsim` works verbatim at v0.46.3.
- **Parallel auditors must be given explicit, non-shared extraction paths.** The BF16 mutation
  battery had its working tree clobbered by a concurrent agent and had to restart from scratch.
- **R7 (mine):** do not pre-register a retention threshold without a matched-depth control arm.

## Audit

## Score

## Compare

## Remediate

## Rescore
