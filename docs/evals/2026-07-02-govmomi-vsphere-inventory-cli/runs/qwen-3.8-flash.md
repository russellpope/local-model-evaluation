---
name: qwen-3.8-flash
created: 2026-09-01
model: qwen3.8-flash — Alibaba **hosted API**, not local weights. Served via opencode provider `alibaba-token-plan`; serving precision **undisclosed**. The open-weight core is `Qwen/Qwen3.8-Flash-Next` (180B total / 6B active MoE, hybrid Gated DeltaNet + QSA attention, BF16 checkpoint 335.28 GiB / official FP8 172.78 GiB, Qwen Community License 1.0, open-weighted 2026-08-26). Per that card, "Qwen3.8-Flash is the official version based on Qwen3.8-Flash-Next with more production features, e.g. 1M context length by default, official built-in tools" — weight identity between endpoint and checkpoint is **not asserted**. Effort `variant = xhigh`, 88,467 reasoning tokens / 75,374 output across 263 messages, 2026-08-31 21:55:32 → 23:12:50.
stage: audited
score: 24 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — qwen-3.8-flash

## Wire

**Audit-only record.** The generation was driven by the operator; this record is written by the
audit side and observed nothing in flight. There are no pre-registered predictions, no gate
table, and no throughput measurement — none were taken, and none are reconstructed after the
fact.

Submission landed at `qwen-3.8-flash/` (untracked at audit time), 27 files, module
`local-model-evaluation/govmomi-cli`. Build verified at audit: `go build ./...` and
`go vet ./...` both exit 0 on Go 1.27.0 darwin/arm64 against govmomi
`v0.57.0-alpha.0.0.20260831023013-ec6493ddde50`.

### Provenance — reconstructed from the opencode store, not from memory

The run directory carried no provenance of its own. Identity was recovered by copying
`opencode.db` + WAL to scratch and querying read-only:

| field | value |
|---|---|
| provider | `alibaba-token-plan` (Alibaba hosted endpoint) |
| model id | `qwen3.8-flash` |
| variant (effort) | `xhigh` |
| messages | 263 (sessions `ses_fa4a8ef4cffe…` 240, `ses_fa4ad6f95ffe…` 23) |
| reasoning tokens | 88,467 |
| output tokens | 75,374 |
| window | 2026-08-31 21:55:32 → 23:12:50 |

**This is the third API-served entry in a repo named `local-model-evaluation`** (after
`gpt-5.5` and `ox-alpha-free`), and the first where the hosted variant is *documented to
differ* from the open weights. Recorded because the distinction is not recoverable from the
tree: nothing on disk says this was hosted.

**Effort was not captured at audit time and had to be backfilled.** Logged as an instrument
gap — see Compare.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context (no fresh-context subagent
dispatch). Raw report: [`qwen-3.8-flash/REVIEW.md`](../../../../qwen-3.8-flash/REVIEW.md).

**PASS WITH CONCERNS — 0 Critical, 2 High, 4 Medium, 2 Low.**

No cheat of any kind. Greps for `t.Skip`, `SkipNow`, `//go:build ignore`, `recover()` return
nothing; no tautological assertion, no stubbed value, no forged evidence — the tree ships no
`build.log` or `PROGRESS.md`, so nothing was claimed that could be forged. `gofmt -l`,
`go vet`, `staticcheck` and `govulncheck` all clean; `go test -race -count=1 ./...` clean;
`make verify` reproduces green end-to-end, exercising both port-group paths against a live
vcsim.

### H-1 — the topology traversal matches the wrong field

`scsiAdapterForLun` (`transport.go:186-196`) returns `ifc.Adapter` from `HostScsiTopology`;
`classifyExtent` (`:159-166`) compares it against `hba.GetHostHostBusAdapter().Device`. These
are different identifier spaces. VMware's own canned ESX data — shipped inside govmomi at
`simulator/esx/host_storage_device_info.go:17-18,165-167` — pairs
`Adapter: "key-vim.host.ParallelScsiHba-vmhba0"` with an HBA whose `Key` is that string and
whose `Device` is `"vmhba0"`. `Adapter` holds the HBA **key**; the code matches the **device
name**, so the comparison can never succeed on a real vCenter and every FC/iSCSI datastore
falls through to `unknown`.

**Verified by differential probe**, not by reading. Injecting an iSCSI HBA + `ScsiLun` +
`ScsiTopology` edge + VMFS extent into vcsim and calling the production `ListDatastores`:

| topology `Adapter` set to | result |
|---|---|
| HBA **key** (the real convention) | `LocalDS_0 -> "unknown"` |
| HBA **device name** | `LocalDS_0 -> "iSCSI"` |

The second result is what proves the traversal is real and reachable rather than dead code;
the first is what proves it cannot fire in production. NVMe (namespace match) and NFS
(filesystem shortcut) are unaffected.

