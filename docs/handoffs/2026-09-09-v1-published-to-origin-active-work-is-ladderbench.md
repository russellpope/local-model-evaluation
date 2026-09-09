# Handoff Reference — v1 published to origin; active work is ladderbench (2026-09-09)

A reproducibility sweep of this repo found the **content** complete and the **publication**
absent. Seven local branches, including everything since 2026-07-15, existed only on the
operator's laptop. All seven are now pushed. `origin/main` is deliberately unchanged.

This doc carries forward `2026-09-03-v1-corpus-committed-readme-current-ladder-unblocked.md`,
whose open question 5 was "Nothing is pushed in either repo. Operator pushes." That is now
resolved for this repo and still open for ladderbench.

## Why (key decisions + rationale)

**The sweep verified reproduction from source, not from memory.** 27 submission roots each
carry a tracked `go.mod`, `go.sum`, and `main.go`, so every scored submission rebuilds. All 53
gitignored entries were listed and inspected: compiled binaries, module caches, OS cruft, and
`PICKUP.md`. No Go source, module file, or document is hidden by any rule — the anchored-path
discipline from the 09-03 handoff held. The README's audit recipe is self-contained: Go plus
`vcsim` through `go run`, with the environment variables spelled out, and `vcsim` comes from
the pinned module rather than a separate install.

**Branches were pushed; `main` was not moved.** `origin/main` last moved 2026-07-15 and
carried 11 of 31 run records. `git merge-base --is-ancestor origin/main HEAD` succeeds, so
fast-forwarding `main` to `ornith-1.5` is clean and available. It was not done: publishing the
default branch decides what a fresh clone gets by default, and that is the operator's call, not
a side effect of a backup request. Until it is done, `git clone` still reproduces 11 of 31 runs;
`git clone --branch ornith-1.5` reproduces all 31.

**Three record/directory mismatches are intentional, not gaps.** `glm-5.2/` and
`ornith-1.0-397B-FP8/` are seeded-only and labeled so in the README layout tree.
`qwen3.8-27b-8bit-mlx` is a pre-registered run record with `stage: wired` and no submission.
Nothing needs fixing.

**The remaining v1 rungs stay not-run.** `ornith-1.5-35b-a3b-q8_0`, `ornith-1.5-9b-bf16`, and
`ornith-1.5-9b-q8_0` are `stage: not-run` by operator decision of 2026-08-20 and fold into
benchmark v2. Do not start them here. v1 is frozen under option 3-lite.

## Alternatives considered / rejected

- **Fast-forward `main`.** Verified possible, deliberately deferred to the operator.
- **Delete the two seeded-only dirs.** Rejected; the README documents them on purpose.
- **Parameterize the hardcoded script paths.** Deferred. They are generation-side helpers and
  do not block the audit path.
- **Remove the stray `.foo.swp` at the repo root.** Left alone; it may belong to a live editor
  session. It is untracked and was not committed.

## Open questions & risks

1. **`origin/main` is still at 2026-07-15.** One fast-forward closes it.
2. **ladderbench has 20 commits unpushed** on `main` and is the only copy of that work. Its own
   handoff 20 (`2026-09-09-i053-...`) is current; the next step there is the **I017 thinking
   study**, pre-registered with cells gated.
3. **Carried over from 09-03, all still open:** two overrulable Critical-accuracy calls
   (`deepseek-v4-flash-0731`, `omp-qwen-3.8-flash-run2`); auditor drift unquantified across the
   nine hosted audits; mutation battery run on only one of nine.
4. **Script portability.** `artifacts/tooling/fetch_ornith15.sh` hardcodes a download root under
   the operator's home; `gate3_tools.py`, `template_probes.py`, and `qwen38-sampler.py` hardcode
   `127.0.0.1:1234`; `run_watch.py` assumes opencode's database path.

## Gotchas & hard-won lessons

- **Anchor every gitignore rule to a full path.** Submission binaries repeatedly share a name
  with a source directory elsewhere. After adding a rule, prove it with `git check-ignore -v`
  and confirm the tree's `main.go` is still visible.
- **`PICKUP.md` is gitignored on purpose.** Never stage it. Each handoff fully replaces it.
- **The ledger is authoritative, not the README.** `spine eval list --dir .` prints the board and
  needs `spine` on PATH at `~/bin/spine`. Run records are plain markdown and read fine without it.
- **Two commits per run:** `chore(<run>): freeze submission as delivered`, then
  `<run>: audited <verdict> <score>`. The freeze commit is the diff base for a remediated tree.
- **Unrelated loose end from this session:** herdr 0.9.0 dropped the click tests for the sidebar
  collapse control that v0.7.4 had. The click itself was verified working against the real binary
  and could not be made to fail. Nothing filed upstream yet.
