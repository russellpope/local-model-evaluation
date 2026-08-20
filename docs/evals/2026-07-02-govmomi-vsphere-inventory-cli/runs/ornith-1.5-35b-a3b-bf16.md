---
name: ornith-1.5-35b-a3b-bf16
created: 2026-08-20
model: Ornith-1.5-35B-A3B (local, BF16 GGUF)
stage: wired
score:
---

# Run — ornith-1.5-35b-a3b-bf16

## Wire

**Status: pre-registration.** `ornith-1.5-35b-a3b-bf16/` is seeded with the eval prompt
only; no submission exists yet. The rubric (`govmomi-cli-audit-prompt.md`) is **not** staged
in the workspace and must never be.

First rung of a four-rung arc agreed 2026-08-20: 35B BF16 → 35B Q8_0 → 9B BF16 → 9B Q8_0,
each a full eval, then MTP throughput testing across Qwen 3.8 and Ornith.

### Weights

`/Users/ldh/models/Ornith-1.5-35B-A3B-GGUF/Ornith-1.5-35B-BF16.gguf` — 71,066,994,240 bytes,
size-verified against the CDN `x-linked-size` on download. First-party
`ornith-ai/Ornith-1.5-35B-A3B-GGUF`; no third-party requant. Provenance, full manifest and
the MTP findings: [`../artifacts/ornith-1.5-download-manifest.md`](../artifacts/ornith-1.5-download-manifest.md).

Model released 2026-08-19, MIT. 35.51 B params, 256 experts, 8 active per token. Hybrid
backbone — 30 `linear_attention` / 10 `full_attention` layers. Multimodal (`mmproj` pulled
but unused; this eval is text-only).

### Endpoint

llama.cpp `llama-server`, build **10470** (`34af94cd9`, Homebrew, installed 2026-08-17).

```
llama-server -m .../Ornith-1.5-35B-BF16.gguf \
  --alias ornith-1.5-35b-a3b-bf16 \
  -c 262144 -ngl 99 -fa on --jinja \
  --temp 0.6 --top-p 0.95 --top-k 20 \
  --host 127.0.0.1 --port 1234 -v
```

Driven from opencode via the `llamacpp-local` provider (`http://localhost:1234/v1`), model
id `ornith-1.5-35b-a3b-bf16`. Verbose server log: `/Users/ldh/models/logs/ornith-1.5-35b-a3b-bf16.log`.

LM Studio previously held port 1234 (serving `gemma-4-12b` + an embedding model) and was
stopped by the operator before this server was started.

### Sampler — matches Ornith 1.0 exactly

`temperature 0.6, top_p 0.95, top_k 20`, context 262,144. This is simultaneously the vendor
recommendation on the 1.5 model card for general tasks, and byte-identical to what every
Ornith 1.0 run used, so the 1.0 → 1.5 comparison holds. Corroborated three ways:
`ornith-1.0-35b-fp16/notes.txt:39`, `docs/handoffs/2026-07-08-ornith-397b-fp8-experiment.md:89`,
`docs/handoffs/2026-06-28-ornith-remediation-round2-rescore.md:121`.

Recorded inline here deliberately — eight other run records carry their sampler, but neither
Ornith 1.0 record does, and it only survived in handoffs and `notes.txt`.

The card's `temperature 1.0` is its *benchmark-reproduction* setting and was **not** used;
this field measures practical coding capability, not benchmark reproduction.

### Run condition — compaction is charged to the model, by design

Runs go straight through with no operator intervention. If the model spawns a subagent to
conserve context, that is its own call and counts; if it sprawls and compacts, that counts
too. Context management is treated as part of coding capability rather than as a nuisance
variable to control, and it is the only protocol that stays consistent without babysitting
or maintaining a prompt library. Prior runs have reached ~240k tokens without compacting.

Consequence to carry into scoring: run-to-run spread is roughly **3 points** — Qwen3.8-27B
Q8_0 scored 20 with one compaction at 230,641 tokens and 23 without (second sample recorded
in the `deepthought` repo under the spine-harness work, not here). **A precision effect
smaller than 3 points is therefore not detectable at n=1.** The BF16-vs-Q8 question is
answered across this arc by looking for a consistent direction over two independent
contrasts (35B and 9B), not by any single pair.

### Load verified against the live server

- `general.architecture = qwen35moe`, `n_ctx = n_ctx_seq = 262144`, `n_ctx_train = 262144`
- `model params = 35.51 B`; resident **69.9 GiB** on a 128 GiB M5 Max
- `qwen35moe.nextn_predict_layers = 1`, with `blk.40.nextn.*` tensors present and logged as
  `unused … ignoring` — expected, since `--spec-type draft-mtp` is deliberately **not** set
  for scoring runs. This is the MTP head confirmed in the live artifact rather than inferred
  from the HF config.
- Generation: returns `OK` on an exact-reply probe, `finish_reason: stop`
- Reasoning parser: `<think>` content is split into `reasoning_content`, leaving `content`
  clean. A first probe at `max_tokens=24` returned empty `content` with 24 completion
  tokens — truncation inside the think block, not model collapse. Worth knowing before
  reading any short reply as a failure.
- Tool calling under `--jinja`: emits a well-formed `tool_calls` entry with valid JSON
  arguments, `finish_reason: tool_calls`. This is the capability the eval actually depends
  on and it was verified before the run, not assumed.
- Prompt cache active (`cached_tokens` non-zero on the second probe).

Build of the submission tree: **not applicable yet** — no submission. To be recorded when
the run lands.

## Audit

## Score

## Compare

## Remediate

## Rescore
