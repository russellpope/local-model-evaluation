---
name: deepseek-v4-flash-0731
created: 2026-09-01
model: deepseek-v4-flash-0731 — served via opencode provider `alibaba-token-plan`, i.e. an Alibaba **hosted third-party endpoint**, not local weights and **not DeepSeek's own API**. Serving precision **undisclosed**. Nominal open-weight counterpart `deepseek-ai/DeepSeek-V4-Flash` is 284B total / 13B active MoE, 1M context, MIT, shipped natively at **FP4+FP8 mixed** ("MoE expert parameters use FP4 precision; most other parameters use FP8"), ~160 GB checkpoint across 46 safetensors shards. **Snapshot identity unverified** — the `-0731` suffix is not established to match the published card, and community repackagings of a "0731" variant state a different total parameter count. Effort `variant = high`, 53,253 reasoning tokens / 51,861 output across 178 messages, 2026-09-01 09:59:43 → 12:25:52.
stage: audited
score: 18 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — deepseek-v4-flash-0731

## Wire

**Audit-only record.** Generation was driven by the operator; nothing was observed in flight.
No pre-registered predictions, no gate table, no throughput measurement.

Submission landed at `deepseek-v4-flash-0731/` (untracked at audit time), module `vint`, 28
files. Build verified at audit: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean on Go
1.27.0 darwin/arm64 against govmomi v0.56.0.

### Provenance — reconstructed from the opencode store

| field | value |
|---|---|
| provider | `alibaba-token-plan` |
| model id | `deepseek-v4-flash-0731` |
| variant (effort) | `high` |
| messages | 178 |
| reasoning tokens | 53,253 |
| output tokens | 51,861 |
| window | 2026-09-01 09:59:43 → 12:25:52 |

**Three provenance hazards, all recorded because none is recoverable from the tree:**

1. **Not local, and not first-party.** A DeepSeek model served by Alibaba. Neither the weights
   nor the serving stack are under operator control.
2. **Precision is documented for the checkpoint but not for this endpoint.** DeepSeek publishes
   FP4+FP8 mixed for the downloadable artifact; nothing states what the hosted route serves.
   Unlike a local run, the precision line here is an inference about a different artifact.
3. **Snapshot identity is unverified.** `-0731` reads as a date stamp. It is not established
   that it corresponds to the published `DeepSeek-V4-Flash` card.

**Effort discrepancy, unresolved.** The store records `variant = high` for all 178 messages
with no mid-run change; operator recollection at the time of wiring was "medium". No `medium`
value appears anywhere in the opencode store — the observed vocabulary is
`(null)` / `high` / `xhigh` / `max`. The store is taken as authoritative here; flagged rather
than silently reconciled.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context. Raw report:
[`deepseek-v4-flash-0731/REVIEW.md`](../../../../deepseek-v4-flash-0731/REVIEW.md).

**PASS WITH CONCERNS — 1 Critical (accuracy), 2 High, 4 Medium, 2 Low.**

No cheat found: no `t.Skip`, no build-tag fencing, no `recover()`, no disabled tests, no
tautologies, no value hardcoded to fake a gate. `make verify` reproduces green end-to-end.
gofmt/vet/staticcheck/govulncheck clean; `-race` clean with zero goroutines created.

### C-1 (Critical, accuracy) — the classifier fabricates specific wrong protocols

`transport.Classify` (`transport.go:31-44`) treats identifier *prefixes* as proof of transport:

```go
case lower("fibre"), lower("fcoe"), strings.HasPrefix(d, "naa."), lower(" fc "):
        return FC
case lower("iscsi"), strings.HasPrefix(d, "iqn."), strings.HasPrefix(d, "eui."):
        return ISCSI
```

Both prefix rules are false. NAA (Network Address Authority) canonical names are
transport-agnostic — FC, iSCSI, SAS and local SCSI disks all use them. EUI-64 names are used
by NVMe namespaces and other SCSI devices. Yet `naa.` is the primary FC signal, and
`transportFromInfo` (`datastores.go:95-107`) feeds it exactly that: `ext.DiskName`, the
extent's canonical name.

**Two mechanisms proven by injection against the production `ListDatastores`:**

| probe | setup | correct | reported |
|---|---|---|---|
| A | software iSCSI HBA (`iscsi_vmk`, `iqn.…` target) owning LUN `naa.60014051…`, VMFS datastore on that extent | `iSCSI` | **`FC`** |
| B | local disk `mpx.vmhba0:C0:T0:L0`, host also has an unrelated FC HBA | `unknown` | **`FC`** |

Probe B exposes a second, independent defect: `transportFromHosts` (`datastores.go:109-127`)
iterates every HBA on every mounting host and returns the first that classifies to anything,
with **no linkage between that adapter and the datastore's LUN**.

*Why this is worse than degrading.* The spec explicitly permits `unknown` and calls that
graceful degrade correct. This manufactures a confident wrong answer instead — an operator
reading `FC` for an iSCSI datastore is misled in a way `unknown` never would.

