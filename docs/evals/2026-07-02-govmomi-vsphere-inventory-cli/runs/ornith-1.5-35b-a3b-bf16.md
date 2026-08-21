---
name: ornith-1.5-35b-a3b-bf16
created: 2026-08-20
model: Ornith-1.5-35B-A3B (local, BF16 GGUF)
stage: audited
score: 12 / 30
---

# Run — ornith-1.5-35b-a3b-bf16

## Wire

**Status: pre-registration.** `ornith-1.5-35b-a3b-bf16/` is seeded with the eval prompt
only; no submission exists yet. The rubric (`govmomi-cli-audit-prompt.md`) is **not** staged
in the workspace and must never be.

First rung of a four-rung arc agreed 2026-08-20: 35B BF16 → 35B Q8_0 → 9B BF16 → 9B Q8_0,
each a full eval, then MTP throughput testing across Qwen 3.8 and Ornith.

### Weights

`/Users/ldh/models/Ornith-1.5-35B-A3B-GGUF/Ornith-1.5-35B-BF16.gguf` — 71,066,994,240 bytes,
size-verified against the CDN `x-linked-size` on download. First-party
`ornith-ai/Ornith-1.5-35B-A3B-GGUF`; no third-party requant. Provenance, full manifest and
the MTP findings: [`../artifacts/ornith-1.5-download-manifest.md`](../artifacts/ornith-1.5-download-manifest.md).

Model released 2026-08-19, MIT. 35.51 B params, 256 experts, 8 active per token. Hybrid
backbone — 30 `linear_attention` / 10 `full_attention` layers. Multimodal (`mmproj` pulled
but unused; this eval is text-only).

### Endpoint

llama.cpp `llama-server`, build **10470** (`34af94cd9`, Homebrew, installed 2026-08-17).

```
llama-server -m .../Ornith-1.5-35B-BF16.gguf \
  --alias ornith-1.5-35b-a3b-bf16 \
  -c 262144 -ngl 99 -fa on --jinja \
  --temp 0.6 --top-p 0.95 --top-k 20 \
  --host 127.0.0.1 --port 1234 -v
```

Driven from opencode via the `llamacpp-local` provider (`http://localhost:1234/v1`), model
id `ornith-1.5-35b-a3b-bf16`. Verbose server log: `/Users/ldh/models/logs/ornith-1.5-35b-a3b-bf16.log`.

LM Studio previously held port 1234 (serving `gemma-4-12b` + an embedding model) and was
stopped by the operator before this server was started.

### Sampler — matches Ornith 1.0 exactly

`temperature 0.6, top_p 0.95, top_k 20`, context 262,144. This is simultaneously the vendor
recommendation on the 1.5 model card for general tasks, and byte-identical to what every
Ornith 1.0 run used, so the 1.0 → 1.5 comparison holds. Corroborated three ways:
`ornith-1.0-35b-fp16/notes.txt:39`, `docs/handoffs/2026-07-08-ornith-397b-fp8-experiment.md:89`,
`docs/handoffs/2026-06-28-ornith-remediation-round2-rescore.md:121`.

Recorded inline here deliberately — eight other run records carry their sampler, but neither
Ornith 1.0 record does, and it only survived in handoffs and `notes.txt`.

The card's `temperature 1.0` is its *benchmark-reproduction* setting and was **not** used;
this field measures practical coding capability, not benchmark reproduction.

### Run condition — compaction is charged to the model, by design

Runs go straight through with no operator intervention. If the model spawns a subagent to
conserve context, that is its own call and counts; if it sprawls and compacts, that counts
too. Context management is treated as part of coding capability rather than as a nuisance
variable to control, and it is the only protocol that stays consistent without babysitting
or maintaining a prompt library. Prior runs have reached ~240k tokens without compacting.

Consequence to carry into scoring: run-to-run spread is roughly **3 points** — Qwen3.8-27B
Q8_0 scored 20 with one compaction at 230,641 tokens and 23 without (second sample recorded
in the `deepthought` repo under the spine-harness work, not here). **A precision effect
smaller than 3 points is therefore not detectable at n=1.** The BF16-vs-Q8 question is
answered across this arc by looking for a consistent direction over two independent
contrasts (35B and 9B), not by any single pair.

### Load verified against the live server

- `general.architecture = qwen35moe`, `n_ctx = n_ctx_seq = 262144`, `n_ctx_train = 262144`
- `model params = 35.51 B`; resident **69.9 GiB** on a 128 GiB M5 Max
- `qwen35moe.nextn_predict_layers = 1`, with `blk.40.nextn.*` tensors present and logged as
  `unused … ignoring` — expected, since `--spec-type draft-mtp` is deliberately **not** set
  for scoring runs. This is the MTP head confirmed in the live artifact rather than inferred
  from the HF config.
