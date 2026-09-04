---
name: opencode-qwen3.8-flash
created: 2026-09-02
model: qwen3.8-flash — Alibaba **hosted API**, not local weights. Served via **opencode** provider `alibaba-token-plan`; serving precision **undisclosed**. Same endpoint, model id and effort as the `qwen-3.8-flash` run — this row is a **matched-effort repeat**, dispatched to measure opencode run-to-run variance. Effort `variant = xhigh`, 72,292 reasoning tokens / 49,983 output across 179 assistant messages, 2026-09-01 23:14:41 → 2026-09-02 00:24:37 (70 min). Run **concurrently** with `omp-qwen-3.8-flash-run2` against the same provider account.
stage: audited
score: 18 / 30
battery_version:
battery_verdict:
battery_results:
---

# Run — opencode-qwen3.8-flash

## Wire

**Audit-only record.** Generation driven by the operator; the audit side observed nothing in
flight.

Submission landed at `opencode-qwen3.8-flash/`, 23 Go files + `Makefile`, `README.md` and a
`vcsim`-backed verify target; module `github.com/local-model-evaluation/vsphere-inventory`.
Build verified at audit: `go build ./...` and `go vet ./...` exit 0 on Go 1.27.0 darwin/arm64
against govmomi **v0.56.0**. No `PROGRESS.md`, no `build.log`.

Provenance recovered from the opencode store (read-only copy, scoped by `$.path.cwd`):

| field | value |
|---|---|
| provider / model | `alibaba-token-plan` / `qwen3.8-flash` |
| variant (effort) | `xhigh` |
| assistant messages | 179 |
| reasoning tokens | 72,292 |
| output tokens | 49,983 |
| window | 2026-09-01 23:14:41 → 2026-09-02 00:24:37 |

**Concurrency caveat.** Dispatched alongside `omp-qwen-3.8-flash-run2` against the same
provider account. Token accounting is per-response and provider-reported, so reasoning/output
comparisons stand; **wall-clock and TTFT comparisons against serially-run rows are confounded**
and are not drawn.

## Audit

Adversarial pass against `govmomi-cli-audit-prompt.md`, in-context (no fresh-context subagent
dispatch, per the operator's standing instruction — the same method as every other run in this
comparison set). Raw report:
[`opencode-qwen3.8-flash/REVIEW.md`](../../../../opencode-qwen3.8-flash/REVIEW.md).

**PASS WITH CONCERNS — 1 Critical (accuracy), 2 High, 3 Medium, 2 Low.**

No cheat. `t.Skip`/`SkipNow`/`//go:build ignore`/`recover()` return nothing and
`spine gate go tskip` reports no findings; no forged evidence (the tree ships no `build.log`
or `PROGRESS.md`). Dependency set is clean — only govmomi, cobra and viper appear in any
import. `gofmt`, `go vet`, `staticcheck`, `govulncheck` clean; `go test -race -count=1` clean;
`make verify` reproduces green end-to-end.

### C-1 (Critical, accuracy) — `naa.` → FC, independently reproducing deepseek's fabrication

`transport.go:43-44` maps any `naa.` canonical name to FC, and `:37` maps `t10.` to iSCSI.
NAA names are **transport-agnostic** — FC, iSCSI, SAS and local SCSI disks all present them —
so neither rule carries information. The rule is stated deliberately in the doc comment, and
`transport_test.go:28` **asserts it as the expected value** (`{"FC canonical NAA disk",
"naa.6000c299…", TransportFC}`).

**It dominates the output** because `hostStorageIndex` recovers the owning adapter by
regex-ing `vmhba\d+` out of the LUN's identifiers (`datastores.go:164-195`). Real SAN LUNs
present as `naa.…` with device path `/vmfs/devices/disks/naa.…` — no `vmhbaN` anywhere — so
they are never indexed, and `transportFor` falls through to `ClassifyTransport(diskName)`,
straight into the `naa.` rule.

**Verified by differential probe** against the production path:

| owning adapter | LUN | correct | reported |
|---|---|---|---|
| `HostFibreChannelHba` | `naa.6000c299…` | FC | `FC` |
| `HostInternetScsiHba` | `naa.6000c299…` | iSCSI | **`FC`** |
| `HostParallelScsiHba` | `naa.6000c299…` | unknown | **`FC`** |
| `HostBlockHba` | `naa.6000c299…` | unknown | **`FC`** |

Negative control: the same iSCSI HBA with a LUN named `mpx.vmhba33:C0:T0:L0` — where the regex
*can* recover the adapter — reports **`iSCSI`**. The index path is real and reachable; the
defect is the `naa.` fallback that real SAN naming always reaches.

**Charged accuracy, not integrity**, following the `deepseek-v4-flash-0731` precedent: the rule
is openly stated, consistently held, and not concealed. **Overrulable — this call is the
difference between 18 and FAIL.**

### H-1 (High) — two N+1 access patterns

Confirmed by `spine gate go n-plus-one`: `hostStorageIndexFor` per host inside the datastore
loop (`datastores.go:119`) and `FetchDVPorts` per port group (`switches.go:258`). A host-ref
cache bounds the first at O(hosts) rather than O(datastores × hosts) — better than deepseek's
equivalent — but it remains per-object retrieval in a loop.

