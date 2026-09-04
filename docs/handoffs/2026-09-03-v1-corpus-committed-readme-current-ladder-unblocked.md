# Handoff Reference — v1 corpus fully committed, README current, ladder unblocked (2026-09-03)

Nine audited runs that had sat untracked for up to two weeks are committed (19 commits, freeze
+ audit per run, repo convention). The root README and `ox-alpha-free/README.md` no longer
misstate the field. **Nothing in v1 gates starting the ladder on a model**; the gating
decisions are all on the ladderbench side and all operator-owned.

This doc **supersedes** `2026-09-01-six-audits-ledger-wired-omp-pilot.md`, whose omp
provenance table is wrong in two rows (see below). Read that doc for the six-audit findings
and probe gotchas; read this one for current state.

## Why (key decisions + rationale)

**Two commits per run, not one.** `chore(<run>): freeze submission as delivered` then
`<run>: audited <verdict> <score>`. Matches `084b7ae`/`784a346`. The freeze commit is what a
later reader diffs a remediated tree against; folding the audit in would hide whether the
auditor touched the submission. Nine runs → 18 commits plus one `.gitignore` fix. Order is
audit order (ox-alpha-free first, omp run2 last).

**The `.gitignore` fix came first and is anchored to full paths.** `qwen-3.8-max` and
`qwen-3.8-flash` built a 19 MB binary at the submission *root*, next to `main.go`, named
`vsphere-inventory`. The existing `bin/` rule missed it; a bare `vsphere-inventory` pattern
would have hidden every other tree with a source dir of that name. Rule is
`/qwen-3.8-max/vsphere-inventory` and `/qwen-3.8-flash/vsphere-inventory`; negative control:
`git check-ignore -v` hits exactly the two files and `main.go` in both dirs stays untracked-
visible (28 files each).

**The README got a summary table, not a narrative rewrite.** It claimed "twelve runs" against a
31-row ledger and omitted every run since 2026-08-04 (19 rows, not just the nine hosted ones:
also `laguna-s-2.1-hf-Q4_K_M`, kat-coder, muse-glimmer, three `qwen3.8-27b` rungs, four
`ornith-1.5` rungs). Narrating 19 runs would triple the page and duplicate the ledger. The fix
declares the ledger authoritative in the intro, adds one "Later runs" section (table + the four
findings the hosted set established), and completes the layout tree, including KAT-Coder's
misplaced `govmomi-inventory/` at repo root.

**`ox-alpha-free/README.md` now says what the model is.** It never named it. It is GLM-5.3-Flash
(320B/18B, hosted, cloaked as `ox-alpha`, identity confirmed by Z.ai five days after the
audit). "Matches the best local arc" compared a hosted frontier-class product against local
weights; replaced with a hosted-set table and the variance caveat. The "not yet on the board"
line was false and is gone.

**Criterion-4 count in the README was verified against records, not memory.** Of the nine
hosted runs plus the two local `qwen3.8-27b` rungs, the transport classifier is wrong in
production in eight (bf16, q8_0, qwen-flash, glm-5.3, deepseek, both omp runs, the opencode
repeat) and right in three (ox-alpha-free, qwen-max, glm-flash). The first draft said "eight of
ten" with a vague set; corrected before commit.

## Corrections to the 2026-09-01 handoff (carry these, not the original)

| 09-01 claim | actual (per `omp-qwen-3.8-flash.md`, `-run2.md`) |
|---|---|
| omp records no effort; must be captured at dispatch | omp **records** `thinking_level_change` (`thinkingLevel` + `configured`) in the session `.jsonl`. But it is **intent, not wire**: a config defect means the level may never reach the API. Run 2 at recorded `xhigh` moved spend 8% vs run 1 at `null`. **Verify effort at the request, not the config.** |
| tokens are in `~/.omp/logs/*.log` | The logs hold only the title-generator sidecar (1,440 tokens). Agent-turn usage is in the session `.jsonl` `/message/usage`. |
| `agent.db → model_usage` is the provider/model source | Last-used only. Use the session `.jsonl` `model_change` records, run-scoped. |
| — | Changing the model in omp **silently clears** the thinking level; the cleared state (`null`) is not the default (`auto` → `medium`). |

What the 09-01 doc got right and still stands: reasoning-token count is the measured quantity
and the variant label is a request; scope provenance by cwd AND date; a failing probe needs a
differential; vcsim hides criterion 4 entirely; `agent.db` holds `auth_*` tables, query only
`model_usage`/`model_perf` on a copy.

## Is v1 ready for the ladder? Yes; v1 gates nothing.

**v1 side.** Frozen (option 3-lite, 2026-08-20). All 31 ledger rows committed. Every v2 input
this corpus produced is already filed in ladderbench: omp facts (I008), omp session parser
(I010, merged), omp adapter (I011, merged), harness-comparison ADR (I012), plus the design
decisions on effort-as-pin and cell class. The two open v1 questions are calibration of the *v1*
instrument, not prerequisites: a blind re-audit of `ox-alpha-free` for auditor drift, and a
third qwen3.8-flash run for a variance number. The ladder answers variance structurally with
n=3 per cell, so neither blocks it.

