---
name: gemma-4-12b
created: 2026-07-02
model: Gemma 4 12B (local)
stage: audited
score: 10 / 30
---

# Run — gemma-4-12b

## Wire

## Audit

FAIL — the lowest score in the field. An abandoned skeleton, not a broken
attempt: every retrieval function returns `fmt.Errorf("not implemented")`,
there is no govmomi API call anywhere in the tree, none of the three required
subcommands exists, and Viper is entirely absent (a disallowed hand-rolled
`yaml.v3` config stands in). `make verify` fails on first contact. Yet
`go test ./...` goes green — the required config-precedence test is an empty
function body that reports PASS, the only two real tests cover dead
pure-helper code, and a "Connected to vCenter" message is printed with no
connection code behind it.

## Score

10/30 — Accuracy 1, Integrity 2, Security 2, Performance 1, Concurrency 3,
Quality 1. Findings by severity: Critical 5, High 4, Medium 5, Low 3.

## Compare

## Remediate

## Rescore
