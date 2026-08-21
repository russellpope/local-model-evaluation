# Score-calibration defect — field-wide quantification (2026-08-20)

Companion to `docs/handoffs/2026-08-20-ornith-1.5-bf16-audit-and-calibration-defect.md`.
Method: all 18 scored run records read in full this session, plus the four-pass artifact sets
(qwen3.8 ×2, kat, muse, laguna-hf), the four-auditor `ornith-1.5-35b-a3b-bf16/REVIEW.md` distillate,
and the 2026-08-06 retro mutation-battery logs
(`spine/docs/research/2026-08-06-mutation-battery-repro/*.log`). **No `score:` value is changed by
this document.** Section 6's remapped totals are a proposal awaiting the operator's option pick.

## 1. Per-run table — scores, findings, audit depth

Dims are Accuracy/Integrity/Security/Performance/Concurrency/Quality at the run's **latest** scored
round; "base" rows show first-audit dims where an arc exists. Depth = scoring passes + execution
probes + mutation testing *in that audit*. Retro-battery kill rates (2026-08-06, spine 8-class,
never folded into any score) in the last column.

| Run | Audited | A I S P C Q | Total | C/H | Scoring procedure | Mutations in-audit | Retro 8-class |
|---|---|---|--:|---|---|---|---|
| claude-code-opus-4.7 | Jul 2 | 5 5 5 5 5 5 | 30 | 0/0 | 1 subagent + Opus 4.8 re-audit | none | **2/8 = 25%** |
| gpt-5.5 (base 26) | Jul 8→9 | 5 5 5 4 5 5 | 29 | 0/0 | 1 subagent + orch repro | none | **2/8 = 25%** |
| ornith-1.0-397B (base 22) | Jul 7→? | 4 5 5 5 5 4 | 28 | 0/0 | 2 subagents + orch repro | none | **2/8 = 25%** |
| ornith-1.0-35b (base 16) | Jul 2→ | (r3 dims n/r) | 25 | 0/0 | single-pass era; rounds self-prompted | none | no spec |
| qwen-agentworld (base 16) | Jul 2→ | (p2 dims n/r) | 23 | 0/0 | single-pass era | none | no spec |
| **qwen3.8-27b-bf16** | Aug 14 | 3 3 **5** 3 **5** **4** | 23 | 0/3 | blind scorer (25) + driver synthesis (23) | 55 probes, **33 survived (60%)** | — |
| gemma-4-31b (base 16) | Jul 2→ | (r3 dims n/r) | 22 | 0/0 | single-pass era | none | no spec |
| laguna-s-2.1 (base 18) | Aug 1→6 | 5 2 4 3 5 3 | 22 | ?/? | 2 subagents + orch; blind dissents recorded | 26–87/round | 5/8 = 62% |
| qwen3.6-mlx (base 15) | Jul 2→ | (p3 dims n/r) | 21 | ?/? | single-pass era | none | no spec |
| laguna-hf (base 16) | Aug 4→5 | 3 3 3 3 **5** 3 | 20 | 0/7 | blind (18) + claims (20) + orch; **C5 "on cohort precedent"** | 26 | — |
| **qwen3.8-27b-q8_0** | Aug 16 | 2 3 4 3 5 3 | 20 | 1/2 | blind (FAIL) + driver synthesis (20 PWC) | 22 probes, 28% kill | — |
| qwen-3.6-27b | Jul 2 | 2 1 3 3 **4** 3 | 16 | 5/6 | single pass | none | **0/8 = 0%** |
| muse-glimmer-30b | Aug 12 | 2 1 3 3 3 2 | 14 | 3/– | blind + claims + battery + ground truth; blind scored 14 independently | 14 | — |
| kat-coder-v2.5 | Aug 14 | 1 1 3 3 4 2 | 14 | 7/4 | 4 passes | 38 probes, 26% scorable | — |
| qwen3-coder-next | Jul 2 | 2 1 3 2 3 2 | 13 | 3/5 | single pass | none | — |
| **ornith-1.5-35b-bf16** | Aug 20 | **2 2 2 2 2 2** | **12** | 1/7 | **4 independent fresh-context scorers, no synthesis adjustment** | 28 hand-authored, 13 killed (46%) | — |
| gemma-4-12b | Jul 2 | 1 2 2 1 3 1 | 10 | 5/4 | single pass | none | — |
| agents-a1-f16 | Jul 15 | 1 1 2 1 3 1 | 9 | 6/7 | 1 subagent + orch + ground truth | none | — |

