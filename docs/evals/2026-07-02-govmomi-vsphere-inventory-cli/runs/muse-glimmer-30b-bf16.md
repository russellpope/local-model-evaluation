---
name: muse-glimmer-30b-bf16
created: 2026-08-11
model: Muse Glimmer 30B (Meta, released 2026-08-10, Apache 2.0 — dense causal transformer, ~29.6B total incl. ~1.8B ViT-G/14 perception encoder, ~27.9B language model; 52 layers, alternating SWA-2048 / global attention; 131,072+ context. Run at **BF16, native precision** from `unsloth/Muse-Glimmer-30B-GGUF` (2 shards, 55,725,511,168 B = 51.9 GiB). Local on Apple M5 Max 128 GiB via self-built llama.cpp llama-server; driven via opencode)
stage: audited
score: 14 / 30
---

# Run — muse-glimmer-30b-bf16

## Wire

**Status: audited 2026-08-12 — FAIL 14/30.** Submission frozen at `b9c9f6d` before the audit. Directory was seeded with the eval
prompt only. The audit prompt is deliberately **not** present in the workspace — it is dropped in at
audit time, never before. Measured wire details are in *Wired* below; the text above it is the
pre-registration, left as written before anything ran.

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
   [#26841](https://github.com/ggml-org/llama.cpp/pull/26841). Record the build tag actually run,
   and prove support by loading the arch — a version comparison is not evidence.
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

### Wired — measured 2026-08-12, all gates green

Server: Homebrew `llama-server` **build 10360 (48d22e295)**, upstream — *not* the
`poolsideai/llama.cpp` fork at build 10010 that every laguna number came from. That fork exists to
add Laguna support and stays untouched so those runs remain reproducible.

```
--jinja -fa on -ngl 999 -np 1 -c 131072 -b 4096 -ub 2048
--temp 1.0 --top-p 0.95 --top-k 64 --reasoning-preserve -lv 4 --log-timestamps
```

Both shards auto-load from shard 1 (`split.count = 2`, 731 tensors). Model reports
`27.85 B params`, `n_expert = 0` (**dense**), `n_ctx_train = 131072`, `n_head_kv = 2` (16:1 GQA),
`n_swa = 2048` with `sliding_window_pattern = 4` → 13 full-attention layers, 39 SWA.

**Memory, measured:** 50,566 MiB model + 1,820 MiB KV (1,664 full-attention + 156 SWA, both F16) +
1,100 MiB compute = **53,486 MiB, with 56,613 MiB free.** F16 KV at full context costs under 2 GiB
here, so the `q8_0` KV compromise the laguna arc was forced into never arises — no quantization
anywhere in this stack.

Gates: arch loads live (not inferred from build number); template accepted; **two-turn tool
round-trip returns `finish_reason=stop`** — the `-inf` EOG logit bias is benign and the Harmony
recipient syntax (`<|start|>assistant to=user<|message|>`) parses correctly; single slot, so `-c` is
the true per-request context; thinking on (6,039 chars `reasoning_content`, `reasoning_tokens: 0` —
the llama-server accounting artifact `a0547c4` exists for).

`Reasoning strength: high` is the **chat template's own default**, visible in the rendered
`example_format` — not an auditor injection. Recorded as such.

**Deviation from the card, recorded not corrected:** llama-server applies a default `min_p = 0.05`
on top of the card's documented `temp 1.0 / top_p 0.95 / top_k 64`. Left as-is for rung 1; whatever
it is, it must be identical on all three rungs.

Harness: opencode provider `llamacpp-local` → `http://localhost:1234/v1`, model id
`muse-glimmer-30b-bf16` (matches `--alias`), limits `context 131072 / output 32768`. Deliberately a
new provider rather than an entry under the `lmstudio` one, whose defaults are laguna-shaped
(262144 context, temp 0.6 / top_k 20) and would have silently misconfigured this model.

### Pre-registered throughput predictions (recorded before measurement)

Decode is memory-bandwidth-bound and this model is **dense**, so every token reads the entire weight
set. That is the whole explanation for it feeling slow next to laguna, which is 118B total but only
**~8B active** per token — an MoE reads one expert set, not the model. Same machine, opposite
sparsity, ~4× the bytes per token.

| Rung | Bytes read/token | Predicted |
|---|---|---|
| BF16 | ~55.7 GB | 8–10 t/s |
| Q8_0 | ~29.6 GB | 18–19 t/s |
| UD-Q4_K_XL | ~15.9 GB | 33–35 t/s |

If these hold, **the precision ladder is also a speed ladder**, and the deployable configuration is
whichever rung holds its score at usable speed — not necessarily the most accurate one. That is the
adoption question, and it is not answerable from the BF16 run alone.

This also predicts DFlash should pay off *unusually well here*: speculative decoding amortises one
weight read across a whole accepted block, which is exactly the bottleneck a dense BF16 model has
and a sparse MoE does not. Pre-registered as an arm on rung 1 after the baseline scores, testing two
claims at once — Meta's "identical output quality", and whether it makes BF16 usable. If it is
lossless and fast it becomes an adoption fact, not a ladder variable, and the ladder stays measured
with DFlash off.

## Audit

Full report: [`artifacts/muse-glimmer-30b-bf16-REVIEW-baseline.md`](../artifacts/muse-glimmer-30b-bf16-REVIEW-baseline.md).
Submission frozen at `b9c9f6d` **before** the audit began, so the artifact is re-derivable from git alone.

Four independent passes: a blind adversarial auditor (walled off from the scratchpad, all run records,
every `REVIEW*`/`FINDINGS*`/`EVIDENCE*` file and every other model's submission), a claims reviewer
working from the opencode transcript, a 14-mutation battery with negative controls at both ends, and a
ground-truth pass. The three Criticals were each reached by more than one pass independently.

**Ground truth was re-established before anything was charged.** The submission pins govmomi
**v0.52.0**; every prior run in this field used v0.55.1. Identical probes were run against both in
parallel modules: they agree on every fact, the sole difference in 578 lines of dumped state being
`summary.storage.committed` 233 vs 234 bytes. This prevented two false findings — `memoryMB = 32` and
`committed = 233` bytes mean **`RAM 0.0 GiB` is the *correct* output**, not the unit-scaling defect
first suspected, and any non-zero figure would have been the fabrication.

## Score

**FAIL — 14 / 30.** Accuracy 2 · Integrity 1 · Security 3 · Performance 3 · Concurrency 3 · Quality 2.

Three Criticals: `vswitches --portgroup` is a `return []VMInfo{}, nil` stub (AC6 unimplemented, proven
by a 2-SOAP-call round-trip count against 16 correct rows); its test asks for a *nonexistent* port group
and asserts empty, which only a stub satisfies (the battery proved a correct implementation, the stub,
and a broken mutant all pass identically); and the transport classifier substring-matches the
**datastore name**, never requesting `config.storageDevice` anywhere in the tree (AC4's logic does not
exist).

No dissent on the total — the blind auditor scored 14 independently and the dimension split matched.

**The pattern is more informative than the score.** Almost every value that looks right on vcsim is a
literal or an accident rather than a mechanism: `unknown` transport reached by name-matching, DVS
`Ports: 0` hardcoded while the real per-portgroup `numPorts = 1` sat retrieved-and-discarded, `VLAN "0"`
a default that happens to equal vcsim's real VLAN 0, `LACP: "disabled"` asserted where `N/A` is honest.
The output reads clean and the logic mostly isn't there — so on a live vCenter, which is exactly where
the spec says criteria 4 and 5 are validated, most of those columns would be wrong.

Genuinely good, and worth not losing: `vms` and `datastores` are textbook single-`ContainerView` +
explicit property list, 5 round-trips at any inventory size. AC3 correct. Viper precedence verified live
across all four tiers. `--timeout 1ms` honoured. `-race`, `staticcheck`, `govulncheck` clean. No
fabricated output — 17 claims checked, 12 true, and the pasted samples are byte-identical to real runs.
This submission does not lie in prose; it lies in code.

## Compare

**This is the first unconfounded data point in the field** — no quantization in the weights, none in the
KV cache — and it lands at **14 / 30**, below every local baseline in the cohort except the three
lowest-scoring models.

| Run | Baseline | Final |
|---|---|---|
| `laguna-s-2.1` (Q4_K_M, 118B) | **18** — previously the field's best local baseline | 22 |
| `qwen-3.6-27b` | 16 | 16 |
| `qwen-agentworld-35b-a3b` | 16 | 23 |
| `ornith-1.0-35b-fp16` | 16 | 25 |
| `qwen3.6-35b-a3b-mlx` | 15 | 21 |
| **`muse-glimmer-30b-bf16`** | **14** | — |
| `qwen3-coder-next` | 13 | 13 |

**So quantization was never the explanation.** Both laguna records disclose the 4-bit confound as
unresolvable within their runs; this run resolves it, and the answer is that the asterisk was not
load-bearing. A 4-bit 118B MoE at 18 beats a BF16 dense 30B at 14 on the same task, same rubric, same
auditor.

The model card's own comparators are also not borne out here: it claims to beat Gemma 4 31B and Qwen 3.6
27B across 25+ benchmarks, and it scores below `qwen-3.6-27b`'s 16 baseline on this task. Benchmarks
covering AIME and SWE-Bench Pro do not predict a multi-hour agentic build with mutation-tested ground
truth — which is the reason this eval exists.

**Pre-registered cull thresholds — none met.** Baseline > 18 was "earns its slot", ≥ 22 "retire gemma
and the qwens", ≥ 25 "retire agentworld". At 14 the retirement decision is off, and the thresholds were
recorded before the run precisely so this could not be rationalised after the fact.

**Process note, recorded against the operator not the model:** a `/tmp` access permission prompt sat
unanswered for **2 h 22 m** mid-run. Active time was **~4 h 20 m** (10:59 → 15:19) against ~6 h 42 m
wall clock. The same wall-clock/active-time distinction that had to be corrected for laguna round 3.

Not yet run, both pre-registered and unaffected by this result: the Q8_0 and UD-Q4_K_XL rungs, and the
DFlash / `-np` concurrency arms.

## Remediate

## Rescore
