---
name: laguna-s-2.1-hf-Q4_K_M
created: 2026-08-04
model: Laguna S 2.1 (Poolside, arch laguna, HuggingFace GGUF — file named `-Q4_K_M` but actually MOSTLY_Q8_0 mixed-precision, 89.4 GiB single shard, 814 tensors; 118B total / ~8B active, 256 experts / 10 used + 1 shared, interleaved SWA-512 + global attention; local on Apple M5 Max 128 GiB via self-built llama.cpp llama-server; driven via opencode)
stage: wired
score:
---

# Run — laguna-s-2.1-hf-Q4_K_M

## Wire

**Status: wired, loaded and probed — all four gates passed.** This is a **second quantization of a
model already in the field**, not a new model: same architecture, same 814 tensors, same expert
config as [`laguna-s-2.1`](laguna-s-2.1.md). That record carries the full architecture, tokenizer
and lineage analysis and is not repeated here. What follows is only what differs.

**The run name is wrong and the file name is the reason.** The GGUF is named
`laguna-s-2.1-Q4_K_M.gguf`, but its own header says otherwise:

| | LM Studio build (`laguna-s-2.1`) | **This file** |
|---|---|---|
| `general.file_type` | 15 (**MOSTLY_Q4_K_M**) | **7 (MOSTLY_Q8_0)** |
| Size on disk | 71,163,115,283 B (66.28 GiB), 2 shards | 96,031,829,760 B (**89.4 GiB**), 1 shard |
| `laguna.context_length` | 262,144 | **1,048,576** |
| `general.finetune` | — | `hf` |
| Tensors | 814 | 814 |

Tensor-level quantization mix, read from the header: **Q8_0 386 · F32 287 · Q4_K 117 · BF16 24.**
So it is Q8_0-dominant with a Q4_K minority — genuinely mixed precision, and roughly 8-bit where the
cohort's existing laguna run is 4-bit. The `-Q4_K_M` in the filename is simply incorrect; `hf` in
the run name is the operator's own source tag (HuggingFace), not a quantization claim.
**Run name retained for now at the operator's call (2026-08-04); revisit before publishing any
cross-run table, since as it stands the label asserts the opposite of the file's most important
property.** Branch named `laguna-s-2.1-hf-q8_0-mixed` to carry the correct fact meanwhile.

**Why this run exists, and why it is worth the machine time.** The `laguna-s-2.1` record discloses a
confound it could not resolve: *"This is the first Q4_K_M submission in the field… any weakness
laguna shows is therefore confounded with 4-bit quantization and must not be attributed to the
architecture or the training without that caveat. No F16 GGUF of this model is available locally, so
the confound cannot be eliminated within this run."* This variant largely eliminates it — same
weights, same architecture, ~8-bit instead of ~4-bit. It is **the cleanest quantization A/B
available for any model in this eval**, and it is why a five-round arc on the 4-bit build can now be
read against a higher-precision control.

**Provenance — partially open.** Both files came from the same HuggingFace commit
`fc4e481289523cf7d0df668da6d1d391616141ca` (blob `a8b55c75…` for the weights, `2ee8aa30…` for the
DFlash draft). The download metadata records no repository name, and the repo is **not yet
recorded** — see Open below.

**Harness — this run changes the runtime, and that is a comparability break.** Every prior cohort run
was driven through LM Studio (llama.cpp 2.27.1 build). This one runs a **self-built
`llama.cpp/build/bin/llama-server`** directly on the same port and OpenAI-compatible path, so
opencode needed **no configuration change**: the existing `lmstudio` provider
(`baseURL http://localhost:1234/v1`, model id `poolside/laguna-s-2.1`, temp 0.6 / top_p 0.95 /
top_k 20) routes straight through, and llama.cpp ignores the `model` field. Server args as run:

```
--jinja --reasoning-preserve -fa on
-c 262144 -b 4096 -ub 2048
--cache-type-k q8_0 --cache-type-v q8_0
--cache-reuse 256 --port 1234 -lv 4 -np 1
```

**Three deliberate divergences from the cohort baseline, recorded so no result is silently
attributed to the model:**

1. **`--reasoning-preserve` is ON.** Every prior cohort run — all five laguna rounds included — ran
   with reasoning preservation **off**. This is the `preserveThinking` variable the laguna-s-2.1
   record pre-registered as its next experiment. It is now moving *simultaneously* with the
   quantization change, so **this run cannot serve as the clean single-variable A/B for either
   one.** If reasoning preservation is the question, it still needs its own run.
2. **`--cache-type-k q8_0 --cache-type-v q8_0`.** The prior run pinned KV to F16 specifically to
   avoid "a second quality confound on top of Q4 weights". Here it is quantized. The operator
   weighed this knowingly: F16 KV at 262,144 costs ~12.0 GiB against q8_0's ~6.4 GiB, and with
   reasoning preservation inflating context every turn, halving the window to afford F16 risked a
   truncated run — judged worse than a nameable KV confound. **A confound, not a defect.**