## 2. Hypothesis test: does score track audit depth or defect count?

**On totals, depth does NOT depress scores across eras** — the naive story fails. July-audited
local baselines average 14.6 (9–16); August-audited local baselines average 16.7 (12–23). Better
models arrived later; era and model tier are confounded.

**Within the August cohort the test is clean, and it isolates the defect.** All seven August
audits used comparable evidence depth (multi-pass + battery + ground truth). Criticals predict
totals monotonically for every run except one:

| Aug run | Criticals | Highs | Total |
|---|--:|--:|--:|
| qwen3.8-bf16 | 0 | 3 | 23 |
| qwen3.8-q8_0 | 1 | 2 | 20 |
| laguna (base) | 4 | 6 | 18 |
| muse | 3 | – | 14 |
| kat | 7 | 4 | 14 |
| **ornith-1.5** | **1** | **7** | **12** |

One Critical + 7 High lands **below** 7-Critical kat and 3-Critical muse, and 8 below its
severity-profile neighbour q8 (1C/2H = 20). Interpolating the cohort's own line, ornith-1.5's
defect profile predicts **~16–18**. The residual is not evidence depth — it is the **scoring
procedure**: ornith-1.5 is the only run scored by four independent fresh-context auditors whose
numbers were kept without a synthesis-against-precedent step. Every prior multi-pass run had a
driver synthesis that moved the blind number *toward* field precedent (qwen3.8-bf16 25→23,
q8 FAIL→20 PWC, laguna-hf 18/20→20). Four auditors, six dimensions, **24 cells, all 2** — a shared
default, not 24 measurements.

**Where depth does correlate — negatively, per-dimension.** Concurrency: the spec requires no
concurrency (rubric defect RA-F), so the dimension is priced almost entirely on unverified
absence. Field scores: 3 (×4, incl. agents-a1 whose `-race` **could not run**), 4 (×4, incl.
qwen-3.6-27b whose binary cannot connect and qwen3.6-mlx which panics on every subcommand),
5 (×6). The **only** run where the dimension's one substantive requirement (an honoured timeout)
was actually probed — ornith-1.5, via delaying proxy — is the only run scoring 2. Probing the
dimension and scoring below the never-probed field is a negative depth→score correlation.

**Mutation evidence was never priced — for anyone except ornith-1.5.** The 2026-08-06 retro
battery: the 30/30 reference kills 2/8 (surviving: `--portgroup` ignored, columns swapped, LACP
column deleted, RAM mislabelled, TLS-off, logout-deleted); gpt-5.5 (29) 2/8; ornith-397B (28)
2/8; qwen-3.6-27b (16, Quality 3) **0/8**; laguna (22, Quality 3) 5/8 — the field's best kill
rate belongs to a 22. qwen3.8-bf16's Quality 4 explicitly *declined* to charge its 33/55
survivors ("not double-penalised"). Ornith-1.5's Quality 2 charged its 15/28. Kill rate and
Quality score are uncorrelated in this field except in the one run where survivors were charged.

## 3. Hygiene-priced 4s and 5s — the inventory

Dimension scores ≥4 whose recorded justification is hygiene or unverifiable absence, not verified
absence of defects:

| Run | Dim | Score | Recorded justification | What was never probed |
|---|---|--:|---|---|
| qwen3.8-bf16 | Security | 5 | "TLS verify default false, no credential leakage, gosec clean" | ambient env override; injection; leak-form probes |
| qwen3.8-bf16 | Concurrency | 5 | "Race-clean." | timeout plumbing (the ornith-1.5 probe) |
| qwen3.8-q8_0 | Concurrency | 5 | "Zero goroutines, `-race` clean" | (timeout *was* verified — borderline, substantive) |
| laguna-hf r1 | Concurrency | 5 | explicit: "5 rather than 4 **on cohort precedent**" (blind said 4) | — |
| qwen-3.6-27b | Concurrency | 4 | green `-race` suite | binary cannot connect; nothing exercised |
| qwen3.6-mlx | Security | 4 | hygiene | binary panics on every subcommand |
| qwen3.6-mlx | Concurrency | 4 | hygiene | same |
| gemma-4-31b | Performance | 4 | code-shape | submission demonstrably never run |
| gemma-4-31b | Concurrency | 4 | green `-race` suite | vswitches crashes on first invocation |
| qwen-agentworld | Security | 4 / Conc 4 | hygiene | baseline; no probes |
| ornith-1.0-35b | Security | 4 | hygiene | baseline; no probes |
| gpt-5.5 | Concurrency | 5 | "no goroutines, `-race` clean" | borderline: timeout verified under Security |

