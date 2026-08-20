---
name: ornith-1.0-35b-fp16
created: 2026-07-02
model: ornith-1.0 35B (local, fp16)
stage: rescored
score: 25 / 30
---

# Run — ornith-1.0-35b-fp16

## Wire

## Audit

Original: FAIL, 16/30. The first local submission to actually run: it builds
(gofmt-dirty), vets clean, passes a zero-skip `-race` suite, and its binary
logs into the simulator and runs all three subcommands end-to-end with
correct `vms`/`datastores` output. Three Criticals: every connection flag is
dead (registered inside `PersistentPreRunE`, after cobra parses argv, so the
tool is configurable only by env var), the `vswitches` `USED`-ports column is
fabricated (always equals total because `UsedPorts` is never populated), and
standard vSwitches silently vanish (wrong managed-object property, faulted
error swallowed by a `continue`). Its transport classifier is fully honest
but starved — the real FC/iSCSI/NVMe logic is wired and reachable, but its
HBA feeder is hardstubbed to `return nil, nil`.

## Score

Original 16/30 — Accuracy 2, Integrity 2, Security 4, Performance 2,
Concurrency 4, Quality 2. Findings: Critical 3, High 3, Medium 4, Low 5.
Final (Round 3): **25 / 30**.

## Compare

## Remediate

Self-prompted remediation loop (model read its own review, authored its own
fix prompt, repeated against each re-score — no external correction).
Round 1 (16→20) fixed C3/H1/H2/H3/M2/M3/L1–L4: flags now *parse* but their
values are silently dropped, DVS `USED` is still fabricated, and one new
latent ordering bug appeared. Round 2 (20→22) fixed all four Round-1
survivors — `--url` now overrides env live, DVS `USED`→`N/A` (honest),
VM↔NIC keyed by `.Self.Value`, HOST column added — but introduced a new
firing `t.Skip` that broke criterion 8. Round 3 (22→25) replaced that
`t.Skip` with a genuine bidirectional exact-set test (zero skips) and fixed
the residual N+1 loops and O(ds×hosts) HBA walk.

## Rescore

Round 3 reached **PASS WITH CONCERNS at 25/30** — all 8 acceptance criteria
met and independently verified, `go test ./...` reproduced with zero
failures and zero skips, and nothing previously fixed regressed. Full arc:
16 → 20 → 22 → 25.
