---
name: qwen3.8-27b-8bit-mlx
created: 2026-08-16
model: Qwen3.8-27B (Apache 2.0 — hybrid linear-attention/SSM **dense** model, `model_type: qwen3_5`, `Qwen3_5ForConditionalGeneration`; 64 blocks, `full_attention_interval = 4`, 262,144 native context; natively multimodal, run **text-only**). Run at **MLX 8-bit** from `mlx-community/Qwen3.8-27B-8bit` — 6 safetensors shards, **29.53 GB**, quantization `bits 8, group_size 64, mode affine`. Local on Apple M5 Max 128 GiB via Homebrew **mlx-lm 0.31.3** (`mlx_lm.server`); driven via opencode
stage: wired
score:
---

# Run — qwen3.8-27b-8bit-mlx

## Wire

**Status: pre-registration.** Everything in this section is written **before the model is loaded**.
At the time of writing the weights are still downloading; **no weights have been executed and no
throughput number of any kind has been observed.** This is the same ordering discipline that
produced honest forecasts on the BF16 and Q8_0 rungs (both held prediction 1), and it is the fix
for KAT's leaked throughput figure.

### Why this run exists — and what it is NOT

The operator's goal is **wall-clock**: Q8_0 took **25.2 hours**. This rung asks whether MLX is a
faster backend for the same task.

**This is deliberately not a fourth rung on the precision ladder.** BF16 → Q8_0 was constructed so
precision was the *only* variable — same producer (`ggml-org`), same build 10360, same sampler,
same file family. MLX 8-bit changes **three things at once**:

| Variable | GGUF Q8_0 | MLX 8-bit |
|---|---|---|
| quantization scheme | block-32 **symmetric** | **`group_size 64, mode affine`** |
| packager | `ggml-org` | `mlx-community` |
| runtime | llama.cpp b10360 (Metal) | mlx-lm 0.31.3 |

A score delta against Q8_0's 20 is therefore **uninterpretable as a precision effect**, and must
not be reported as one. What this run *is* good for is the question the Q8_0 close left open:
BF16 (23) and Q8_0 (20) failed at **completely non-overlapping points** — BF16's transport died at
the URL parse, Q8_0's three hops later, while Q8_0 *fixed* BF16's defect and lost the standard
vSwitch path BF16 had working. That pattern is consistent with sampling variance rather than a
precision effect, and **n=1 per arm at temperature 1.0 cannot distinguish them.** This is a second
independent sample at ~8 bits.

### Architecture support — resolved before the download, not after

Same discipline as the GGUF rungs, which read the header over a range request rather than pulling
53.8 GB on faith. Here the equivalent is `config.json`:

```
model_type            = qwen3_5          <- mlx-lm 0.31.3 ships models/qwen3_5.py (118 archs)
architectures         = Qwen3_5ForConditionalGeneration
text_config           = 64 layers, full_attention_interval 4, max_position_embeddings 262144
quantization          = bits 8, group_size 64, mode affine
nextn / mtp keys      = NONE
```

Three risks checked and cleared in the same pass:

- **The loader resolves by string** — `importlib.import_module(f"mlx_lm.models.{model_type}")`.
  `qwen3_5` is present. Recorded as **necessary but not sufficient**: string presence in a module
  list is not a load, exactly as recorded for the GGUF arch table. **Gate 1 below is the load.**
- **Multimodal wrapper is handled.** This is a `ForConditionalGeneration` checkpoint with a
  `vision_config`, but `qwen3_5.Model.__init__` builds **only** `self.language_model` and never
  instantiates the vision tower. Text-only by construction, matching BF16/Q8_0.
- **The `#26916`-class MTP load failure does not apply** — no `nextn` keys, same clearance as the
  GGUF.

### Speculative decoding — investigated, and it is NOT available

The operator asked for a draft model if one exists. **MTP does not work on this backend**, and the
reason is worth recording so it is not re-derived:

- `mlx-community/Qwen3.8-27B-MTP-8bit` exists but is **0.48 GB** — the MTP *head alone*
  (`model_type: qwen3_5_mtp`, `mtp_num_hidden_layers: 1`), not a full model.
- **There is no `qwen3_5_mtp` module in mlx-lm 0.31.3**, so that repo cannot be loaded standalone,
  and `--draft-model` requires a fully loadable model.