Contrast rows: opus C5/S5 (everything verified live), 397B r1 S5 (both Lows fixed and re-verified),
laguna S4 ("timeout measured, logout ordered" — substantive). The pattern: **hygiene earns 4–5
when nobody probes; probing converts the same hygiene into a 2–3.**

## 4. Proposed anchors — fitted to the evaluated corpus, opus = 30 preserved

Per operator direction, anchors are oriented on what has actually been evaluated, with the
claude-code-opus-4.7 audit as the definition of 5. Cross-dimension severity rule: within a
dimension, 5 = zero C/H/M in-dimension **and** the dimension's substantive requirement positively
verified; 4 = Medium-only; 3 = one High; 2 = multiple Highs or one High defeating the dimension's
core requirement; 1 = in-dimension Critical / dimension not exercisable. Two standing rules the
inventory above forces: **unverifiable ≠ clean** (a dimension that could not be exercised caps at
3 and is flagged, never 4–5), and **hygiene alone caps at 4** (5 requires the substantive
requirement verified in-run).

- **Accuracy** — 5: 8/8 ACs met, verified live. 4: 6–7 met, rest honest-partial. 3: ~5 met incl.
  ≥1 hard AC (3/4/5/6). 2: ≤4 or a hard AC unmet with the data present. 1: binary doesn't
  run / most unmet.
- **Integrity** — 5: no cheat, forensics clean, disclosures honest. 4: no cheat; membership-test /
  few withheld limitations. 3: silence about known gaps or invented illustrative docs; no runtime
  fabrication. 2: rubric-§B cheat signature (dead-code classifier behind a green suite, tautology)
  without forged evidence. 1: forged evidence / fabricated runtime output / never-compiled suite.
- **Security** — 5: hygiene + security behaviors probed clean (leak forms, env, TLS
  enforcement). 4: hygiene verified, ≤1 Medium or corners unprobed. 3: one High (ambient-env TLS
  override; documented-form credential leak). 2: leak/defeat in the default path. 1: systematic.
- **Performance** — 5: batched ContainerView/PropertyCollector, minimal props. 4: batched with an
  over-fetch. 3: mixed flat/N+1. 2: N+1 throughout. 1: N+1 plus broken retrieval. (RA-C stands:
  the spec never requires this; grading it is field precedent, kept deliberately.)
- **Concurrency** — redefined openly (RA-F makes the literal dimension unmeasurable): session/
  cancellation lifecycle. 5: `-race` clean + logout all paths + views destroyed + **timeout
  verified honoured**. 4: race + lifecycle verified, timeout unprobed. 3: race clean only,
  lifecycle unexamined. 2: proven lifecycle/timeout defect. 1: `-race` never ran.
- **Quality** — mutation results stay a **reporting instrument** (per skill convention), priced
  only via "does the suite prove the ACs the audit credited," identically for every run — the
  retro battery proves the reference tree itself kills 2/8, so survivors-as-charges would unmake
  the 30 anchor. 5: tooling clean, suite proves its ACs, deliverables complete. 4: tooling clean,
  real suite, survivors confined to instrument-prescribed assertion classes. 3: real suite with
  structural holes (`cmd/` untested) or missing deliverables. 2: green suite hollow on
  AC-load-bearing paths. 1: suite doesn't compile / gutted bodies.

## 5. Remapped totals under the proposed anchors (provisional, from recorded findings only)

Flags: ⚑ = a cell rests on an unprobed dimension scored from the record (no new probes run).

