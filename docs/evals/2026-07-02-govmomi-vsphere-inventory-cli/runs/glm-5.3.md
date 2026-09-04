---
name: glm-5.3
created: 2026-09-01
model: glm-5.3 (Z.ai / Zhipu **GLM-5.3** flagship — distinct from `glm-5.3-flash`) — hosted endpoint via opencode provider `zai-coding-plan`; serving precision **undisclosed**, no open-weight checkpoint identity verified for this endpoint. Effort `variant = max`, 44,063 reasoning tokens / 49,163 output across 164 messages, 2026-09-01 15:27:46 → 15:59:00 (31 min).
stage: audited
score: 23 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — glm-5.3

## Wire

**Audit-only record.** Generation driven by the operator; nothing observed in flight.

Submission at `glm-5.3/vsphere-inventory/`, module `vsphere-inventory`, 33 files, govmomi
v0.46.1. Build verified at audit: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean on
Go 1.27.0 darwin/arm64.

### Provenance — captured at audit time

| field | value |
|---|---|
| provider | `zai-coding-plan` (Z.ai direct) |
| model id | `glm-5.3` |
| variant (effort) | `max` |
| messages | 164 (cwd-scoped) |
| reasoning tokens | 44,063 |
| output tokens | 49,163 |
| window | 2026-09-01 15:27:46 → 15:59:00 |

Run immediately after `glm-5.3-flash` on the same provider at the same effort setting, which
makes the two a near-controlled pair — see Compare.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context. Raw report:
[`glm-5.3/REVIEW.md`](../../../../glm-5.3/REVIEW.md).

**PASS WITH CONCERNS — 0 Critical, 2 High, 5 Medium, 0 Low.**

No cheat of any kind: no `t.Skip`, no build-tag fencing, no `recover()`, no tautologies, no
stubbed values, no forged evidence. gofmt/vet/staticcheck clean; `-race` clean with no
goroutines, channels or `sync` primitives in non-test code; `make verify` reproduces green
through a full vcsim end-to-end pass.

### Two field-firsts

- **Criterion 8 satisfied exactly** — `go.mod` requires only cobra, viper and govmomi, and a
  grep for non-allowed imports across the tree returns nothing. Every other submission in this
  field carries a direct `pflag` dependency.
- **`cmd/` is tested and passes** — `cmd/config_precedence_test.go` exercises
  flag > env > file > default at the Cobra layer. No prior submission has any `cmd/` coverage;
  this closes the most common Medium in the series.

Also notable: DVS LACP is read from `len(vmware.LacpGroupConfig) > 0` (`switches.go:185`) — the
**correct** field, unlike its own Flash sibling, which used `LacpApiVersion`.

### H-1 — the transport classifier is unreachable from production

`datastores.go:74-76` keys the per-host storage cache by the **`HostStorageSystem`** managed
object reference (`storage[s.Reference()]`, where `s` is a `mo.HostStorageSystem`).
`datastores.go:127` looks it up by the **`HostSystem`** reference (`storage[mount.Key]`, where
`mount.Key` is a `DatastoreHostMount.Key`). These are different MoRef types with different
values, so the lookup **never hits**: `info` is always nil, `continue` fires for every mount,
and `classifyVmfs` returns `TransportUnknown` unconditionally. Every VMFS datastore reports
`unknown` on any real vCenter, and the whole body of transport work — `LunDescriptor`,
`ClassifyTransport`, the `MultipathInfo` join, the NVMe topology fallback — is dead code.

**Proven differentially.** Injecting a software iSCSI HBA, a matching `ScsiLun`, a
`MultipathInfo` path binding them, and a VMFS extent, then calling production `ListDatastores`:

| tree | result |
|---|---|
| as submitted | `LocalDS_0 -> "unknown"` |
| map correlated back to the host ref (auditor patch) | `LocalDS_0 -> "iSCSI"` |

The first probe returning `unknown` was **not** taken at face value — the negative control
(changing only the map key) is what establishes that the classifier itself is correct and the
defect is purely the key mismatch.

**Charged High, not Critical, for comparability.** The 2026-08-16 close records the rule
explicitly — *"Transport-unreachable charged High, not Critical… charging it Critical here
would measure auditor drift rather than model difference."* This is the **third instance of the
same failure family**, after `qwen3.8-27b-bf16` (a `vmfsUUID` parse) and `qwen-3.8-flash` (a
key/device field mismatch), and is charged identically.