*Why charged accuracy, not integrity.* The doc comment states the rule openly (`"naa.…" → FC`),
the unit test asserts the same belief, and the README does not overclaim. A wrong premise
consistently held, not a concealed one. Under the precedent from the 2026-08-16 handoff, the
auto-FAIL rule is scoped to Critical **integrity** findings; this is an unmet hard requirement.
**Recorded as overrulable — this single call is the difference between 18 and FAIL.**

### H-1 — two N+1 access patterns

- `datastores.go:116` — `RetrieveOne` per mounting host, called **per datastore**. Cost is
  O(datastores × hosts) sequential round trips.
- `vswitches.go:203` — `dvsName` issues one `RetrieveOne` **per distributed port group**,
  inside the row-building loop, purely to resolve the owning switch name.

Bulk retrieval is otherwise correct, which makes these two sites the outliers. Neither is
load-bearing for correctness; both could be a single batched retrieve.

### H-2 — nothing in the suite can catch C-1

`datastores_test.go:29-32` builds `validTypes` = {FC, iSCSI, NVMe, NFS, unknown} and asserts
membership. Every fabricated value C-1 produces is inside that set. `transport_test.go:11,13`
compounds it by asserting `Classify("naa.6005076802…") == FC` — an expected value
reverse-engineered from a false premise rather than sanity-checked against the spec. The suite
*ratifies* the defect rather than exposing it.

### Other confirmed findings

- **Medium** — UPLINKS column emits raw managed-object keys
  (`key-vim.host.PhysicalNic-vmnic0` instead of `vmnic0`), verified in live output.
- **Medium** — standard port-group path is functional (verified live) but **untested**;
  `portgroup_test.go` covers only distributed.
- **Medium** — `cmd/` and `main.go` at 0.0% coverage.
- **Medium** — direct `pflag` dependency, two import sites (criterion 8 partial).
- **Low** ×2 — `RAM (GB)` renders `0.0` for sub-GB VMs, no MiB tier; `build/vcsim.log` shipped
  in the delivered tree despite `build/` being gitignored.

### Notable strengths, recorded for balance

Viper precedence is the best in the field: `stringFrom`/`boolFrom` gate on `f.Changed` so an
unset flag never masks env/file, backed by nine tests including `TestLoadFullPrecedence` and
`TestEnvOverridesFlag_NoFlagGiven`. Criterion 6 is handled by a single elegant path —
`f.NetworkList(ctx,"*")` returns both `Network` and `DistributedVirtualPortgroup`, filtered on
`vm.Network` membership — which covers standard and distributed without branching.

### Not run

**The behavioural mutation battery was not run**; `battery_*` fields left empty. `gosec` could
not be installed. No git history in the run directory, so test-churn forensics were not
possible.

## Score

**18 / 30 — PASS WITH CONCERNS.** 1 Critical (accuracy), 2 High, 4 Medium, 2 Low.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 2 | 6 of 8 criteria met. Criterion 4 **unmet** — transport fabricated, not derived. Criterion 8 partial. UPLINKS emits raw MO keys. |
| Integrity | 3 | No cheat, no skip, no forged evidence, no stubbed constant. Deductions: the transport unit test encodes factually false premises as expected values, and the sim test accepts both `unknown` and the fabricated `FC`. |
| Security | 4 | `insecure` default false; no credential leakage; timeout plumbed into every subcommand. staticcheck + govulncheck clean. `gosec` not run. |
| Performance | 2 | Two N+1 sites, one nested per-datastore-per-host — the rubric's named scale-killer. |
| Concurrency | 4 | `-race` clean; zero goroutines. Deduction: cancellation plumbed but **never probed** by any test. |
| Quality | 3 | gofmt/vet/staticcheck clean; genuine package separation; `%w` wrapping. Costs: `cmd/` 0.0% coverage, the uplink-key defect, and an identifier-semantics error at the heart of the primary feature. |

## Compare

Lowest-scoring of the three hosted API runs wired on 2026-09-01, and **the first submission in
this field to fail criterion 4 by fabrication rather than by omission**. Every prior failure of
that criterion was either an always-`unknown` stub (`gemma-4-31b`), a dead-code path
(`qwen-3.6-27b`), or a real traversal matching the wrong field (`qwen-3.8-flash`). This one
derives a specific protocol and gets it confidently wrong, which is a distinct and arguably
worse failure mode: the spec's permitted `unknown` degrade was available and not taken.

**Comparability caveats:**

1. **Effort `high`** — **not** comparable to `qwen-3.8-flash` (24) or `qwen-3.8-max` (25),
   both `xhigh`. Any score delta against those two confounds model with effort budget. A
   re-run at `xhigh` would be the controlled comparison.
2. **Hosted, third-party, precision undisclosed, snapshot unverified** — four provenance
   dimensions the matrix does not currently carry.
3. The C-1 severity call is overrulable and is the difference between this row reading 18 and
   reading FAIL.

## Remediate

## Rescore