| Run | Old | Remap | Δ | Movers |
|---|--:|--:|--:|---|
| claude-code-opus-4.7 | 30 | 30 | 0 | anchor definition |
| gpt-5.5 | 29 | 29 | 0 | already findings-anchored |
| ornith-1.0-397B | 28 | 28 | 0 | already findings-anchored |
| ornith-1.0-35b | 25 | ~24–25 | ≈0 | r3 verified all ACs |
| qwen-agentworld | 23 | ~23 | 0 | p2 live-verified |
| **qwen3.8-27b-bf16** | 23 | **~20–21** ⚑ | −2/−3 | S5→4 (hygiene-only), C5→4 (timeout unprobed), Q4→3 (`cmd/` zero coverage) |
| gemma-4-31b | 22 | ~22 | 0 | r3 verified; **baseline 16→~12** (P4/C4 on a never-run tree) |
| laguna-s-2.1 | 22 | 22 | 0 | base 18 also stable — audit was anchored in practice |
| qwen3.6-mlx | 21 | ~19–21 ⚑ | 0/−2 | p3 fabrication caps Integrity |
| laguna-hf | 20 | 20 | 0 | C5 keeps only if precedent-rule accepted; else 19 |
| **qwen3.8-27b-q8_0** | 20 | ~19–20 | ≈0 | S4→3 possible (reproduced doc-form leak) |
| qwen-3.6-27b | 16 | **~10–11** ⚑ | −5 | A2→1, I1, C4→3⚑, Q3→2 (0/8 retro kill; binary can't connect) |
| muse-glimmer | 14 | ~14–15 | ≈0 | C3→4 possible (timeout verified) |
| kat-coder | 14 | ~14 | 0 | consistent |
| qwen3-coder-next | 13 | **~8** | −5 | doesn't build: A1, C3→1 (race never ran), Q1 |
| **ornith-1.5-35b-bf16** | **12** | **~16** | **+4** | S2→3 (one High over verified hygiene), C2→3 (one High; race+logout verified), Q2→3 (real suite, `cmd/` hole, 2 deliverables absent); A2, I2, P2 stand |
| gemma-4-12b | 10 | ~8 | −2 | I2→1 (empty test bodies reporting PASS), C3→2⚑ |
| agents-a1 | 9 | ~7 | −2 | C3→1 (`-race` could not run) |

Rank-order effects: ornith-1.5 rises from 13th of 15 to the 16-cluster (still FAIL — the Critical
stands, uncontested); the "convincing veneer" tier (qwen-3.6-27b, qwen3-coder-next) drops below
the honest-skeleton tier, which is the ordering the audits' own prose already asserts; the top
five are untouched. qwen3.8-bf16 remains the best local baseline (~20–21 vs ornith-1.5 ~16) — the
anchors do not flip the rivalry, they narrow it from 11 points to ~4–5.

## 6. Resolution options (operator's call)

1. **Anchor the rubric; remap all 18 from existing findings** (Section 5 is that remap, ready for
   review). Most correct; cost is mostly already sunk in this document. Caveats: ⚑-cells are
   scored from the record without new probes; anchoring the rubric is an instrument change that
   must be dated in the rubric file and noted in every remapped record (kat-R2 precedent:
   mid-field instrument edits break comparability — here that break is the *purpose*).
2. **Re-score ornith-1.5 only, calibrated to qwen3.8's mapping.** Fast; the handoff already
   records the trap — lifting Security/Concurrency to qwen's hygiene mapping erases the two real
   bugs deeper probing found. §2 shows the inconsistency is field-wide (gemma-4-31b P4, qwen-3.6-27b
   C4), so this fixes one row of an internally inconsistent ledger.
3. **Flag 12/30 non-comparable; re-audit qwen3.8-bf16 at ornith depth.** Most empirical; but §3
   shows the hygiene premium extends across at least seven runs — one re-audit re-anchors one
   rivalry, not the field, and costs a full four-auditor pass.
4. **Split ledger (legacy scale + deep-audit scale).** Permanently forks the instrument; every
   future rung pays the dual-bookkeeping cost; nothing is ever reconciled.

**Recommendation: Option 1.** The quantification shows the defect is ledger-wide, the remap is
already drafted from evidence the operator can check row by row, and it unblocks rungs 2–4 on a
single scale (all four Ornith 1.5 rungs then score against the same anchors). Optional add-on, not
required: a targeted probe pass (hours, not a re-audit) on the two decision-relevant ⚑ cells —
qwen3.8-bf16's timeout plumbing and ambient-env override — to convert its 4s from "unprobed" to
measured either way.
