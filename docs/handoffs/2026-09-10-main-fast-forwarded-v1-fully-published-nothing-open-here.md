# Handoff Reference — main fast-forwarded; v1 fully published; nothing open here (2026-09-10)

This closes the two publication questions left open by
`2026-09-09-v1-published-to-origin-active-work-is-ladderbench.md`. No eval work happened
this session. The repo is frozen and published. Active work is ladderbench.

## Why (key decisions + rationale)

**`origin/main` was fast-forwarded to `ornith-1.5`.** The operator chose this on 2026-09-10
after the merge-base check confirmed a clean fast-forward. Command run:

```
git push origin ornith-1.5:main
   07ed7d2..aa6e231  ornith-1.5 -> main
```

Verified after the push: `origin/main` is at `aa6e231` and
`git ls-tree -r --name-only origin/main -- docs/evals/2026-07-02-govmomi-vsphere-inventory-cli/runs/`
lists 31 files, up from 11. A plain `git clone` now reproduces all 31 runs.

**ladderbench's unpushed commits were already pushed** before this session started.
`git rev-list --left-right --count origin/main...main` there prints `0 0`, tip `b5a863e`
(handoff 23). The 09-09 PICKUP block in this repo was written before that push and was stale
on this point.

**The 09-09 PICKUP's step 3 ("run the I017 thinking study") is also stale.** ladderbench
handoffs 21 through 23 record that the study ran, its discussion was held, and it closed with
ADR 0006. The next step there is grilling ladder 2 (ticket I054).

## Alternatives considered / rejected

- Leave `main` at `07ed7d2`. Rejected by the operator; the default clone should carry the
  full corpus.
- Merge commit instead of fast-forward. Unnecessary; the ancestor check passed.

## Open questions & risks

None new in this repo. Carried over unchanged from 09-03 and 09-09:

1. Two overrulable Critical-accuracy calls (`deepseek-v4-flash-0731`,
   `omp-qwen-3.8-flash-run2`).
2. Auditor drift unquantified across the nine hosted audits; mutation battery run on only one.
3. Script portability: hardcoded paths in `artifacts/tooling/fetch_ornith15.sh`,
   `gate3_tools.py`, `template_probes.py`, `qwen38-sampler.py`, `run_watch.py`.

## Gotchas & hard-won lessons

- `ornith-1.5` and `main` now point at the same commit. Future commits here should land on
  `main`; `ornith-1.5` is a historical branch name, not the working branch.
- Everything in the 09-09 handoff's gotchas still applies: anchored gitignore rules, explicit
  staging, ledger over README, two commits per run, `PICKUP.md` never staged.
- `.foo.swp` at the repo root is still untracked and still left alone.
