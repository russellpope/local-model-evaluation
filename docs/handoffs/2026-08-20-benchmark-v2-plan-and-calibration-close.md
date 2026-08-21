# Handoff Reference — Benchmark v2 plan + calibration-session close (2026-08-20)

Successor to `2026-08-20-ornith-1.5-bf16-audit-and-calibration-defect.md`. This session (a) finished
the field-wide scoring-calibration quantification, (b) ran transcript forensics on how ornith-1.5
actually failed, (c) audited the vendor card/template for parameter errors (none material), and
(d) ended with the operator pivoting to a **benchmark v2 plan**, captured here. Nothing was
committed and no `score:` value was changed.

## The v2 plan (operator's four items, digested)

**1. Subagent-decomposition variant of the benchmark.** The eval prompt itself instructs the tested
model to spawn subagents per task block; each executes its block and exits. Mirrors how the operator
actually works (`/to-spec`, `/to-tickets`). Purpose: rules out context depth, attention decay and
compaction as score factors, and is the controlled experiment for this session's central finding
(defect-introduction depth — see below). Consequence to state up front: v2 measures *orchestration
under prescribed decomposition*, v1 measured *context management as capability*. The two ledgers are
deliberately non-comparable.

**2. New repository, outside `local-model-evaluation`, similar structure.** Model matrix: Ornith 1.0,
Ornith 1.5, Qwen3.8, Laguna, maybe one more. Precisions BF16 / Q8 / Q4 where obtainable. Known
wrinkles: Laguna has no local BF16 (118B ≈ 236 GiB, exceeds the machine) — its two builds are
Q4_K_M and the mislabelled `hf-Q4_K_M` that is actually MOSTLY_Q8_0 (~"q7.5"); Ornith 1.0 exists as
fp16 35B; 1.5 has all four GGUFs byte-verified on disk. ~350 GiB free at close.

**3. Granular anchored rubric, fixed BEFORE any scoring.** The v1 defect (no anchors → auditors
calibrate from scratch) must not recur. Seed material exists: §4 of
`docs/evals/2026-07-02-govmomi-vsphere-inventory-cli/artifacts/2026-08-20-score-calibration-quantification.md`
(severity-consistent per-dimension anchors, unverifiable ≠ clean, hygiene caps at 4). Calibration
plan: run frontier models — operator named **Fable, Opus, Sonnet, Sol, Terra** — on the v2 bench
first to establish "what good looks like" empirically, then pin anchors to that corpus. Procedural
rules to bake in from this session's evidence: blind pass + synthesis-against-precedent is
**mandatory** (the ornith-1.5 flat-2 anomaly came from four independent scorers with no synthesis);
mutation battery is a reporting instrument, never a survivor-charge (the 30/30 reference kills only
2/8); requirements-attack before judging; the v1 rubric's known contradictions (RA-A…RA-G in the
prior handoff) get *fixed* in the v2 instrument — v1 stays frozen for comparability.

**4. The bench doubles as the measuring stick for the harness-enhancement project** (maipipe,
maikanban, spine gate packs): same model ± harness tooling, so v2 must be re-runnable and stable —
pinned govmomi version, pinned simulator, versioned rubric.

**Anti-gaming rationale.** Operator suspects the vendor's card benchmarks (SWE-bench 79,
Terminal-Bench 68.5, DeepSWE 22 — all measured at temp 1.0) are trained-against. A private/fresh
task surface is the defense, even if this repo is an unlikely target.

## What this session established (carry, don't re-derive)

