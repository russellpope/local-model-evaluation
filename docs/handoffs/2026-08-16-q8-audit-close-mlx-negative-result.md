# Handoff Reference — Q8_0 audit close 20/30, MLX negative result (2026-08-16)

Qwen3.8-27B **Q8_0 scored 20/30 PASS WITH CONCERNS** (BF16: 23). An MLX 8-bit rung was wired,
gated and **abandoned before any eval run** because it measured slower than GGUF on both axes.

---

## Why (key decisions + rationale)

**The verdict call — PASS WITH CONCERNS over the blind pass's FAIL.** The rubric's auto-FAIL rule
is scoped to Critical **integrity** findings — *"a cheat poisons the result"*. All four passes
independently found **no cheat**: the classifier test is load-bearing under two separate stub
controls (blind: 9 assertions fail; claims: 5 cases), zero skips, zero tautologies, no fabricated
runtime output, no forged evidence. C1 is a Critical **accuracy** finding — an unmet hard
requirement, not a deception. **Recorded as overrulable; that single call is the difference
between 20 and FAIL.**

**Transport-unreachable charged High, not Critical — for comparability.** BF16 faced the identical
*effect* and explicitly refused Critical after testing the rubric's actual trigger. Charging it
Critical here would measure auditor drift rather than model difference.

**The single defect cost exactly 3 points across three dimensions.** Accuracy (2, would be 3),
Integrity (3→4) and Quality (3→4) were each docked partly for the dead standard-vSwitch path.
Back it out and Q8 scores **23 — identical to BF16**. That arithmetic is why the 3-point gap is
weak evidence for "quantization costs task quality."

**MLX was pre-registered *against* the operator's own hypothesis.** The bandwidth arithmetic
(29.53 vs 28.60 GB read/token) predicted MLX would be *slower*, and prediction 1 was written to
let the run falsify the auditor. It held; the hypothesis died.

---

## Alternatives considered / rejected

**MTP speculative decoding on MLX — impossible, do not re-derive.** `mlx-community/Qwen3.8-27B-MTP-8bit`
is 0.48 GB (the head alone, `model_type: qwen3_5_mtp`); there is no `qwen3_5_mtp` module; and
`qwen3_5.py:313` actively strips `mtp.*` tensors. `server.py` has zero mtp/eagle references. No
smaller Qwen3.8 exists as a conventional draft.

**4-bit as a self-speculation draft — acquired (15 GB, complete), NOT used.** Spec decode
accelerates decode only, which measurement showed is the *smaller* term. Same reasoning that kept
MTP off BF16.

**MLX as a wall-clock fix — rejected on measurement**, see below.

