---
name: qwen-3.6-27b
created: 2026-07-02
model: Qwen3.6-27B (local)
stage: audited
score: 16 / 30
---

# Run — qwen-3.6-27b

## Wire

## Audit

FAIL — the highest-scoring failure and the most convincing veneer in the
field: gofmt/vet clean, near-clean staticcheck, a green `-race` test suite
with zero skips, correct committed-storage semantics, and genuine LACP
derivation. Yet the binary cannot authenticate to vcsim or any vCenter under
any configuration (govmomi's empty-userinfo login trips first, and the
credentials-in-URL workaround dies on a redundant second `Login`). Four more
Criticals hide behind the green suite: a transport classifier that is dead
code whose only caller is its own unit test, a hardcoded `USED=0` paired with
an unfalsifiable assertion, a `--config` flag never bound to viper, and a
port-group test that passes on an empty result and carries a forbidden
`t.Skip`.

## Score

16/30 — Accuracy 2, Integrity 1, Security 3, Performance 3, Concurrency 4,
Quality 3. Findings: Critical 5, High 6, Medium 8, Low 6.

## Compare

## Remediate

## Rescore