**Calibration quantification — complete, decision pending.** Artifact:
`artifacts/2026-08-20-score-calibration-quantification.md` (untracked; commit it). Headlines: within
the August cohort Criticals predict totals monotonically for every run except ornith-1.5 (1C/7H → 12
where the cohort line predicts ~16–18); twelve dimension scores ≥4 across the field rest on
unverified absence (Concurrency 4 for binaries that cannot run, etc.); the 2026-08-06 retro battery
(spine repo, `docs/research/2026-08-06-mutation-battery-repro/`) shows opus 30/30 kills 2/8,
gpt-5.5 2/8, 397B 2/8, qwen-3.6-27b 0/8, laguna 5/8 — mutation evidence was never priced into any
score except ornith-1.5's. Remap under proposed anchors: ornith-1.5 12→~16, qwen3.8-bf16 23→~20–21,
veneer tier drops (qwen-3.6-27b →~10). **Four resolution options in the prior handoff; recommendation
was option 1; the v2 pivot may argue option 3-lite instead (flag v1 non-comparable, freeze it, put
the energy into v2's anchored rubric). Operator has not chosen.**

**Ornith-1.5 failure mechanism — settled by transcript forensics** (session
`ses_fdeec5cf6ffeIrQsOkdOC96bUK`). 15:29 plan "classifyTransport is used in datastores.go" stated
at ~164k depth; 15:46 first write of `inventory.go` (~188k, 72% of window) already contained a
*second* transport implementation and never referenced the tested one; compactions (16:22, 18:20)
exonerated for introduction but compaction-1's summary carried "Transport: pure classifyTransport →
FC/iSCSI/NVMe/NFS/unknown" as a **verified API fact**, erasing the recovery path; at 17:22 the model
ran a used-only-by-tests check on `formatBytes` from the same grep output showing `transport.go` and
never asked it of the classifier; `--timeout` was never exercised once (0 bash calls). The cut
itself (classifier never fed real HBA data) is a lineage constant — 1.0 stubbed the feeder
`return nil, nil` at 87k (33%) with a false API comment; 397B shipped it unpopulated and fixed it in
one remediation round.

**Five-run defect-depth comparison.** Code-first models (agentworld first code at 5%, laguna 7%,
ornith-1.0 12% of window) introduced every Critical shallow, zero compactions — pure capability
failures. Ground-truth-first models (qwen3.8 at 40%, ornith-1.5 at 58%) implement deep, and each
shows exactly one depth/compaction-mediated integrity defect: qwen3.8's fabricated README sample
written **post-compaction** at 38k rebuilt context; ornith-1.5's unwired classifier. The exploration
strategy carries a depth tax; v2's subagent design removes it by construction.

**Card/parameter audit — nothing material was run wrong.** Sampler matches the card's general
preset; YaRN correctly avoided; tool round-trip verified. Two findings: (a) all card benchmark
numbers are temp 1.0 ("benchmark reproduction") — an untested arm here, not an error; (b) the
first-party GGUF bakes an **Unsloth-patched** template, not the vendor's benchmark
`chat_template.jinja` — diffed: cosmetic deltas only (system-merge, exception strictness, booleans
`True` vs `true`). The template preserves **all** prior reasoning unconditionally (`last_query_index`
computed but never applied) — verified live via `/apply-template` probe on port 1234 (both markers
retained, zero empty think blocks). So the 15:29 plan *was in context* at the fork, and marathon
tasks always run deep on this model by design.

**Adoption verdict (operator's).** 1.5-35B is a turd for the intended use: calibrated ~16 is heavily
qualified, below the pre-registered >18 slot bar, and below its own predecessor's era-relative
position. Remediation declined in principle — record it agents-a1-style ("deliberate decision, not
an omission") in the run record's Remediate section when the ledger settles. Rungs 2–4 go/no-go is
now folded into the v2 planning rather than a standalone decision. Standing caveat: every cull rests
on n=1.

## Open questions for the next session

1. v1 ledger: option 1 (anchor + remap all 18) vs option 3-lite (flag & freeze, energy to v2).
2. v2 task content: reuse the govmomi task (comparability, sunk ground truth) vs a fresh same-shape
   task (anti-contamination — the stronger fit for the gaming suspicion). If fresh: what domain?
3. Ornith-1.5 rungs 2–4: fold into v2 matrix, run under v1 for n=2, or shelve.
4. v2 repo mechanics: name, public vs private (private fits anti-gaming), spine eval scaffold +
   `workflow-init`.
5. Sampler policy for v2: vendor-per-model (v1 rule) vs pinned-across-field; and whether to buy a
   variance floor (n≥2 at one precision) before interpreting any quant delta — the manifest's open
   decision, still open.
6. Subagent mechanics: verify opencode subagent support under `llamacpp-local` and that weak local
   models can drive it; subagent sessions carry `parent_id` in the db (forensics-ready); the 32k
   `max_tokens` harness constant applies to every request.
7. Fifth model slot: kat / muse / gemma / something new.
8. Micro-tasks pending: commit the quantification artifact; §6 of
   `cross-run-reasoning-failure-modes.md` (fork timeline + five-run depth table) drafted in
   conversation but not written; §5's ornith-1.5 row is a mid-run snapshot needing regeneration
   (final: 442 tool calls — 331 bash / 56 edit / 33 read / 22 write).

## Gotchas & hard-won lessons (new this session)

- opencode db (`~/.local/share/opencode/opencode.db`, open `file:...?mode=ro`): `session` has typed
  columns (`directory`, `title`, `model`, token counts) — not JSON; `part`/`message` keep JSON in
  `data`. Local→epoch queries need `+25200000` ms (UTC-7). Compaction summaries are large assistant
  `text` parts (>5k chars), not part of the `compaction` part itself.
- `/apply-template` on llama-server is a free, read-only way to test template behavior against the
  live model — used it to prove reasoning retention; reuse for v2 wire gates.
- llama-server on port 1234 currently serves `ornith-1.5-35b-a3b-bf16` (BF16, experiments); it
  answers regardless of the request's `model` field.
- The retro mutation-battery logs live in the **spine** repo
  (`spine/docs/research/2026-08-06-mutation-battery-repro/`), not here; specs live here under
  `mutation-specs/`.
- WebFetch of the HF card works; the GGUF repo card's "Qwen chat template needs to be modified" is
  about the vendor's *benchmark* setup (Harbor/vLLM reasoning_content alignment), not a llama.cpp
  action item.