**Charged High, not Critical, deliberately.** The rubric's Critical trigger is a classifier
that *always* returns `unknown` with no real FC/iSCSI/NVMe logic paired with a membership
test. This classifier has real branching and a genuine specific-protocol unit test with
negative cases (`HostParallelScsiHba`, `HostBlockHba`, `vmfs`, `""` → `unknown`). This follows
the precedent set at the 2026-08-16 Q8 close — *"Transport-unreachable charged High, not
Critical — for comparability… charging it Critical here would measure auditor drift rather
than model difference."*

### H-2 — the datastore sim assertion cannot fail

`inventory_sim_test.go:97-101` asserts `ds.Transport` ∈ {FC, iSCSI, NVMe, NFS, **unknown**}.
With `unknown` in the accepted set the assertion passes for any implementation, including one
returning `unknown` unconditionally. It is the structural blind spot that let H-1 ship.

In fairness: vcsim's default model backs datastores with `HostParallelScsiHba`, which
legitimately classifies as `unknown`, so a *correct* implementation also reports `unknown`
under `make verify`. Only injection separates the two — which is why the gap is invisible to
the submitted suite.

### Other confirmed findings

- **Medium** — `README.md:14` claims "`go.mod` requires ≥ 1.22" while `go.mod:3` declares
  `go 1.27.0`. False in the direction that matters: nobody on 1.22–1.26 can build it.
- **Medium** — whole-`config` over-fetch on hosts at three sites where only
  `config.storageDevice` / `config.network` are consumed.
- **Medium** — `github.com/spf13/pflag` directly imported (`config.go:9`), outside the allowed
  dependency set. Mitigating: it is cobra's own flag library.
- **Medium** — `cmd/` and `main.go` at 0.0% coverage.
- **Low** ×2 — near-vacuous `vm.Storage < 0` assertion; `verify.sh` asserts only a `^NAME`
  header on `--portgroup`, so it passes with zero rows.

### Not run

**The behavioural mutation battery was not run** for this submission. Disclosed rather than
omitted; `battery_*` front-matter fields are left empty. `gosec` could not be installed and
was not run — static security coverage is staticcheck + govulncheck only. No git history
exists in the run directory, so the test-churn forensics the rubric suggests could not be
performed; anti-cheat conclusions rest on the final tree state only.

## Score

**24 / 30 — PASS WITH CONCERNS.** 0 Critical, 2 High, 4 Medium, 2 Low.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 3 | 7 of 8 criteria met and verified. Criterion 4 **partial** — traversal real and reachable but wrong-field, so FC/iSCSI are inert in production. Criterion 8 partial (`pflag`); `go.mod` 1.27 against a 1.22+ bar. |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value, no forged log. Deductions: the membership assertion cannot catch H-1; README misstates the tree's own Go requirement. |
| Security | 4 | `insecure` default false and test-guarded; no credential leakage; timeout plumbed and probed; staticcheck + govulncheck clean. `gosec` not run. |
| Performance | 4 | No N+1 anywhere — one ContainerView + PropertyCollector per kind, explicit property lists, all views destroyed. Residual: whole-`config` host fetch ×3. |
| Concurrency | 5 | `-race` clean; zero goroutines created; cancellation **probed**, not assumed (`TestContextTimeoutHonored` drives an expired context and requires a non-nil error). |
| Quality | 4 | gofmt/vet/staticcheck clean, real three-layer separation, `%w` throughout. Costs: `cmd/` 0.0% coverage, the key/device confusion at the heart of the primary feature. |

## Compare

**Best first-audit result by any non-frontier model in this field at the time of scoring** — above
`qwen3.8-27b-bf16` (23, the prior best local first-audit) and one below `ornith-1.0-35b-fp16`
(25), which needed three remediation rounds to get there. No remediation was run here.

**Comparability caveats, all of which cut against ranking this row naively:**

1. **Hosted, not local.** Third API-served entry in this repo. Serving precision undisclosed.
2. **The artifact is the API product, not the open checkpoint.** Qwen documents Flash as
   *"based on"* Flash-Next with added production features including **official built-in
   tools** — material for an agentic build task, and weight identity is never asserted.
3. **Effort `xhigh`.** Directly comparable to `qwen-3.8-max` (also `xhigh`, same provider);
   **not** comparable to `deepseek-v4-flash-0731` (`high`). Local runs in this eval carry no
   effort variant at all, so every local-vs-hosted comparison is confounded on effort,
   precision, and host simultaneously.

**Instrument gap, charged to the auditor.** Effort level was not captured at audit time and had
to be backfilled from the opencode store afterwards; the initial cross-run comparison drawn
against `deepseek-v4-flash-0731` was made without it and was confounded. Effort should be a
first-class Wire field going forward, backfillable from `$.variant` for hosted runs and
explicitly `n/a` for llama.cpp-local ones.

## Remediate

## Rescore
