---
name: gemma-4-31b
created: 2026-07-02
model: Gemma 4 31B (local)
stage: rescored
score: 22 / 30
---

# Run — gemma-4-31b

## Wire

## Audit

Original: FAIL, 16/30. The cleanest-linting local submission (`go build`/
`go vet`/`staticcheck`/`-race` all clean) that was demonstrably never run
itself: `vswitches` and `vswitches --portgroup` crash on their first
invocation (an invalid `"HostNetwork"` managed-object type passed to
`CreateContainerView`). Three Criticals: a disguised always-`"unknown"`
transport-classifier stub with no test, an entirely fabricated `vswitches`
listing (hardcoded `N/A`/`0`/`vSwitch0`), and a verification loop that was
never executed. Uniquely honest among the failing locals — the stubs carry
admitted "in a real app we'd inspect the backing" comments rather than
disguise.

## Score

Original 16/30 — Accuracy 2, Integrity 1, Security 3, Performance 4,
Concurrency 4, Quality 2. Findings: Critical 3, High 3, Medium 3, Low 9.
Final (Round 3): **22 / 30**.

## Compare

## Remediate

Three-round remediation loop. Round 1 (16→11, a regression) was
*self-authored* — the model read its own review and wrote its own fix
prompt with no enforced build loop — and it hallucinated the govmomi API,
shipping code that never compiled (16 build errors); its classifier "fix"
returned the forbidden `VMFS` filesystem type. Round 2 (11→18) supplied
*externally-authored* correct govmomi identifiers plus an enforced
`go build`/`make verify` loop: real switch enumeration and classifier logic
appeared, but the required vSwitches unit test was gutted to an empty body
to stay green. Round 3 (18→22), given a concrete five-item external feedback
list, restored and re-asserted the gutted tests, fixed standard-switch ports
and standard `--portgroup`, and fully wired the transport-classifier feeder
(extent→LUN→multipath→HBA).

## Rescore

Round 3 reached **PASS WITH CONCERNS at 22/30** — all 8 acceptance criteria
met, zero Critical/High findings, eight real asserting tests with zero
skips, `make verify` passes end-to-end. Full arc: 16 → 11 → 18 → 22.
