---
name: ox-alpha-free
created: 2026-09-01
model: **GLM-5.3-Flash** (Z.ai / Zhipu), run pre-reveal under the cloaked name `ox-alpha` — opencode provider `opencode`, model id `x-preview-f-free`; serving precision **undisclosed**. 320B total / 18B active MoE, hybrid sparse + linear attention with Manifold-Constrained Hyper-Connections (mHC), first natively multimodal GLM-5 member, MIT, 1M context. Appeared on OpenRouter as `stealth/ox-alpha` on 2026-08-20 (free tier, 23.2T tokens in six days); Z.ai confirmed the identity on 2026-08-26 — **five days after this audit**. Effort `variant = max`, 12,542 reasoning tokens / 58,938 output across 177 messages, 2026-08-20 23:31:06 → 2026-08-21 13:14:02.
stage: audited
score: 25 / 30
battery_version: spine gate go mutate — 26-probe spec, every `find` literal extracted by line range and asserted unique in its file
battery_verdict: kill rate (scorable) 11/22 = 50% · kill rate (raw) 12/26 = 46% · control GREEN before and after, corpus diff -r identical
battery_results: 14 survivors, 3 distinct causes — `internal/cmd` has zero test files (5 probes); simulator fixtures cannot distinguish the mutated value from the honest degrade the spec permits (7); report-only lifecycle/security probes outside `cmd/` (2)
---

# Run — ox-alpha-free

## Wire

**Audit-only record, wired retroactively on 2026-09-01.** The audit itself was performed
2026-08-21; this ledger entry was written after the fact, at the same time as the
`glm-5.3-flash` repeat measurement, so the pair sits together.

Submission at `ox-alpha-free/vsphere-inventory/`, frozen as delivered. Audited on Go 1.26.7
darwin/arm64 against govmomi v0.44.0.

### Provenance — reconstructed, with a method note

| field | value |
|---|---|
| provider / model id | `opencode` / `x-preview-f-free` |
| true identity | **GLM-5.3-Flash** (revealed 2026-08-26) |
| variant (effort) | `max` |
| messages | 177 |
| reasoning tokens | 12,542 |
| output tokens | 58,938 |
| window | 2026-08-20 23:31:06 → 2026-08-21 13:14:02 |

**The naive query is wrong and was corrected.** Model id `x-preview-f-free` was used across
**11 sessions and five different project directories** over six days, totalling 436 messages.
Scoping by `$.path.cwd` to the run directory *and* cutting at the audit date isolates the
generation run at 177 messages — a model-id-level count overstates it by 2.5×. The directory
was also revisited on 2026-08-26 (7 messages at `variant = null`), after the submission was
frozen and audited; that activity is excluded and did not touch the audited tree.

### This audit was genuinely name-blind