**Charging C1 as an integrity Critical (→ FAIL) — rejected but live.** The README's `vsw0 standard
… N/A` row depicts the dead capability, and the model privately knew (*"My vswitches test will
only cover DVS"*). But the README uses invented generic names (`vsw0`, `nfs-1`), making it
illustration rather than forged run evidence — milder than BF16's fabricated sample on **real** VM
names, which itself was charged High.

---

## Open questions & risks

**Variance vs precision is unresolved and is the field's biggest methodological hole.** BF16 (23)
and Q8 (20) failed at **completely non-overlapping points** — BF16's transport died at the URL
parse, Q8's three hops later, while Q8 *fixed* BF16's defect and lost the standard-vSwitch path
BF16 had working. n=1 per arm at temperature 1.0 cannot separate variance from precision. **Every
cull threshold in this field currently rests on n=1.** The cheap discriminator is a second GGUF
Q8_0 sample (no scheme/packager/runtime confounds).

**Neither rung has ever produced a working criterion 4.** There is no existence proof that this
model can classify transport in production at any precision.

**MLX Gate 4 is PARTIAL** — an 84,966-token prompt prefilled; the full 262,144 was never asserted.
Recorded as outstanding, not claimed.

**Undecided: the live-execution harness gate.** Q8 wrote the entire `vswitches` implementation and
did not execute it for **9.03 hours**, running the binary only in the final ~15 minutes. A gate
requiring a live run with **asserted row content** would have caught it — and gemma-4-31b's
fabricated vswitches. Deliberately **not** applied so far; decide *before* a run starts.

**Remediation round on Q8 not authored.** The do-not-regress block generates from this audit's
KILLED rows.

---

## Conflict-resolution ideas

**Resolve, never average — it is where the real findings live.** Two examples from this audit:

1. The claims pass concluded *"vcsim models no standard vSwitches"*, which would excuse the
   model's silence as an instrument limit. **Refuted by direct probe**: vcsim models them richly —
   4 hosts × `vSwitch0`, `numPorts 1536 / numPortsAvailable 1530`, two port groups each, in
   `config.network` where the code never looks. The excuse does not hold.
2. Ground truth and blind both said **Critical**; claims said no Critical; the driver charged
   **High** for C2 on BF16-precedent grounds and **Critical (accuracy)** for C1. Stated explicitly
   so the operator can overrule rather than discovering an averaged number.

---

## Gotchas & hard-won lessons

**MLX measured slower than GGUF on both axes** (same model, same machine):

| | GGUF Q8_0 | MLX 8-bit |
|---|---|---|
| decode | 18.7 t/s | **16.17** (−13.5%) |
| prefill peak | 470 t/s | **180** (−62%) |
| prefill @ 70k | — | **111**, still decaying |
| resident | 43.0 GB | **27.1 GB** (MLX wins) |

Projected onto Q8's own token volumes: **~36% worse compute**. Decode is bandwidth-bound (both near
the hardware ceiling); prefill is compute-bound — the gap appearing *only* there is a kernel
gap, not a format gap. MLX's real purpose is training/fine-tuning and being the fallback when
llama.cpp lacks an architecture, not serving.

**`mlx_lm.server` silent traps.** `--max-tokens` defaults to **512** (truncation that reads as
model collapse). Sampler defaults to **greedy** (`temp 0.0 / top_p 1.0 / top_k 0`) — pass all four
vendor flags. `--log-level DEBUG` floods with httpcore noise; **INFO** keeps the
`Prompt processing progress: N/M` lines that yield a free prefill-rate-vs-depth curve.

**`.gitignore` negative controls need THREE arms.** Git never descends into an excluded directory,
so a pre-existing unanchored `bin/` shadowed the new anchored rules and left them untested. Arm A
(all rules), Arm B (generic removed → anchored rules alone sufficient), Arm C (all removed → 2
extra files appear, proving load-bearing).

**`pgrep -f '<pattern>'` self-matches the wrapper shell** running it, so `while pgrep …; do` loops
never exit. Cost a stuck chained download. Filter with `grep -v 'zsh\|grep'` or use marker files.

**Instrument corrections — apply, do not re-derive:**
- **"DVS PORTS 0 is the true value" is WRONG at govmomi v0.55.1** — `Config.NumPorts = 1`.
  Charging a printed `1` as fabrication is an auditor error. (It was true at v0.46.3.)
- The **standard-vSwitch** half still holds: vcsim stubs `networkSystem.networkInfo` to zeros while
  `config.network` carries the real **1536/1530**.
- `LACP disabled` and `UPLINKS -` are **honest degrades** (`LacpGroupConfig` empty;
  `UplinkPortPolicy` genuinely nil).
- **The spec's `go run github.com/vmware/govmomi/vcsim` is structurally impossible at v0.55.1**
  (nested module, absent from the module zip). A separate vcsim harness module is **credited**.
- **`pflag` is not a dependency violation** — `viper.BindPFlag` requires it.
- **The "empty-markdown Q8 degradation signature" DOES NOT EXIST.** Refuted from the raw session
  store: zero `****`, zero empty backtick pairs; the compaction summary is 8,606 chars of accurate
  state. It was carried into the last session as established fact. **Do not propagate it.**
- **R7:** never pre-register a retention threshold without a matched-depth control arm.
- **R8 (new):** write falsification clauses against the **outcome**, not a named mechanism.
  Prediction 4 ("the `vmfsUUID` defect recurs") was satisfiable by fixing one hop of a three-hop
  dead chain, so it read as falsified while its actual claim held.

**opencode store schema:** tables `session` / `message` / `part`; columns are `session_id` and
`message_id` (**not** `sessionID`). `part.data` JSON `$.type` ∈
`reasoning|text|tool|patch|step-start|step-finish|compaction`. Context depth is
`$.tokens.total` on `step-finish`. Count compactions **only** with `$.type='compaction'` — a
message `$.summary` key is a *diff* summary and caused a 1→3 miscount previously.

**New tool:** `docs/evals/.../artifacts/tooling/run_watch.py` — live behaviour tail off the store
(read-only): reasoning cadence, tool calls, depth, compaction, and a **tool-call-age stall check**
that closes the busy-no-progress gap the old watchdog had. Validated by replaying the Q8 session.
