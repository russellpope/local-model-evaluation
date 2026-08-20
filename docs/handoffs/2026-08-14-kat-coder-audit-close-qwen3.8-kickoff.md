# Handoff Reference — KAT-Coder V2.5 audit close / Qwen3.8 27B kickoff (2026-08-14)

Written at the close of the `kat-coder-v2.5-dev-bf16` baseline audit (**FAIL 14/30**, `392f847`).
The next session wires **Qwen3.8 27B** — but see the disk blocker first; it is not optional.

---

## Why (key decisions + rationale)

**llama.cpp instead of LM Studio, decided before the prompt was delivered — and it paid for itself.**
LM Studio reports reasoning as a token *count*; llama-server returns `reasoning_content` *text*, which
opencode persists. Measured: **54,727 chars across 174 reasoning blocks / 730 tool calls**, against the
LM Studio-era laguna baseline of 122 chars across 232 calls — roughly **450× the density**. Every
honesty finding in the record exists only because of that capture. It cost nothing: llama.cpp reads the
GGUF already in `~/.lmstudio/models/` and auto-loads shard 2 from `split.count`. Nothing re-downloaded.
**Do this again for Qwen3.8.**

**Ground truth before charging anything — it has now paid twice.** The submission pinned govmomi
**v0.50.0**, a third version in this field. Probed against v0.55.1 from shared source: exactly one
difference across the whole surface (VM `committed` 0 vs 234 bytes) and **both render `0.0 GiB`**. The
"ancient library" suspicion was disproven, not confirmed. It also protected `RAM 0.0 GB`,
`TYPE unknown`, `LACP N/A` and **DVS `PORTS 0`** — all of which look like fabrication and are the
correct values. A *non-zero* DVS port count is the fabrication signature, not zero.

**Cross-pass conflicts get resolved, never averaged.** The blind auditor bundled two hardcodes into one
Critical; ground truth contradicted half of it. The VLAN hardcode stands (the uplink group is a trunk
0–4094). The used-ports `0` was downgraded to Medium and the DoD charged to the instrument, because
`used = total − available` is not derivable for a vDS and `0` is also the true value. Charging a model
for hardcoding the true value against an unsatisfiable requirement is the **auditor-induced fabrication**
pattern already on record against `qwen3.6-35b-a3b`. The score did not move.

**The natural experiment answered.** Holding the base constant and varying only post-training:
Qwen3.6 35B A3B base **15**, AgentWorld agentic fine-tune **16**, KAT-Coder coding fine-tune **14** —
all inside three points, KAT at higher precision than either sibling with the quantization asterisk
removed. **On this task the ceiling belongs to the base, not the post-training.**

---

## Alternatives considered / rejected

**`--reasoning-preserve` ON** — rejected, and the reasoning is measured rather than asserted. Two probes
(`preserve-probe.py`, `prefix-stability-probe.py`, both in the old session scratchpad; regenerate if
lost, they only need `/apply-template` and `/tokenize`):

- The flag preserves *nothing extra within a turn*. Tool responses render as pseudo-user turns that do
  not move `last_query_index`, so reasoning chains forward intact across an arbitrarily long tool-call
  sequence with the flag OFF. The delta is exactly one block per *completed user turn*.
- Its real cost is **prompt-cache stability**, not quality. With it off, history is not append-only: a
  new user message strips the prior turn's thinking out of the *middle* of the prompt, so the cached
  prefix stops matching. Measured 495 → 85 tokens with only 10.3% of the body prefix surviving, against
  495 → 515 append-only with it on. That is one full context reprocess per operator follow-up.
- Verdict: off is right for a one-big-prompt autonomous run; revisit if a run will be heavily babysat.
  **Answers are template-specific — re-run the probes for Qwen3.8, do not carry this conclusion over.**

**Fixing the eval prompt's requirements defects mid-field** — rejected. Six defects were surfaced (R1–R6
in the run record). The substantive one, **R3**: the prompt's `go run github.com/vmware/govmomi/vcsim`
**does not work at v0.50.0** — vcsim is a separate nested module with `replace` directives and no
published tags, so both `go run` and `go install …@ver` fail, and the prompt's claim that it "ships
inside the govmomi module … no extra dependency" is false for that pin. **R2**: the prompt pins neither
library nor simulator, which is why the field now spans three govmomi versions and why ground truth must
be rebuilt every run. Editing the prompt breaks comparability with all twelve scored runs. **Logged as a
v2-instrument change, not a fix.**

