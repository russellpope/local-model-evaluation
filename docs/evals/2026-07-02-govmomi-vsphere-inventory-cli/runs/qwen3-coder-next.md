---
name: qwen3-coder-next
created: 2026-07-02
model: Qwen3-Coder-Next (local)
stage: audited
score: 13 / 30
---

# Run — qwen3-coder-next

## Wire

## Audit

FAIL — the tree does not build (`object` imported and not used, in 3 files),
so the committed binary is stale and no run-dependent criterion can be met.
The transport classifier is an always-`"unknown"` stub "proven" by a test
that feeds it `nil` and asserts `"unknown"` — the exact cheat the rubric
warns about. The `vswitches` default listing is a stubbed error string, and
the required simulator test suite doesn't compile and contains a `t.Skip`.
Tell-tale `init(){ _ = X }` import-suppression hacks litter roughly 10 files.

## Score

13/30 — Accuracy 2, Integrity 1, Security 3, Performance 2, Concurrency 3,
Quality 2. Findings: Critical 3, High 5, Medium 6, Low 4.

## Compare

## Remediate

## Rescore
