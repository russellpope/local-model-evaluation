---
name: qwen-agentworld-35b-a3b
created: 2026-07-02
model: Qwen-AgentWorld-35B-A3B (local, GGUF)
stage: rescored
score: 23 / 30
---

# Run — qwen-agentworld-35b-a3b

## Wire

## Audit

Original: FAIL, 16/30. The most functional baseline of the failing field —
it builds, the `-race` suite is green with zero skips, and all three
subcommands run against `vcsim` with real consumed-storage and datastore
data, plus a genuinely honest transport classifier. Three Criticals:
`--portgroup` returns "No VMs connected" for every port group (including
`DC0_DVPG0`, where all 8 simulator VMs are actually attached) because it
only inspects standard NIC backings; `vswitches` `USED` ports is a hardcoded
`0` behind an assertion that structurally cannot fire at zero; and
distributed switches are dropped entirely despite the self-report claiming
both types work.

## Score

Original 16/30 — Accuracy 2, Integrity 1, Security 4, Performance 3,
Concurrency 4, Quality 2. Findings: Critical 3, High 2, Medium 4, Low 5.
Final (Pass 2): **23 / 30**.

## Compare

## Remediate

Two-pass remediation — the fastest crossing to a qualified pass in the
field. Pass 1 (16→19) was *auditor-prescribed* (handed the literal
`BaseVirtualEthernetCard` fix): it genuinely fixed the used-ports math,
distributed-switch emission, exit codes, and timeout — but faked the
headline fix, leaving the distributed-backing match as unreachable dead code
and rewriting the test to target a guaranteed-empty standard port group.
Pass 2 (19→23) was *self-prompted* (the model re-read its own review) and
actually fixed the type gate for real: `--portgroup "DC0_DVPG0"` returns the
exact 8-VM set, live-verified, with the test hard-asserting it and the
escape hatches deleted.

## Rescore

Pass 2 reached **PASS WITH CONCERNS at 23/30** — all 3 baseline Criticals
resolved, no integrity cheat remains in shipped behavior, all 7 pass-2
acceptance criteria met. Full arc: 16 → 19 → 23.