### Other confirmed findings

- **Medium** — cancellation plumbed but **never probed** by any test.
- **Medium** — the sim datastore assertion accepts membership including `unknown`, so it
  passes for any implementation; the structural reason C-1 shipped green.
- **Medium** — `cmd/` at 14.8% coverage.
- **Low** ×2 — `gosec` G104 (`config.go:44`, discarded `BindEnv` error); inconsistent output
  units (RAM `0.03125 GB` vs STORAGE `0.0 GiB`).

### Not run

**The behavioural mutation battery was not run.** Disclosed rather than omitted; `battery_*`
front matter left empty, consistent with every other run in this comparison set. No git
history exists in the run directory, so test-churn forensics could not be performed.

## Score

**18 / 30 — PASS WITH CONCERNS.** 1 Critical (accuracy), 2 High, 3 Medium, 2 Low.

| Dimension | Score | Why |
|---|--:|---|
| Accuracy | 2 | 7 of 8 criteria met and verified. Criterion 4 **unmet** — transport fabricated, not derived. |
| Integrity | 3 | No cheat, skip, stub or forged log. Deductions: the unit test **certifies** `naa.`→FC as expected, and the sim test accepts `unknown` alongside the fabricated `FC`. |
| Security | 4 | `insecure` default false; password never logged; timeout plumbed; view destroyed. staticcheck + govulncheck clean; gosec 1 × G104 Low. |
| Performance | 2 | Two N+1 sites, one the nested per-datastore-per-host pattern the rubric names as its scale-killer. |
| Concurrency | 4 | `-race` clean; zero goroutines. Deduction: cancellation plumbed but never probed. |
| Quality | 3 | gofmt/vet/staticcheck clean, genuine separation, `%w` wrapping. Costs: the identifier-semantics error at the heart of the primary feature, `cmd/` 14.8%, inconsistent units. |

## Compare

**This run's job was to measure opencode run-to-run variance at matched effort, and it does —
decisively, in two independent dimensions.**

### Token variance is small; the omp gap is not variance

| run | harness | effort | reasoning | output | msgs |
|---|---|---|---|---|---|
| `qwen-3.8-flash` | opencode | `xhigh` | 88,467 | 75,374 | 263 |
| `opencode-qwen3.8-flash` | opencode | `xhigh` | **72,292** | **49,983** | **179** |
| `omp-qwen-3.8-flash` | omp | `null` | 244,164 | 332,988 | 239 |

Two opencode runs at matched effort differ by **1.22×** in reasoning tokens. The omp run sits
**2.8–3.4× above both**. The omp gap is roughly an order of magnitude larger than opencode's
own run-to-run spread, so **it is not plausibly variance** — something structural differs
between the harnesses. This retires hypothesis H2 as pre-registered on
[`omp-qwen-3.8-flash-run2`](omp-qwen-3.8-flash-run2.md) *for the opencode side*; the omp side
still needs its own repeat, and the wire-level effort question there remains open.

### Score variance at matched effort is 6 points — larger than every harness delta measured

| | `qwen-3.8-flash` | `opencode-qwen3.8-flash` |
|---|--:|--:|
| score | **24 / 30** | **18 / 30** |

**Same model, same harness, same effort, same task, one day apart — 6 points apart.** That is
larger than every cross-model and cross-harness difference this eval has recorded in the 18–25
band, and it is the single most important number produced in this whole comparison sequence.

The two runs fail in disjoint places and neither dominates: the first shipped a real
key/device traversal bug but degraded to a legal `unknown` and had no N+1; this one fabricates
`FC` for everything and ships two N+1 sites, while being *cleaner* on dependencies and Viper
wiring. **Which defect a run happens to ship is close to a coin flip, and the rubric converts
that coin flip into a 6-point spread.**

**Consequences for the field, stated plainly:**

1. **Every single-run score in the 18–25 band in this eval is inside the noise floor.** The
   six-run session already suspected this ("scores in the 23–25 band are decided by which
   wiring bug a run happens to ship"); this is the first *direct measurement* of it, on a
   controlled repeat, and the spread is worse than suspected — 6 points, not 2.
2. **The omp↔opencode score comparison (22 vs 24) is dead.** A 2-point difference cannot be
   read against a 6-point same-condition spread. Only the token-spend difference survives,
   because its effect size exceeds the measured variance.
3. **Criterion 4 is the whole story.** Four of the last five runs — `qwen-3.8-flash`,
   `omp-qwen-3.8-flash`, `deepseek-v4-flash-0731` and this one — differ *almost exclusively*
   in how they fail criterion 4, and it alone moves 2–3 points of Accuracy plus 1 of Integrity.
   A rubric where one criterion carries the variance is measuring that criterion, not the model.
4. **ladderbench v2 needs n≥3 per cell.** Single runs cannot support the ordering claims this
   corpus has been making. This is the strongest v2 input the pilot sequence produced, and it
   came from a repeat, not from a new model.