*The classifier itself is the most carefully reasoned in the field.* `ClassifyTransport` ranks
its evidence explicitly — HBA **type** first, then NVMe adapter type, then NVMe naming
heuristics — with a doc comment stating why ("a fabric adapter is positive evidence while
naming conventions are only a hint"). It contains no `naa.` → FC rule, the identifier-prefix
fallacy that made `deepseek-v4-flash-0731` fabricate. `transport_test.go` covers 12+ cases
asserting specific protocols with three genuine `unknown` negatives.

### H-2 — duplicate switch rows on every multi-host inventory

The listing emits one row per (host, switch, port group) with **no host column and no
deduplication**. Against a 4-host model, `vSwitch0 / Management Network` appears four times,
byte-identically; the operator cannot tell the rows apart or know why they repeat. Same defect
family as `ox-alpha-free`'s phantom multi-host row (High) and the byte-identical duplicate
`vSwitch0` rows recorded against `qwen3.8-27b-bf16`. `switches_test.go` has no row-count or
uniqueness assertion, so the suite cannot see it.

### The five Mediums

- **M-1** — `datastores_test.go:28-40` asserts transport membership including `unknown`, so it
  passes for an implementation that never classifies anything. This is the negative control
  that would have caught H-1, and its absence is the universal blind spot across this field.
  vcsim populates no extents or multipath info by default, so `make verify` also reports
  `unknown` for a *correct* implementation — only injection separates the two.
- **M-2** — VLAN renders `none` for VLAN 0 where siblings render `0`.
- **M-3** — DVS `UPLINKS` renders `unknown`.
- **M-4** — `internal/render` at 0.0% coverage.
- **M-5** — cancellation plumbed (`cmd/root.go:109`) but never probed by a test.

### Not run

**The behavioural mutation battery was not run**; `battery_*` fields left empty. `gosec` could
not be installed. No git history in the run directory.

## Score

**23 / 30 — PASS WITH CONCERNS.** 0 Critical, 2 High, 5 Medium.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 3 | Criterion 4 unmet in production — classifier genuine, well-tested, unreachable (H-1). Duplicate switch rows (H-2). Offsetting: criterion 8 **fully met for the first time**; LACP reads the correct field; committed storage correct; both port-group paths work and are tested. |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value. `ClassifyTransport` table-tested across 12+ cases with three genuine `unknown` negatives. Deduction: M-1. |
| Security | 4 | `insecure` default false at both Viper default and flag; no credential leakage; timeout plumbed at root, logout on its own 10s budget. staticcheck clean; govulncheck reports nothing reachable. `gosec` not run. |
| Performance | 4 | No N+1 — host storage systems fetched in one batched `Retrieve` with an explicit property list; views destroyed via helpers. Deduction: that fetch is entirely wasted work, since H-1 discards every result. |
| Concurrency | 4 | `-race` clean across all packages including `cmd`; no goroutines, channels or `sync` in non-test code. Deduction: cancellation never probed. |
| Quality | 4 | All static gates clean; cleanest layout in the field (`internal/render` split out, `vlan.go` isolated with its own tests); **`cmd/` tested — field first**. Costs: the MoRef key mismatch is a subtle correctness trap; `internal/render` uncovered. |

## Compare

Level with `glm-5.3-flash` and `qwen3.8-27b-bf16` at 23; below `ox-alpha-free` and
`qwen-3.8-max` (25) and `qwen-3.8-flash` (24); above `deepseek-v4-flash-0731` (18).

### The near-controlled pair: flagship vs Flash

Same vendor, same provider, same `variant = max`, same day, ~45 minutes apart. Unlike the
`ox-alpha-free` / `glm-5.3-flash` repeat pair — where reasoning budgets differed 4.4× despite
matching labels — these two are of the same order.

| | `glm-5.3-flash` | `glm-5.3` |
|---|---|---|
| messages | 132 | 164 |
| reasoning tokens | 55,066 | 44,063 |
| duration | 45 min | 31 min |
| **score** | **23 / 30** | **23 / 30** |
| findings | 0C, 1 High, 5 Medium | 0C, 2 High, 5 Medium |

**Identical totals, disjoint failure modes.** Flash got the production wiring right but read
the wrong LACP field and silently aggregated switch rows across hosts; the flagship reads LACP
correctly, uniquely satisfies the dependency constraint, uniquely tests `cmd/` — and then loses
its entire transport feature to a one-line key mismatch. Neither dominates.

**The flagship does not beat its own Flash variant on this task.** That is the third
independent signal this week pointing the same direction:

1. `qwen-3.8-max` (2.4T total / 95B active) beat `qwen-3.8-flash` (180B / 6B) by **one point**
   at matched `xhigh` effort.
2. The `ox-alpha-free` / `glm-5.3-flash` repeat pair — the same model twice — landed **2 points
   apart**.
3. `glm-5.3` ties `glm-5.3-flash` exactly, failing in entirely different places.

Taken together: **scores in the 23–25 band are being driven by which specific wiring bug a run
happens to ship, not by model capability.** The rubric appears unable to resolve differences
among frontier-class models on this task. That is a benchmark finding, not a model finding, and
it belongs in the v2 design discussion — the transport criterion in particular has now produced
three "genuine classifier, unreachable from production" results (`qwen3.8-27b-bf16`,
`qwen-3.8-flash`, `glm-5.3`) plus one fabrication (`deepseek-v4-flash-0731`), which suggests the
criterion is testing plumbing luck more than understanding.

## Remediate

## Rescore