- mlx-lm **actively discards MTP tensors**: `qwen3_5.py:313`
  `weights = {k: v for k, v in weights.items() if "mtp." not in k}`. It detects them only to decide
  a norm-weight shift (`:308-312`), then drops them.
- `server.py` has **zero** `mtp`/`eagle` references. Only `--draft-model` + `--num-draft-tokens`.
- **No smaller Qwen3.8 exists** to serve as a conventional draft — 27B is the only size shipped.

The one viable path is **self-speculation with a lower-bit quant of the same model**
(`mlx-community/Qwen3.8-27B-4bit`, ~15 GB, identical tokenizer and vocab). It is being fetched so
it is on disk if wanted, but **it is not used for this run** and expectations are low: a 27B 4-bit
draft is only ~2× cheaper than the 8-bit target, and speculative decoding needs a *much* cheaper
draft to pay off. This repo has already measured a draft going **27 t/s → 3 t/s** (laguna DFlash).
Pre-registered as a **separate throughput arm after the baseline scores**, never as a ladder
variable — the same treatment MTP got on the BF16 rung.

### Sampler — the vendor preset, passed explicitly

`temperature 1.0, top_p 0.95, top_k 20, min_p 0.0`. **`mlx_lm.server`'s defaults are wrong for this
model** — it defaults to `--temp 0.0 --top-p 1.0 --top-k 0`, which would silently make this the
only greedy run in the field. All four flags must be passed explicitly. Recorded because the
equivalent silent-default trap (LM Studio's laguna-shaped 262,144/0.6/20) is why this repo pins
`llamacpp-local` rather than `lmstudio`.

### The honesty instrument survives — verified before committing to the backend

The entire reasoning-forensics method depends on reasoning arriving as **text**, not a token count.
LM Studio reports a count and would destroy it. Checked in `mlx_lm/server.py`:

| key | occurrences |
|---|---|
| `reasoning` | 22 |
| `reasoning_content` | **0** |
| `tool_calls` / `tools` | 25 / 14 |

MLX emits **`reasoning`**, not `reasoning_content`. `@ai-sdk/openai-compatible@3.0.30` reads
`reasoning_content ?? reasoning`, so it maps through opencode and the instrument is intact. Tool
calling is implemented. **Both are asserted at gates 3 and 5 rather than trusted** — the `/props`
`chat_format: None` lesson stands: only a real round-trip is evidence.

### Pre-registered predictions (recorded before the model is loaded)

1. **Shallow-context decode 15–23 t/s.** Decode is memory-bandwidth bound. MLX 8-bit reads
   **29.53 GB**/token against GGUF Q8_0's 28.60 GB — *slightly more* bytes, so the naive
   expectation is **marginally slower**, not faster. Any large gain would have to come from kernel
   efficiency, not bandwidth. Falsified outside 15–23. **This directly tests the operator's
   hypothesis that MLX is faster, and the bandwidth arithmetic predicts it is not.**
2. **Prompt processing ≥ 350 tok/s at shallow depth** (GGUF Q8_0 measured 470). Falsified below.
   *This is the number that actually governs wall-clock*, not decode — see 3.
3. **Wall-clock is NOT dominated by decode, and MLX will not fix it.** Q8_0's 25.2 h contained a
   single compaction costing **5 h 04 m** — 20.1% of the run — reprocessing 173,606 tokens at
   `cache:{write:0,read:0}`. Predicting the dominant wall-clock term is again **prompt reprocessing
   at depth, not token generation**. Falsified if decode time exceeds prompt-processing time.
4. **Effective context 262,144 asserted from the server, and KV growth is the risk.** mlx-lm
   exposes no `--max-kv-size` / F16-KV equivalent in its server flags. Falsified if the served
   context is below 262,144 or if the run OOMs before 200k.
5. **Baseline score 18–24.** Deliberately wide: this is a *variance* probe, not a precision probe.
   A result near 20 with a **similar defect profile** argues for a real ceiling; a result near 23
   with **different** defects argues the BF16/Q8_0 gap was sampling. Either is informative; the
   uninformative outcome is a score in band with an *identical* defect profile.
6. **The standard-vSwitch defect does NOT recur.** Q8_0 queried `HostSystem.network` for
   `HostVirtualSwitch` refs — data objects that never appear there — and never ran the subcommand
   until 9 hours after writing it. Predicting this specific error is **run-specific, not
   model-level**, since BF16 got it right. Falsified if `vswitches` again emits zero standard rows.

### Instrument corrections carried into this run's audit

From the Q8_0 close, so they are not re-derived — note two of these *reverse* earlier guidance:

- **`LACP disabled` is honest** (`LacpGroupConfig` empty). Do not charge.
- **"DVS PORTS 0 is the true value" is WRONG at govmomi v0.55.1** — `Config.NumPorts = 1`. Charging
  a printed `1` as fabrication would be an auditor error.
- **The standard-vSwitch half still holds:** vcsim stubs `networkSystem.networkInfo` to zeros while
  `config.network` carries the real **1536/1530** on 4 × `vSwitch0`.
- **`UPLINKS -` is an honest degrade** — `UplinkPortPolicy` is genuinely nil at vcsim.
- **The spec's prescribed `go run github.com/vmware/govmomi/vcsim` is structurally impossible** at
  v0.55.1 (nested module, absent from the module zip). A separate vcsim harness module is a correct
  workaround and is **credited, not charged**.
- **`pflag` is not a dependency violation** — `viper.BindPFlag` requires it.
- **The alleged "empty-markdown" Q8_0 degradation signature does not exist** and was refuted from
  the raw session store. Do not carry it forward.
- **R7:** never pre-register a retention threshold without a matched-depth control arm — which is
  why no retention-percentage bar appears above.
- **R8:** write falsification clauses against the *outcome*, not a named mechanism. Prediction 6 is
  phrased as "emits zero standard rows", not "queries the wrong property".

### Harness change under consideration — recorded before it could bias the result

The Q8_0 post-mortem established that the model wrote the entire `vswitches` implementation and did
not execute it for **9.03 hours**, running the binary against a live simulator only in the final
~15 minutes. A gate requiring a live run with **asserted row content** before code counts as
written would have caught it — and would also have caught gemma-4-31b's fabricated vswitches.
**It is NOT being applied to this run**, which stays harness-identical to the sixteen prior runs.
Recorded here so that if it is adopted later, the ordering is on paper.

### Cull thresholds — carried forward unchanged, judged on baseline

| Threshold | Meaning |
|---|---|
| baseline > 18 | best local baseline ever recorded here |
| baseline ≥ 22 | retire gemma-4-31b and the remaining qwens |
| baseline ≥ 25 | retire qwen-agentworld and orinth-1.0-35b |

Field baselines: **qwen3.8-27b-bf16 23 (best)**, qwen3.8-27b-q8_0 20, laguna-s-2.1 18,
qwen-3.6-27b 16, qwen-agentworld 16, orinth-1.0-35b 16, qwen3.6-35b-mlx 15,
muse-glimmer-30b-bf16 14, kat-coder-v2.5-dev-bf16 14, qwen3-coder-next 13.

**Caveat on the cull, recorded now:** every retirement decision in this field currently rests on
**n=1 per model**, and the BF16/Q8_0 pair is direct evidence that n=1 is unstable at this bar. The
thresholds are reproduced because they were set before this run, not because they are sound.

### Wired — measured 2026-08-16

Everything above this line was committed at `9acd031` with the weights still downloading.
Below is measurement.

| Gate | Result |
|---|---|
| 1. Arch loads **live** | **PASS** — `qwen3_5` loaded and generated. Not a module-list grep |
| 2. Chat template accepted | **PASS** — tool schema rendered, no template exception |
| 3. Real two-turn tool round-trip | **PASS** — `finish_reason: tool_calls`, args `{"datastore":"LocalDS_0"}`, turn 2 consumed the injected result: *"3,221,225,472 bytes, which is exactly 3 GiB"* |
| 4. Effective context asserted | **PARTIAL** — an **84,966-token** prompt was accepted and fully prefilled. The full 262,144 is **not yet asserted**; recorded as outstanding rather than claimed |
| 5. Thinking on | **PASS** — `reasoning` returned as **text** (374 chars), key is `reasoning` not `reasoning_content`, as predicted |

Resident set **27.1 GB** — against the GGUF Q8_0 rung's 43.0 GB for the same precision. **MLX uses
~16 GB less**, the one unambiguous win in this section.

### Prediction 1 — HELD, and it falsifies the reason the rung was requested

Pre-registered 15–23 t/s. Measured **16.17 t/s** median (16.17 / 16.25 / 15.67 over three
600-token streamed generations).

Against the GGUF Q8_0 rung's **18.7 t/s** that is **13.5% slower**. The bandwidth arithmetic in the
pre-registration called this: MLX 8-bit reads 29.53 GB/token against GGUF Q8_0's 28.60 GB. The
prediction was deliberately written against the operator's hypothesis that MLX would be faster, and
the hypothesis is **falsified for decode**.

### Prediction 2 — FALSIFIED, and this is the headline

Pre-registered **≥ 350 tok/s** prompt processing. `mlx_lm.server` logs prefill progress every 2048
tokens, which yields a free rate-vs-depth curve — reported as a **curve, not a threshold**, per R7:

| depth (tok) | prefill rate |
|---|---|
| 6,144 | 172.4 tok/s |
| 10,240 | **180.2** (peak) |
| 16,384 | 155.0 |
| 26,624 | 142.2 |
| 36,864 | 136.3 |
| 47,104 | 128.3 |
| 57,344 | 121.4 |
| 67,584 | 114.6 |
| 69,632 | 110.9 |

**Peak ~180 tok/s against a 350 bar. Falsified by roughly 2×** — and against llama.cpp's measured
**470 tok/s** on the same model and machine, MLX prefill is **~2.6× slower at shallow depth and
~4× slower by 70k**, still degrading.

### What this means for the goal that motivated the rung

The rung was requested to **cut wall-clock**. Projecting the Q8_0 run's own measured token volumes
(426,516 prompt tokens processed, 145,454 decoded) onto both backends:

| Backend | prefill | decode | compute total |
|---|---|---|---|
| GGUF Q8_0 (measured rates) | 0.25 h | 2.16 h | **2.41 h** |
| MLX 8-bit (measured rates, generous 150 tok/s prefill) | 0.79 h | 2.50 h | **3.29 h** |

**MLX is ~36% *worse* on compute for this task, and the gap widens with depth** because its prefill
curve decays faster. Prediction 3 (wall-clock is governed by prompt reprocessing, not generation)
is **supported in mechanism** — prefill is where the backends differ most — but the direction is the
opposite of what was hoped.

**Recorded caveat:** this does *not* make MLX a worse backend in general. It has 16 GB lower
resident set, a clean `reasoning` channel, and working tool calls. It is worse **for this
prefill-heavy 200k-token task specifically.**

### Speculative decoding — draft acquired, not used

`mlx-community/Qwen3.8-27B-4bit` (15 GB, 3 shards, complete) is on disk as the only viable draft,
since MTP is unusable on this backend (see pre-registration). **Not used for this run.** Given
prefill is the bottleneck and speculative decoding accelerates *decode only*, it addresses the
smaller of the two terms — the same reasoning that kept MTP off the BF16 rung.

### Server invocation — and a silent trap worth recording

```
mlx_lm.server --model mlx-community/Qwen3.8-27B-8bit \
  --host 127.0.0.1 --port 8080 \
  --temp 1.0 --top-p 0.95 --top-k 20 --min-p 0.0 \
  --max-tokens 65536 --log-level INFO
```

- **`--max-tokens` defaults to 512.** Left unset, every generation truncates at 512 tokens and the
  run would fail in a way easily mistaken for model collapse. This is the MLX analogue of LM
  Studio's laguna-shaped defaults.
- **All four sampler flags must be passed.** The server defaults to `temp 0.0 / top_p 1.0 /
  top_k 0` — greedy — which would have made this the only greedy run in the field.
- `--log-level DEBUG` floods the log with httpcore noise; **INFO** keeps the prefill-progress lines
  that produced the curve above.

### Anomaly, logged and watched

The **very first** request returned JSON containing a raw control character, which strict
`json.loads` rejected. **Not reproduced in 5 subsequent requests** (0/5 strict-parse failures).
Recorded because a strict client mid-run would surface it as an opaque provider error. One
occurrence in six requests, all on the cold first call.

## Audit

## Score

## Compare

## Remediate

## Rescore