- Generation: returns `OK` on an exact-reply probe, `finish_reason: stop`
- Reasoning parser: `<think>` content is split into `reasoning_content`, leaving `content`
  clean. A first probe at `max_tokens=24` returned empty `content` with 24 completion
  tokens — truncation inside the think block, not model collapse. Worth knowing before
  reading any short reply as a failure.
- Tool calling under `--jinja`: emits a well-formed `tool_calls` entry with valid JSON
  arguments, `finish_reason: tool_calls`. This is the capability the eval actually depends
  on and it was verified before the run, not assumed.
- Prompt cache active (`cached_tokens` non-zero on the second probe).

Build of the submission tree: **not applicable yet** — no submission. To be recorded when
the run lands.

## Audit

**FAIL — 1 Critical, 7 High.** Raw report: [`REVIEW.md`](../../../../ornith-1.5-35b-a3b-bf16/REVIEW.md).

Four independent fresh-context auditors, each given ONLY the spec, the rubric and the frozen
tree — never the model's self-assessment, never each other's findings, never the driver's
inline pass. Each opened with a requirements-attack on the instrument and was required to prove
defects with runnable probes.

**Critical — the tested transport classifier is dead code.** `transport.go:21 classifyTransport`
carries genuine FC/iSCSI/NVMe/NFS branching and is the sole subject of the criterion-4 test (12
cases asserting specific protocols). It has **no production caller**. The live path is
`inventory.go:136 deriveTransport → :177 transportFromHost`, which has no test of its own.
Negative control: replacing `deriveTransport`'s entire body with `return "unknown"` leaves
`go test ./...` green. The rubric §B cheat signature verbatim. Found independently by all four
auditors, including the two not assigned integrity.

Supporting Highs: `hostAdapterKey` (`inventory.go:215`) type-switches on the base
`*types.HostHostBusAdapter` while callers pass the embedding `*HostFibreChannelHba` /
`*HostInternetScsiHba`, so it always returns `""` and the LUN→adapter join is permanently dead
(a mixed FC+iSCSI host returns `"iscsi"` where the ambiguity guard should return `unknown`);
production emits lowercase `fc`/`iscsi`, outside the spec's vocabulary and rejected by the
project's own `validTypes` map; NVMe unreachable on the live path; `--timeout` created then
dropped (`root.go:63` shadows locally — a 30s run under `--timeout 1s`, proven with a proxy that
delays everything except login); `AutomaticEnv` without `SetEnvPrefix` lets ambient
`$INSECURE`/`$URL`/`$PASSWORD` override the documented `VSPHERE_*` vars and silently disable TLS
verification; multi-datacenter vCenter hard-fails all three subcommands; nested DVS silently
dropped.

**What is real, not fake.** Genuinely executed against vcsim — built the harness, diagnosed the
separate-module problem correctly, ran all three subcommands. Zero `t.Skip`. `-race` green.
`make verify` reproduces end to end and tears the simulator down leaving `go.mod` byte-identical.
VM storage reads the correct `Summary.Storage.Committed`. `insecure` defaults false and is
enforced. Passwords never logged; `Logout` on every exit path. Standard-switch `USED` is
genuinely derived from `len(pg.Port)` — 1 and 0 against 1536, not Ornith 1.0's fabrication.
`TestVMsForStandardPortGroup` forces the standard path with a `Portgroup: 0` topology and
asserts a bidirectional exact set. **No forged evidence anywhere** — the missing run-confirmation
deliverable was simply absent, not faked.

**Honest degrades confirmed and NOT charged:** DVS `USED=0` (probe: vcsim's `FetchDVPorts`
returns ports with `Connectee=nil`); datastore `TYPE=unknown` under vcsim (only
`HostParallelScsiHba`/`HostBlockHba` present); the separate vcsim harness module; `pflag`.

### Mutation battery

**Deviation disclosed:** the spine class-based battery (`spine gate go mutate`) was **not** run,
so the standard ten-class verdict matrix and the scorable/raw kill-rate pair are unavailable in
their usual form. What was run instead is a 28-mutation hand-authored battery, broader in
coverage but not class-mapped. Rerun the spine battery before comparing this row against
class-matrix rows elsewhere in the field.

**13 caught / 28 applied = 46.4% kill rate.** Caught: descending sort, dropped VM, always-unknown
`classifyTransport`, `UsedBytes = capacity`, TYPE = filesystem type, both LACP directions
(always-`N/A` and fabricated-`enabled`), standard and distributed switches dropped, both
`vmConnectedTo` branches, inverted config precedence, unhonoured default timeout, 1000-based
`formatBytes`.

