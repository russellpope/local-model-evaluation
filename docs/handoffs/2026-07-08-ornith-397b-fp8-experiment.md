# Handoff Reference — Ornith-1.0-397B eval + FP8 quant experiment (2026-07-08)

> **Net state (2026-07-08, session close):**
> - **bf16: DONE — 28/30, committed** (branch `ornith-397b-remediation-pass1`, unpushed). ✅
> - **FP8: BLOCKED** — `qwen3_5_moe` is too new for HF's FP8 serving path on *both* engines. Not a
>   model problem, a tooling problem. Honest fallback: the bf16 **28/30 stands**; FP8 is noted as
>   "blocked as of 2026-07-08."
> - **FP8 on HOLD** (user's call, 2026-07-08) — not shelved, just parked while the user sorts
>   things out. No path chosen yet (H200 vLLM retry / newer SGLang tag / custom container).
> - **Both failed FP8 endpoints (`-dhv`, `-icb`) LEFT UP intentionally** — user chose not to
>   delete them. They're `failed` (not running GPUs, so harmless). **Do NOT delete without asking.**

## Why (the goal)
Evaluate **deepreinforce-ai/Ornith-1.0-397B** (a 397B Qwen3.5-MoE hybrid model) on the repo's
govmomi-CLI agentic eval, via a Hugging Face Inference Endpoint wired into **opencode**.
- **bf16 is DONE** (2026-07-07): baseline PASS WITH CONCERNS 22/30 → one self-prompted
  remediation round → **28/30**, the highest non-reference score in the field, with **no
  relocated cheat**. Committed on branch `ornith-397b-remediation-pass1`; README + run record
  updated.
- **FP8 is BLOCKED** (2026-07-08): a quantization-sensitivity experiment (does the author's FP8
  quant `deepreinforce-ai/Ornith-1.0-397B-FP8` preserve the 28/30 honesty/capability?) that
  **can't run — neither HF engine serves this brand-new arch.** vLLM 0.23.0 (=`latest`) supports
  `qwen3_5_moe` but hits a CUTLASS FP8 `weight_loader` kernel bug on the FP8 checkpoint; SGLang
  v0.5.8 doesn't recognize `qwen3_5_moe` (bundled transformers too old → `KeyError`). Endpoints
  `ornith-1-0-397b-fp8-dhv` (vLLM) and `ornith-1-0-397b-fp8-icb` (SGLang) are both `failed` (left
  up for now — user's call 2026-07-08; **don't delete without asking**). See the **UPDATE**
  section below for the 4 remaining options.

## bf16 result (COMPLETE — do not redo)
- Branch `ornith-397b-remediation-pass1`, HEAD `937047a`. Four commits: baseline `a32e699`
  → audit `792063c` → remediation `6fbaa35` → re-audit `937047a`.
- Run record: `docs/evals/2026-07-02-govmomi-vsphere-inventory-cli/runs/ornith-1.0-397B.md`
  (`stage: rescored`, `score: 28 / 30`). Raw reports: `ornith-1.0-397B/REVIEW.md` +
  `REVIEW-remediated-r1.md`. README updated (results table, scorecard, metrics, per-model
  writeup, remediation-experiment section, Takeaways nuance).
- Residuals in the 28/30: `inferHBAType` over-classifies `naa.`/`t10.`→FC (untested heuristic);
  H3 test is subset+non-empty not bidirectional-exact; `make verify` hardcodes `DC0_DVPG0`
  (model got it to complete in its own env; times out in the audit sandbox — env, not code).
- **Branch is UNPUSHED (42 ahead of origin/main). Push only when the user asks.**

## FP8 saga (the blocker — READ THIS before touching the FP8 endpoint)
The FP8 model would not serve under vLLM. Chronology:
1. **vLLM attempt** (endpoint `ornith-1-0-397b-fp8-dhv`, 8×RTX PRO 6000 Blackwell, `vllm/vllm-openai:v0.23.0`)
   → **FAILED**. Root cause (from container logs): vLLM selects
   `CutlassFP8ScaledMMLinearKernel` for the FP8 W8A8 linear layers, and its
   `process_weights_after_loading` (`kernels/linear/scaled_mm/cutlass.py:228`) re-registers a
   tensor attr → `AssertionError: Overwriting existing tensor attribute: weight_loader`.
   Weights loaded fine (122/122 shards); it's a **vLLM code bug in the FP8 kernel**, not config.
2. **`vllm/vllm-openai:latest` == v0.23.0** (user verified) → bumping the image does NOT help;
   the bug is in the current release. No newer vLLM image available on HF.
3. **An H200 swap would NOT dodge it** — corrected an earlier over-claim: CUTLASS FP8 scaled-MM
   is the *standard* FP8 path on Hopper too, so the same kernel bug would likely hit. Hardware
   swaps don't fix a vLLM kernel-code bug.
4. Also hit a transient **"Scheduling failure: unable to schedule"** on the RTX PRO 6000
   (capacity), which later cleared.
5. **Pivoted to SGLang** (its FP8 loader never touches vLLM's buggy kernel). NEW endpoint
   `ornith-1-0-397b-fp8-icb`, engine `lmsysorg/sglang:v0.5.8`, 8×RTX PRO 6000, URL
   `https://y81h035dzmi2enyy.us-east-2.aws.endpoints.huggingface.cloud`.
6. The new endpoint was created with **vLLM-style args by mistake** (`--enable-auto-tool-choice
   --tool-call-parser qwen3_xml`). Those are invalid for SGLang. **Corrected via API PUT** to
   the SGLang recipe: `--tool-call-parser qwen3_coder --reasoning-parser qwen3` (no
   `--enable-auto-tool-choice`; `qwen3_coder` not `qwen3_xml`). Confirmed applied (http 200).
7. **As of handoff: `initializing`** (SGLang image pulling / weight loading). Background monitor
   `btutiaa5c` (script `scratchpad/monitor_sglang.sh`) is watching for ready-or-crash.

## Open questions this FP8 deploy resolves
- Does **SGLang v0.5.8 natively support Qwen3.5-MoE hybrid GDN** (Gated DeltaNet), or fall back
  to `transformers` (slow / shaky on FP8)?
- Does its **FP8 path work on Blackwell** (sm_120)?
- If it fails specifically on Blackwell-FP8 (not arch support), the next lever is **H200×4 in
  Seoul (~$20/hr, 564 GB — fits the ~400 GB FP8 weights)**, where SGLang FP8 is mature.

## Endpoints & cost (HF Inference Endpoints, namespace `russellcg`)
| endpoint | engine | GPU | $/hr | state |
|---|---|---|---|---|
| `ornith-1-0-397b-fp8-icb` (FP8) | SGLang v0.5.8 | 8×RTX PRO 6000 (Ohio) | $22 | **failed** — `qwen3_5_moe` unrecognized; DELETE it |
| `ornith-1-0-397b-fp8-dhv` (FP8) | vLLM 0.23.0 | 8×RTX PRO 6000 | $22 | **failed** — DELETE it |
| `ornith-1-0-397b-uki` (bf16) | vLLM 0.23.0 | 8×H200 (Seoul) | $40 | paused (the 28/30 run) |
- H200 x4 = $20/hr (Seoul, 564 GB); H200 x8 = $40/hr. `scaleToZeroTimeout` is in **MINUTES** (60 = 1 hour), NOT seconds — a costly early misread.
- Mgmt API: `GET https://api.endpoints.huggingface.cloud/v2/endpoint/russellcg/<name>` (config+state),
  `.../<name>/logs` (container logs), `.../russellcg` (list), `.../v2/provider` (instance catalog + live `pricePerHour`).
- **HF token:** in `~/.config/opencode/opencode.json` → provider `hf-ornith-397b-fp8` →
  `options.apiKey` (short-lived `hf_...`; the user is unconcerned about leak but don't paste it into commits).

## opencode wiring
- Provider `hf-ornith-397b-fp8` (in `~/.config/opencode/opencode.json`) → baseURL now points at
  the SGLang endpoint (`https://y81h035dzmi2enyy.../v1`). Model id in opencode:
  **`hf-ornith-397b-fp8/deepreinforce-ai/Ornith-1.0-397B-FP8`**.
- Preset: temp 0.6 / top_p 0.95 / top_k 20, ctx 262144 (matches the bf16 eval for fairness).
- Backups: `opencode.json.bak-2026-07-07`, `.bak-2026-07-08`.
- SGLang serves OpenAI-compatible at `/v1`. Verify served id via `/v1/models` once up.

## Gotchas (avoid rediscovering)
- **Fish shell:** the Bash tool runs fish. NO bash `for … do … done` inline (use fish `for … in …; …; end`, or write a `#!/usr/bin/env bash` script and run it). `grep --include=*.go` needs the glob quoted (`--include='*.go'`) or fish tries to expand it.
- **SGLang args ≠ vLLM args:** tool parser is `qwen3_coder` (SGLang) vs `qwen3_xml` (vLLM); SGLang has no `--enable-auto-tool-choice`. Reasoning parser `qwen3` works on both.
- **Don't set `--served-model-name`** — it renames the served id and breaks the opencode model key.
- **Reasoning traces:** opencode captures the model's `<think>` into `part` rows (client-side
  extraction) even though the API's `reasoning_content` is empty and session `tokens_reasoning`=0.
  Transcripts live in `~/.local/share/opencode/opencode.db` (SQLite; tables `session`/`message`/`part`; `part.data` is JSON with `type`∈text/tool/reasoning/patch). Query read-only: `sqlite3 -readonly "file:...opencode.db?mode=ro&immutable=1"`.
- **Monitor scripts** are in the session scratchpad: `monitor_sglang.sh` (running as `btutiaa5c`), plus `poll_*.sh`. A fresh session won't have that scratchpad — just re-poll the endpoint state directly.
- **Cost model:** the real spend is GPU wall-time, not tokens (opencode logs `cost=$0` for custom endpoints; the ~8.6M "input" tokens are mostly re-sent context the HF endpoint doesn't report as cached).

## Next steps (in order)
1. **FP8 is blocked by serving-stack tooling (not the model).** Options (all uncertain, see
   §UPDATE below): (a) a newer SGLang image whose transformers knows `qwen3_5_moe`; (b) vLLM on
   H200 — coin-flip on whether Hopper picks a non-buggy FP8 kernel; (c) custom container with a
   source-built engine; (d) **SHELVE FP8 and let the bf16 28/30 stand.** Recommend confirming with
   the user which to pursue before another paid redeploy.
2. **When an FP8 submission lands** in `ornith-1.0-397B-FP8/` (currently only holds the audit
   rubric): run the **/model-eval** skill →
   `spine eval add-run --eval 2026-07-02-govmomi-vsphere-inventory-cli --name ornith-1.0-397B-FP8 --dir .`
   → Wire → Audit → Score → **Compare vs bf16 28/30** (the whole point: does FP8 preserve honesty/capability?).
   Audit rubric: `govmomi-cli-audit-prompt.md`. Write `ornith-1.0-397B-FP8/REVIEW.md`.
3. **Clean up:** delete the failed `ornith-1-0-397b-fp8-dhv`; delete/pause the SGLang endpoint
   after the FP8 eval to stop the $22/hr meter.
4. bf16 branch stays unpushed until the user says push.

## UPDATE (post-handoff, 2026-07-08 ~21:07Z) — SGLang ALSO failed; supersedes the optimistic next-steps above
SGLang v0.5.8 crashed at config load:
`ValueError: ... model type 'qwen3_5_moe' but Transformers does not recognize this architecture` →
`KeyError: 'qwen3_5_moe'`. SGLang has no native impl for this arch, so it fell back to
`transformers`, and that image's transformers is **too old** to know `qwen3_5_moe`. This is an
**arch-version** failure, **NOT** hardware/FP8 — so an **H200 swap does NOT fix the SGLang path**
(the KeyError happens on any GPU). Endpoint `ornith-1-0-397b-fp8-icb` is now `failed`.

**Corrected state — neither available engine serves the FP8 model:**
- vLLM 0.23.0 (= `latest`): recognizes the arch (`Qwen3_5MoeForConditionalGeneration`) but has the
  CUTLASS FP8 `weight_loader` kernel bug on this W8A8-FP8 checkpoint.
- SGLang v0.5.8: doesn't recognize the arch at all (transformers too old).

**Remaining options for the FP8 data point (all uncertain):**
1. **A newer SGLang image** (if HF offers a tag > v0.5.8 whose transformers knows `qwen3_5_moe`)
   — check HF's SGLang image dropdown / tags. Best bet if one exists.
2. **vLLM on H200 (Hopper)** — GENUINELY UNCERTAIN, not "guaranteed" either way. vLLM *does*
   support the arch; the open question is whether Hopper selects a *different* FP8 linear kernel
   than Blackwell's buggy `CutlassFP8ScaledMMLinearKernel`. Might dodge the bug, might not. One
   deploy tells you. (Was my flip-flop point — treat as a coin-flip worth one try.)
3. **Custom container** with a source-built vLLM/SGLang (newest transformers + FP8 fixes) — most
   control, most work.
4. **Shelve FP8** — it's blocked by serving-stack/arch tooling, not the model. The **bf16 28/30
   stands** as the model's result; note FP8 as "blocked: qwen3_5_moe FP8 unsupported by
   available HF engines as of 2026-07-08."

**Cleanup:** delete BOTH failed FP8 endpoints — `ornith-1-0-397b-fp8-dhv` (vLLM) and
`ornith-1-0-397b-fp8-icb` (SGLang). The bf16 `ornith-1-0-397b-uki` is paused (harmless).