3. **Runtime and build changed** (LM Studio 2.27.1 → self-built llama-server), with `-np 1` matching
   the cohort's PARALLEL 1.

**Memory: measured RSS 94.8 GiB**, against ~95.8 GiB predicted (89.4 weights + ~6.4 KV for the 12
full-attention layers at q8_0). Holds comfortably without `--mlock`. An earlier attempt with
`--no-mmap --mlock` **was OOM-killed**: that pair loads all 89.4 GiB into anonymous memory and pins
it, leaving no headroom for compute buffers on a 128 GiB machine. Dropping both lets the kernel
reclaim clean file-backed pages; on Apple Silicon's unified memory there is no copy cost. The model
declares a 1,048,576 window, so 262,144 is a deliberate cohort-matching choice, not a ceiling.

**Speculative decoding — tried, and it is a large net negative on this hardware.** An initial
configuration added the shipped DFlash draft
(`-md laguna-s-2.1-DFlash-BF16.gguf --spec-type draft-dflash --spec-draft-n-max 15`, block_size 16,
mask_token_id 12, n_extract 6). Measured **~27 t/s → ~3 t/s, a ~9× slowdown** — consistent with
near-total draft rejection making every drafted block wasted compute. **Removed.** Recorded because
it is a real and reusable finding about this draft on this architecture, not a configuration error.

**`--cache-reuse 256` is inert.** The server rejects it at startup (`cache_reuse is not supported by
this context, it will be disabled`) — incompatible with this model's 36 sliding-window layers. Left
in the command line; it does nothing.

**Live wire verification (2026-08-04) — all four gates passed.**

- **Probe A — plain completion: PASS.** Exactly `WIRED`, `finish_reason: stop`, 4 completion tokens.
  The startup warnings `special_eos_id is not in special_eog_ids` /
  `special_eot_id is not in special_eog_ids` are **benign** — generation terminates cleanly. They
  were a plausible cause of unbounded generation and are ruled out.
- **Probe B — tool call: PASS, and this was the gate.** `finish_reason: tool_calls` with a
  well-formed `{"command":"ls /etc"}`. **This llama.cpp build parses the GLM-style
  `<tool_call>NAME<arg_key>…<arg_value>…</tool_call>` form into OpenAI `tool_calls`** under
  `--jinja`, so the agentic loop is viable.
- **Probe C — end-to-end through opencode: PASS.** `opencode run -m lmstudio/poolside/laguna-s-2.1`
  drove a real Write → Read sequence with genuine tool parts in the session store. Run from a
  scratch directory; the eval workspace is untouched.
- **Reasoning: ON — 3,382 chars of `reasoning_content`.**

**`check-thinking.sh` had to be fixed first, and the failure mode is worth knowing.** The probe read
only `usage.completion_tokens_details.reasoning_tokens`, which LM Studio populates and llama-server
**does not emit at all**. Against this server it printed `THINKING OFF  <-- do not start the run`
while the model was demonstrably reasoning. That is the same accounting artifact already recorded for
opencode's `tokens_reasoning`, in a new place — and this one fails toward *blocking a valid run*
rather than a silent misread. It now checks `reasoning_content` too and reports both; verified ON
against the live server and OFF against a synthetic no-reasoning payload. Fixed at `a0547c4`.

**Telemetry note.** The operator reports llama-server's logs (`-lv 4`) substantially more informative
than LM Studio's for this model. Relevant to method: LM Studio's per-token `print_timing` lines are
what made the round-5 runaway generations diagnosable at all, and this build exposes more.

**Workspace.** `laguna-s-2.1-hf-Q4_K_M/` at the repo root, seeded per cohort convention with
`govmomi-cli-eval-prompt.md` (**verified byte-identical to the repo-root baseline by `diff`**) and
`govmomi-cli-audit-prompt.md`. **No submission present yet.** Branch `laguna-s-2.1-hf-q8_0-mixed`.

**Workspace hygiene — deliberate, and different from the 4-bit run.** This workspace holds *only* the
two prompts. The `laguna-s-2.1` workspace accumulated seven audit documents, which was intentional
ecological validity there — but that arc ended with the auditor committing a full rescore into the
workspace immediately before a round, which the model then located by name and read, voiding it (see
that record's round-5 disclosure). **For this run, no `REVIEW*`, `HITLIST*` or `REMEDIATION*`
document should sit in the workspace while a round targeting it is pending.**

**Open — fill before the audit:**
1. **HuggingFace repository name** — only commit `fc4e481…` is recorded.
2. Submission contents, `go build` / `go vet` on arrival, git-history forensic availability,
   wall-clock, tool-call count, and any stalls or operator interventions.
3. Whether the runaway-generation behaviour seen under LM Studio with reasoning preservation recurs
   here. Watch for any single task exceeding ~3k decoded tokens against a 100–1,200 token norm.

## Audit

## Score

## Compare

## Remediate

## Rescore