**Ladder side (verified in `~/Projects/github.com/ladderbench` at `3d14402`, main clean, not
pushed).** The ladder has already run to completion on a model: pilot
`qwen3.8-flash-opencode-90` reached `COMPLETE` B0–B8 in 3h35m, audited 88/88 under rc5, re-judged
under rc6 (I032 fixed). An omp pilot record `qwen3.8-flash-omp-90` exists in the i011 worktree,
kept-or-discarded pending the operator. The instrument works; what has not happened is a
**corpus** run (status `planned`, attempt numbers < 90). Three tracks, three different gates:

| track | first cell | gate | owner |
|---|---|---|---|
| cloud | `qwen3.8-flash-opencode` / `-omp` (n=3 each, xhigh) | the harness pick itself (ADR 0005); this *is* the first real run and is on the lab's critical path | ready to dispatch; `ALIBABA_TOKEN_PLAN_API_KEY` in `fish -lc` only |
| local (Mac) | `ornith-1.5-9b-q8_0` (n=3, port 4101, weights on disk) | none hard; the 08-24 smoke hit a B3 DONE-marker protocol gap, I019 landed since. Design says the local matrix *moves to the lab* (I013/I015, lab access is operator-owned) | operator: run on the Mac now, or wait for the lab |
| frontier | `opus` (pilot) | I036/I037 go-ahead: hours of Opus subscription; `memory_paths` isolation escalation open | operator decision, explicitly requested in ladderbench handoff 12 |

Recommendation: the cloud harness comparison is the natural first corpus run. It is fully
gated, its inputs (effort verified at the request, tokens from the session store) are exactly
what the v1 omp pilot taught, and it unblocks the lab plan.

## Alternatives considered / rejected

- **One commit per run.** Rejected; loses the freeze/audit boundary.
- **Full README narrative for the 19 later runs.** Rejected; ledger duplication, ~1500-word
  cap, and the v1 page is frozen prose.
- **Editing the 09-01 handoff body to fix the omp rows.** Rejected; a banner pointing here
  keeps the historical record and the correction both readable. (Precedent `f8dc0c1`
  retracted in place, but that was the same session.)
- **Running the blind ox-alpha-free re-audit now.** Not asked, and it is a v1 calibration
  question the ladder does not depend on.

## Open questions & risks

1. **Two Critical-accuracy-not-integrity calls** (`deepseek-v4-flash-0731`, `omp-qwen-3.8-flash-run2`)
   are each the difference between their score and FAIL. Marked overrulable in §11 of each REVIEW.
   Nobody has overruled or confirmed them.
2. **Auditor drift is unquantified** across the nine hosted audits; the probe set grew mid-sequence.
3. **Mutation battery not run** on eight of the nine (only `ox-alpha-free`, 50% kill). Disclosed
   with empty `battery_*` front matter.
4. **`glm-5.2/` and `ornith-1.0-397B-FP8/`** remain seeded-only dirs; README labels them so.
5. **Nothing is pushed** in either repo. Operator pushes.

## Gotchas & hard-won lessons

- **Bash cwd persists across tool calls in this harness.** A `cd docs/…` in one call made the
  next call's relative paths fail and made `git check-ignore` report "not ignored" (exit 1)
  for a file that *was* ignored. Use absolute paths or `cd <repo> &&` per call.
- **Hosted submissions put binaries anywhere.** Root, `bin/`, `build/`, nested module dirs.
  Before a freeze commit run `git status --short --ignored <dir>` *and* list what the ignore
  rules hide, and check for >1 MB untracked files. The `.gitignore` header comments are the
  log of every collision so far.
- **A ledger record's `model:` line is the identity source**, not the run-dir README. Three
  run-dir READMEs (ox-alpha, and the two qwen-3.8) describe the model loosely or not at all.
- **Fish:** `$pipestatus[1]` not `$?`; no `timeout(1)`; use `bash -c` for flag bundles.
- **Never write files via heredoc** (CLAUDE.md); Write/Edit only. The commit script was written
  with Write and executed with `bash <path>`.

## State at handoff

- `ornith-1.5` @ `99c174d` (20 commits ahead of `a6b009b`, not pushed). Working tree clean
  except this doc, the 09-01 handoff (banner added, committed with this), `.foo.swp`, PICKUP.md.
- `spine eval list --dir .` → 31 rows; `spine doctor` D1 errors expected (not spine-scaffolded).
- No `opencode serve` running from this repo (ladderbench handoff 12 lists :4101/:4110 as up;
  `pgrep -fl "opencode serve"` shows none as of this session). No llama-server, no vcsim.
- omp config: `modelRoles.default: alibaba-token-plan/qwen3.8-flash:xhigh`,
  `defaultThinkingLevel: auto`.