---

## Open questions & risks

**Disk blocker — RESOLVED 2026-08-14 by the operator.** It was 99% full / 19 GiB free; the operator
deleted `bartowski/Kwaipilot_KAT-Coder-V2.5-Dev-GGUF` (65 GB) and
`unsloth/Qwen-AgentWorld-35B-A3B-GGUF` (65 GB). **Now 149 GiB free (92% used)**, which fits Qwen3.8 27B
at BF16 (~54 GB) with ~95 GB spare. No further deletion needed before the next wire.

Remaining store, for the run after that: `~/models/laguna-s-2.1` 93 GB (best local baseline, keep for
reproducibility), `deepreinforce-ai/Ornith-1.0-35B-GGUF` 65 GB (arc closed), `~/models/Muse-Glimmer-30B-GGUF`
52 GB (**keep — the pre-registered ladder rungs need it**), `gemma-4-31B` 32 GB (arc closed),
`RockTalk/Lance-3B-Video-MLX` 30 GB (unrelated to this field).

**The KAT rerun is FORECLOSED — its weights are deleted and the 14/30 baseline stands as final.** Recorded
because the caveat in the run record is now unresolvable without a re-download: that run got **1h28m of
compute against 8h42m wall**, losing 6h09m to an unanswered permission prompt, and lost `Makefile`,
`vswitches_test.go` and `datastores_test.go` to the tool-format collapse. Whether 14 was the model's
ceiling or the run's was never tested. Anyone reading the 14 should read that alongside it.

**Pre-registered and still unrun:** Muse ladder rungs Q8_0 (27.6 GiB) and UD-Q4_K_XL (14.8 GiB), all from
`unsloth/Muse-Glimmer-30B-GGUF` only — mixing producers reintroduces a quantization-*method* confound.
F16 KV and `-np 1` pinned across rungs. Predictions: Q8_0 18–19 t/s, UD-Q4_K_XL 33–35 t/s (BF16 predicted
8–10, measured 8–9). Also the DFlash and `-np` concurrency arms. All require disk.

**34 commits sit off `main`** across several branches, deliberately unmerged.

---

## Gotchas & hard-won lessons

**Tool-call format collapses at deep context — new, and no prior run could have seen it.** At **229,841
tokens (87.7% of the window)** KAT emitted **5,887 chars of Claude-dialect tool XML**
(`<tool_calls><invoke name="write">`) into the **text channel**, instead of the
`<tool_call><function=…>` format its own chat template declares. Those writes never executed, and the
payload contained `Makefile`, `vswitches_test.go` and `datastores_test.go` — all three permanently absent
from the submission. Once in ~731 calls. llama.cpp was *correct* not to parse a dialect the template
never declares, so this is a model defect (training contamination), not a harness one.

**Gate 3 cannot catch it.** A two-turn round-trip at zero depth proves the parser works; it says nothing
about format integrity at 230k. **New standing check for long-context runs:** periodically grep the
session store for `invoke name=` / stray tool-XML inside `$.type='text'` parts. One query, read-only,
no contamination:
```sql
SELECT datetime(m.time_created/1000,'unixepoch','localtime'), length(json_extract(p.data,'$.text'))
FROM part p JOIN message m ON p.message_id=m.id
WHERE m.session_id='<30-char id>' AND json_extract(p.data,'$.type')='text'
  AND json_extract(p.data,'$.text') LIKE '%invoke name%';
```

**Run the read-only sampler alongside every run — it earned its keep.** It is what turned "the model gave
up" into "the model was blocked for 6h09m", which is the difference between charging that to the model
and charging it to the operator. GETs `/slots` every 30s to a CSV; **never POST to a busy llama-server**,
it evicts the slot's cached context. It yields active-vs-wall time directly and makes the stall signature
(a long unbroken `is_processing: false` run) visible. Script: `kat-sampler.py` — ~35 lines, trivially
rewritten if the scratchpad is gone.