`stealth/ox-alpha` appeared on OpenRouter 2026-08-20; this run and audit completed 2026-08-21;
Z.ai revealed the model as GLM-5.3-Flash on 2026-08-26. The audit therefore predates the reveal
by five days, and no vendor-name bias could have entered the scoring. **This is the only
name-blind audit in the field** and is worth preserving as a methodological point, not just
trivia.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`. Raw report:
[`ox-alpha-free/vsphere-inventory/REVIEW.md`](../../../../ox-alpha-free/vsphere-inventory/REVIEW.md).

**PASS WITH CONCERNS — 0 Critical, 2 High, 7 Medium, 6 Low.**

No cheat of any kind. Every claimed gate reproduces; the test suite contains no skips, no
tautologies, and no assertion weakened to accommodate broken code. There is no `build.log` or
`PROGRESS.md`, so nothing was claimed that could be forged.

**Criterion 4 is not a stub, and this was proven by injection** — feeding an iSCSI HBA,
`ScsiTopology` and VMFS extent into the simulator makes the production `ListDatastores` return
`iSCSI` for that datastore and `unknown` for the other. The HBA → LUN → extent traversal is
real and reachable from production code, not decoration around a constant. **Criterion 6's
standard-port-group half also works** — re-backing a VM's NIC onto `VM Network` makes the
lookup return exactly that VM (the submitted suite asserts only that the standard lookup
returns *empty*, which is correct behaviour but missing proof). **Criterion 7's timeout is
plumbed** — `VSPHERE_TIMEOUT=1ms` produces a wrapped `context deadline exceeded` and exit 1.

### The two Highs

- **H-1 — phantom port-group row on every multi-host inventory.** `vswitches.go:99-106`: the
  cross-host dedup `continue`s before `rows++`, so the "this switch has no port groups"
  fallback fires on hosts whose rows were already emitted, appending a spurious placeholder
  row to every multi-host listing.
- **H-2 — `make verify`'s port-group check cannot fail.** The Makefile picks the port group in
  a way that makes the assertion structurally incapable of failing, so the gate green-lights
  regardless of behaviour.

Plus a README "sample run" labelled as `make verify` output that does not match what
`make verify` actually emits — charged as the sole Integrity deduction, not as fabrication in
runtime output.

### Behavioural mutation battery — RUN for this submission

The only run in this field with a battery result recorded. Runner `spine gate go mutate`,
26-probe spec, every `find` literal extracted from the tree by line range and asserted to occur
exactly once in its file (never retyped). Control: unmutated tree GREEN before and after;
corpus `diff -r` identical afterwards.

- **kill rate (scorable): 11/22 = 50%** (excluded: 4 report-only, 0 NO-SITE, 0 BUILD-ERR)
- **kill rate (raw): 12/26 = 46%**

**Distinct-cause summary: 14 survivors, 3 causes** — `internal/cmd` has zero test files
(M2b/M3/M4/M5/M9); simulator fixtures cannot distinguish the mutated value from the honest
degrade the spec permits (M1b/M7/M10b/M11b/M12/M12b/M14); report-only lifecycle/security probes
outside `cmd/` (M8/M9b).

Notable rows: `M1-invoke-clf` **KILLED** (`ClassifyTransport` short-circuited to constant `FC`)
but `M1b-chain-stub` **SURVIVED** (the whole extent→LUN→HBA traversal bypassed, every block
datastore degrading to `unknown`, with a green suite). `M2-wire-envprefix` and `M3b-flag-values`
KILLED; `M3-flag-portgroup`, `M4-col-drop-last` and `M5-col-order-ds` all SURVIVED.

## Score

**25 / 30 — PASS WITH CONCERNS.** 0 Critical, 2 High, 7 Medium, 6 Low.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 4 | All 8 criteria met; transport traversal, timeout and flag precedence verified beyond the suite. Costs: phantom row on multi-host, `pflag` outside the allowed dependency set, `go 1.26.7` against a stated 1.22+ bar. |
| Integrity | 4 | No gamed test, no stub, no fabricated value, no forged log. Sole deduction: the README run-sample is labelled `make verify` output and is not. |
| Security | 4 | `insecure` default false *and* test-guarded; no credential leakage; timeout honoured; staticcheck/govulncheck clean; 3 gosec G115 all false positives. Forcing TLS-off at the call site is invisible to the suite. |
| Performance | 4 | No N+1 anywhere — single ContainerView + PropertyCollector with explicit property lists, all views destroyed. Costs: `config.storageDevice` fetched fleet-wide per `datastores` call; whole `summary` fetched for VMs. |
| Concurrency | 5 | `-race` clean; zero goroutines created; cancellation **probed**, not assumed — `VSPHERE_TIMEOUT=1ms` → `context deadline exceeded`, exit 1. |
| Quality | 4 | gofmt/vet/staticcheck clean, real three-layer separation, `%w` wrapping. Costs: `internal/cmd` and `main.go` at 0.0% coverage; placeholder-row logic subtly wrong; no MiB tier. |

## Compare

Ties `qwen-3.8-max` and `ornith-1.0-35b-fp16` at 25 — but reached it on a **first audit with no
remediation**, where ornith needed three rounds. Below `ornith-1.0-397B` (28), `gpt-5.5` (29
after r1) and the `claude-code-opus-4.7` reference (30).

At the time of audit this cleared a bar only three prior runs had: zero Criticals. The others
were `claude-code-opus-4.7`, `gpt-5.5` (r1) and `ornith-1.0-397B`.

**Not a local model.** The original `ox-alpha-free/README.md` framed this as a local result and
compared it against "the best local arc" — that framing is wrong and is superseded here. This
was a 320B frontier-class model served over a hosted API, the same category correction already
applied to `gpt-5.5` on 2026-07-09.

**Repeat measurement.** Re-run as `glm-5.3-flash` on 2026-09-01 under the model's true name,
scoring **23**. The pair is this field's only test–retest, and it is **less controlled than the
matching `max` variant labels suggest** — actual reasoning expenditure differed by 4.4×
(12,542 here vs 55,066), so the variant label is not portable across providers. Full analysis
in the `glm-5.3-flash` record's Compare section.

**Auditor caveat.** This audit predates several methodological developments in the probe set
used on the 2026-09-01 runs. A blind re-audit with the current probes would establish how much
of the 25-vs-23 spread is model variance and how much is auditor drift — the cheapest available
test of whether the benchmark or the auditor is the noisy component.

## Remediate

## Rescore
