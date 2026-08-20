# Ornith 1.5 — download manifest

Prepared 2026-08-20. Release date 2026-08-19, MIT.

## Provenance

**All pulls are first-party `ornith-ai` only.** No third-party requants.

`ornith-ai` is the DeepReinforce team's org, renamed. Verified mechanically rather than by
badge — the org this repo already pulled Ornith 1.0 from redirects to it:

```
$ curl -sI https://huggingface.co/deepreinforce-ai/Ornith-1.0-35B
307 -> https://huggingface.co/ornith-ai/Ornith-1.0-35B
```

There is no HF verification badge on the org. The redirect from an org already trusted in
this field is the stronger evidence.

Third-party repos exist (`bartowski`, `AtomicChat`, `mradermacher`, `mudler`, `Avifenesh`,
`scottlowry`) and are **deliberately excluded**. The two-stage design is a precision study,
so a requant's calibration choices would land directly on the variable under measurement.

## Manifest

Target: `/Users/ldh/models/`, alongside the existing `Qwen3.8-27B-GGUF/`.

| # | Repo | File | Bytes | Size |
|---|------|------|-------|------|
| 1 | `ornith-ai/Ornith-1.5-9B-GGUF` | `Ornith-1.5-9B-Q8_0.gguf` | 9,527,501,248 | 8.87 GiB |
| 2 | `ornith-ai/Ornith-1.5-9B-GGUF` | `Ornith-1.5-9B-BF16.gguf` | 17,920,696,768 | 16.69 GiB |
| 3 | `ornith-ai/Ornith-1.5-9B-GGUF` | `mmproj-Ornith-1.5-9B-BF16.gguf` | 921,704,416 | 0.86 GiB |
| 4 | `ornith-ai/Ornith-1.5-35B-A3B-GGUF` | `Ornith-1.5-35B-Q8_0.gguf` | 37,802,149,120 | 35.21 GiB |
| 5 | `ornith-ai/Ornith-1.5-35B-A3B-GGUF` | `Ornith-1.5-35B-BF16.gguf` | 71,066,994,240 | 66.18 GiB |
| 6 | `ornith-ai/Ornith-1.5-35B-A3B-GGUF` | `mmproj-Ornith-1.5-35B-BF16.gguf` | 902,822,016 | 0.84 GiB |
|   | | **total** | **138,141,867,712** | **128.65 GiB** |

All six verified `HTTP 200` with the byte sizes above on 2026-08-20. Sizes are
`x-linked-size` from the CDN, not README claims.

Free space at manifest time: **427 GiB**. Post-download: ~298 GiB.

Order is smallest-first so the cheapest file validates the loader before the 66 GiB one
commits.

### A note on "FP16"

The stage-1 rung is **BF16**, not FP16 — that is what Ornith ships and what the model was
trained in. It is the correct full-precision rung and matches the existing
`qwen3.8-27b-bf16` run. Naming only; no action needed.

### mmproj

Ornith 1.5 is multimodal (both sizes carry a `vision_config`) — new versus the 1.0 runs.
The govmomi eval is text-only, so `mmproj` is **not required** for scoring. Included because
it is 1.7 GiB for both and cannot be reconstructed later without a re-pull.

## Architecture and engine support — verified, not assumed

GGUF arch names are `qwen35` (9B) and `qwen35moe` (35B) — **not** the `qwen3_5` HF
`model_type`, which is why a naive grep for the arch string comes back empty.

llama.cpp **build 10470** (`34af94cd9`, installed 2026-08-17, predates the release by two
days) already supports both. From `libllama.dylib`:

```
"QWEN35 MTP currently only supports a single MTP block"
"QWEN35MOE MTP currently only supports a single MTP block"
```

The arch enums and their MTP paths are compiled in. No llama.cpp upgrade is required before
stage 1. This is expected — Qwen3.8-27B is the same `qwen35` family and already runs here.

## MTP / speculative decoding status

Checked per-model against both the HF `config.json` (does the checkpoint have the head?) and
the shipped GGUF KV metadata (did conversion keep it?).

| Model | Head in checkpoint | `nextn` in GGUF | Usable today |
|---|---|---|---|
| Ornith-1.5-35B-A3B (BF16 + Q8_0) | `mtp_num_hidden_layers: 1` | **yes** — `qwen35moe.nextn_predict_layers = 1`, `block_count = 41` | **Yes** |
| Ornith-1.5-9B (BF16 + Q8_0) | `mtp_num_hidden_layers: 1` | no — `block_count = 32`, no key | No |
| Qwen3.8-27B (local Q8_0) | `mtp_num_hidden_layers: 1` | no — `block_count = 64`, no key | No |

The 35B's `block_count = 41` against 40 transformer layers is the MTP head surviving as
`blk.40`. **The official GGUF already carries it** — the third-party "APEX-MTP" repos that
advertise bundling it add nothing the first-party file lacks.

Both 9B and Qwen3.8-27B have the head upstream but lose it in GGUF conversion. Recovering it
means converting from safetensors locally; not queued.

Free, no extra download, 35B only:

```
llama-server -m Ornith-1.5-35B-Q8_0.gguf --spec-type draft-mtp -ngl 99
```

Treat as a throughput experiment **after** scoring, not during — it changes decode
behaviour and would confound the precision comparison.

### DSpark — unavailable for every model in scope

- **Ornith 1.5** — no dspark/dflash drafter published, either size. Existing Ornith drafters
  (`stanleyphoong/Ornith-1.0-9B-DSpark`, a `pablogrant` 1.0-35B fork) target **1.0**;
  drafters are target-specific and will not transfer to 1.5.
- **Qwen3.8-27B** — `RadixArk/Qwen3.8-27B-DSpark` exists (2.72 GB) but is safetensors for
  vLLM/SGLang with no GGUF. The deeper blocker is architectural: `qwen35` is a hybrid
  backbone (Qwen3.8-27B is 48 `linear_attention` / 16 `full_attention`), and the recurrent
  linear-attention state cannot be advanced or rolled back through a batched spec-decode
  verify path. MLX force-disables spec-decode for it; unresolved upstream. Converting the
  drafter would not fix this.

Ornith 1.5 shares that hybrid layout (35B: 30/10; 9B: 24/8), so the same constraint applies —
which is why MTP, verified above to work, is the route that matters here rather than DSpark.

## Sampling

Vendor sampler, per repo convention (each model gets its own):

- General: `temperature 0.6, top_p 0.95, top_k 20`
- To reproduce published benchmarks: `temperature 1.0`
- Context: 262,144 native; ~1M via YaRN factor 4.0

**Open decision before stage 1.** Qwen3.8-27B ran this exact two-stage shape at temp 1.0 and
produced BF16 23/30 vs Q8_0 20/30, failing at non-overlapping points — n=1 at temp 1.0 could
not separate sampling variance from precision loss, and the 3-point delta was uninterpretable.
Running Ornith 1.5 the same way reproduces the same ambiguity across four more rungs. Resolve
by either establishing a variance floor (n=2 at one precision) or pinning a lower temperature
and accepting an off-vendor sampler. Decide before the first run, not after seeing a score.