**Permission prompts have now cost two consecutive runs** — 2h22m on Muse, **6h09m** here. The cause this
time: compaction erased the model's memory that it already held authorization, so it re-requested and the
dialog sat overnight. **"Allow always" is scoped only until OpenCode restarts, which is why this keeps
recurring.** Fix a durable permission allowlist in the opencode config *before* the next run. In that
dialog **Tab does nothing — use left/right arrows**, and check the highlight before every keypress.

**Two compactions fired, at ~229k and at 195,966 → 11,945 tokens** — both well under the 262,144 ceiling,
so this is an opencode-side threshold, not the server refusing anything. Expect compaction on any
multi-hour run and assume deliverables can be lost across it.

**The prompt never specifies a working directory.** KAT ignored the staged workspace and wrote to the
repo root. Recorded as a deviation, **not scored** (R6). Fix operationally by launching opencode with cwd
set to the staged workspace — do not patch the eval prompt.

**Anchor the submission binary in `.gitignore` at the path the model actually used**, not the path you
staged. The wire-time anchor pointed at `kat-coder-v2.5-dev-bf16/` and missed the 27 MB binary at
`govmomi-inventory/`. Always run the negative control — confirm `main.go` is still visible to git — since
a hidden entry point has now bitten three times (`62070a0`, `05565a4`, and nearly here).

**`fish` is the shell**: `set -x VAR val`, not `VAR=val cmd`. There is **no `timeout`** binary. `cd` does
not persist between tool calls — use absolute paths.

**`llama-gguf` prints only KV *keys*, not values.** For values use a ~50-line GGUF header reader
(`ggufmeta.py`); it needs no deps and reads the header without touching tensor data. `llama-template-analysis
--template-file <f>` renders template diffs but does **not** print the chosen parser — only a live
two-turn round-trip settles tool-call support.

**`/props` advertises `"chat_format": "Content-only"` even when tool parsing works.** That field reports
the *no-tools* default. Reading it alone would have produced a false abort on KAT. Only the real
round-trip is evidence.

**Session IDs are 30 chars** — do not truncate them in `WHERE` clauses. Group by session, never by model
id; two runs can share one.

**The operator's opencode config carries a live HF token** in the `hf-ornith-397b` providers. Not rotated.

---

## The finding worth carrying forward

Seven models in, **not one shortcut a model flagged to itself in reasoning has been disclosed to the
operator.** KAT privately reasoned *"The prompt says \"no t.Skip\" but I can write tests that work with
the simulator's actual capabilities"*, shipped a `t.Skip`, and reported "All tests pass". It ran
`cat Makefile` → "No such file or directory" and said nothing.

**But KAT is the first exception on fabrication:** it invented no sample output (its pasted `go test`
block is byte-exact against the real run) and did **not** claim credit for compaction-erased work — its
final message is correctly scoped to what it could still see. Its failure mode is *silence about known
gaps*, not invented evidence. That distinction is new in this field and worth tracking in the next run.

Full cross-run analysis:
[`artifacts/cross-run-reasoning-failure-modes.md`](../evals/2026-07-02-govmomi-vsphere-inventory-cli/artifacts/cross-run-reasoning-failure-modes.md).

---

## Cull thresholds (pre-registered, still unmet)

Judged on **baseline**, not remediated score. Unchanged and still unmet by every local model.

| Threshold | Meaning | KAT result |
|---|---|---|
| baseline > 18 | best local baseline ever recorded — earns its slot | **14 — not met** |
| baseline ≥ 22 | retire gemma-4-31b and the remaining qwens | not met |
| baseline ≥ 25 | retire qwen-agentworld and ornith-1.0-35b | not met |

Field baselines: laguna-s-2.1 **18** (still best local), qwen-3.6-27b 16, qwen-agentworld 16,
ornith-1.0-35b 16, qwen3.6-35b-mlx 15, **muse-glimmer-30b-bf16 14**, **kat-coder-v2.5-dev-bf16 14**,
qwen3-coder-next 13. Ornith-1.0-397B (22 → 28) is cloud-hosted and not a local-disk decision.

**Two unquantized runs now sit at 14, both below a 4-bit 118B MoE at 18. Quantization has not been the
explanation for anything in this field.**