**15 survived, 2 distinct causes:** (1) *the production transport path is untested* — gutting
`deriveTransport` or `transportFromHost` entirely both survive, as does hardcoding TYPE to `"FC"`;
(2) *the assertions are algebraic identities or all-inclusive sets* — `inventory.go:132` clamps
free to capacity so the capacity assertions cannot fail, `vswitches.go:81/134` rewrite USED down
and PORTS up purely to satisfy `used <= ports`, and both port-group tests expect *all* VMs so a
filter that always returns true is indistinguishable from a working one. Also surviving:
committed→uncommitted storage (criterion 3's headline trap is unpinned), UPLINKS, VLAN, vCPU and
RAM all fabricatable. `internal/command` has zero test files (0.0%).

## Score

**12 / 30 — CONTESTED, NOT FIELD-COMPARABLE. Do not rank this row against the others until the
calibration question is resolved.** See
[`docs/handoffs/2026-08-20-ornith-1.5-bf16-audit-and-calibration-defect.md`](../../../handoffs/2026-08-20-ornith-1.5-bf16-audit-and-calibration-defect.md).

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 2 | Criteria 1, 3, 6, 7, 8 met; criterion 4 unmet on the production path; criterion 5 unimplemented with the data present; criterion 2 partially unmet |
| Integrity | 2 | Criterion-4 evidence tests a function production never calls; gutting the live path leaves the suite green |
| Security | 2 | TLS default, error hygiene and no-logging correct; ambient env vars override the documented ones and disable cert verification |
| Performance | 2 | Every retrieval path N+1, no `ContainerView`; 36× measured penalty vs a batched baseline |
| Concurrency | 2 | No races, no leaks, `Logout` verified on all paths; the dimension's one substantive requirement — an honoured timeout — is dropped |
| Quality | 2 | Clean surface, 13/28 mutations caught; 15 survived, two deliverables absent |

**Why the total is contested.** The rubric defines **no score anchors** —
`govmomi-cli-audit-prompt.md:233` says only "score 1–5", and a grep for anchor language returns
zero. Four auditors calibrated independently and all six dimensions landed on exactly 2, which
reads as a shared default rather than six measurements. Against `qwen3.8-27b-bf16` (23/30) the
same evidence classes were scored very differently: qwen took **Security 5** for "TLS verify
default false, no credential leakage, gosec clean" (hygiene Ornith also has), **Concurrency 5**
for "Race-clean." alone (Ornith is also race-clean), **Performance 3** for the same N+1 defect
classes, and **Quality 4** explicitly *declining* to charge mutation survivors to the submission.
Ornith was audited at far greater depth — four parallel auditors with reverse-proxy timeout
probes, SOAP-trace round-trip counting and a 28-mutation battery, versus qwen's blind+synthesis
pair. More scrutiny found more defects; that is an instrument change, not a model regression.

**The FAIL verdict is not contested** — the Critical is real, independently confirmed four times,
and qwen3.8-27b-bf16 recorded 0 Criticals against this submission's 1.

## Compare

Against the field, with the comparability caveat above in force.

Behaviourally this is the strongest local submission recorded here on *process*: it explored
`vim25/types` by targeted line-seek rather than bulk read (368 type-file opens, 190 `sed`/`grep`
line-seeks, **zero** `read` calls — see `artifacts/cross-run-reasoning-failure-modes.md` §5),
executed live against a simulator it had to diagnose and build itself, spun up multiple vcsim
topologies including a zero-portgroup edge case, found and fixed a real cobra
`PersistentFlags()`-vs-`Flags()` bug during its own verification, and shipped no forged evidence.
Ornith 1.0's failures were fabricated columns and dead flags; those specific classes are gone.

The failure that remains is subtler and, by this rubric, worse: a correct classifier written,
tested against the spec's exact prescribed assertion, and then **not wired to the binary**. Where
Qwen3.6-27B shipped a dead-code classifier feeding nothing, this ships a *good* dead classifier
beside a *broken* live one — a shape that survives casual review precisely because the tests are
real and green.

Direct rival `qwen3.8-27b-bf16` scored 23 with 0 Criticals under a shallower audit. Whether
Ornith 1.5 is genuinely worse or merely better-examined is the open question, and it is the
reason the total above is flagged. Resolving it needs qwen3.8 re-audited at this depth, or the
whole field re-scored against anchors that do not yet exist.

## Remediate

**No remediation round will be run — a deliberate operator decision (2026-08-20), not an
omission.** Rationale: the calibrated position (~16 under the proposed anchors, see
`artifacts/2026-08-20-score-calibration-quantification.md` §5) sits below the pre-registered
>18 slot bar and below the predecessor's era-relative position; remediation would measure
patience, not adoption fitness. The v1 ledger is frozen under option 3-lite (flagged
non-comparable, remap not applied); Ornith 1.5's remaining rungs (35B Q8, 9B BF16/Q8) fold into
the successor instrument rather than running under v1. `stage` stays `audited`; `score:` stays
**12 / 30, CONTESTED**, unchanged.

## Rescore

Not applicable — no remediation round (see Remediate). 12 / 30 (contested) is final for v1.
