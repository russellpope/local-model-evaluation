# Independent Audit — omp-qwen-3.8-flash-run2 (vSphere Inventory CLI)

Audited 2026-09-02 against `govmomi-cli-audit-prompt.md`. Tree:
`omp-qwen-3.8-flash-run2/`, module `github.com/example/govc-inventory`, govmomi
`v0.0.0-20260902050809-f0d835f384b5` (untagged master pseudo-version), `go 1.25.0`,
Go 1.27.0 darwin/arm64.

## 1. Verdict

**PASS WITH CONCERNS — 1 Critical (accuracy), 1 High, 2 Medium, 2 Low.**

The cleanest tree of the four in this comparison set on every dimension *except* the one that
decides the score: no N+1 anywhere, cancellation genuinely probed, the most thorough
`verify.sh` of any submission. But criterion 4 is fabricated — `naa.` → FC, the same rule
`deepseek-v4-flash-0731` and `opencode-qwen3.8-flash` shipped — and the one genuinely correct
signal in the implementation (HBA type) is **dead code on any real host**.

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 2 | 7 of 8 criteria met. Criterion 4 **unmet** — transport fabricated, and the correct path unreachable. Criterion 8 partial (`pflag` imported directly). |
| Integrity | 3 | No cheat, skip, stub or forged log. Deductions: the unit test **certifies** `naa.`→FC as expected (`transport_test.go:16,17,75`), and the sim test accepts `unknown` in the legal set. |
| Security | 4 | `insecure` defaults false; timeout plumbed; view destroyed. staticcheck + govulncheck clean; gosec 2 × G115 (benign). |
| Performance | 4 | **No N+1** — `spine gate go n-plus-one` reports no findings. One `ContainerView` + `PropertyCollector` with explicit fields, view destroyed via `defer`. |
| Concurrency | 5 | `-race` clean; zero goroutines; cancellation **probed** — `vsphere_sim_test.go:262-269` drives a cancelled context and requires an `errors.Is(err, context.Canceled)` descendant. |
| Quality | 4 | gofmt/vet/staticcheck clean; the most thorough verify script in the set (config-precedence and clean-error checks, not just exit codes). Costs: the shadowed dead path, `pflag`, root package 0%. |

**Total: 22 / 30.**

## 3. Spec-conformance matrix

| Requirement | Status | Evidence |
|---|---|---|
| 1. VM storage = consumed | met | `vsphere.go:48` `Summary.Storage.Committed`, field named `CommittedStorage` |
| 2. Datastore used/available | met | `make verify` output |
| 3. Datastore TYPE = real transport | **unmet** | See C-1, H-1 |
| 4. LACP distributed-only | met | `vsphere.go:483,504` `LACPNA` on standard rows; additionally returns `N/A` when `LacpCapability.LacpSupported` is false/absent (`:587-592`) — more careful than any other run in the set |
| 5. used ports = total − available | met | `vsphere.go:493-494` |
| 6. `--portgroup` both std + dvs | met | `cli.go:88`; resolution via `vm.Network` MoRefs joined against both standard `Network` entities and DVPGs (`:719,743-761`) — a different route from ethernet-card backings, covering both types |
| 7. Viper precedence | met | `config.go:45-56` `SetEnvPrefix` + `AutomaticEnv` + `BindPFlags` + `SetDefault`; `verify.sh` asserts precedence end-to-end |
| 8. Deps | **partial** | `github.com/spf13/pflag` imported directly, outside the allowed set (mitigating: it is cobra's own flag library) |

## 4. Integrity & anti-cheat findings

No `t.Skip`/`SkipNow`/`//go:build ignore`/`recover()`; `spine gate go tskip` → no findings. No
`build.log`/`PROGRESS.md`, so no forgery surface.

### C-1 (Critical, accuracy) — `naa.` → FC, a third independent occurrence

`vsphere.go:343`:

```go
case strings.HasPrefix(c, "naa."), strings.HasPrefix(c, "fc."):
    return TransportFC
```

NAA canonical names are **transport-agnostic** — FC, iSCSI, SAS and local SCSI disks all
present them. The doc comment states the belief explicitly ("FC LUNs canonically appear as
`naa.`/`fc.` ids"), and `transport_test.go:16,17,75` assert it as the expected value.

**Verified by differential probe** against the production `indexHostStorage` →
`deviceTransport` path:

| owning adapter | LUN | correct | reported |
|---|---|---|---|
| `HostFibreChannelHba` | `naa.6000…` | FC | `FC` |
| `HostInternetScsiHba` | `naa.6000…` | iSCSI | **`FC`** |
| `HostParallelScsiHba` | `naa.6000…` | unknown | **`FC`** |
| `HostBlockHba` | `naa.6000…` | unknown | **`FC`** |

Three of four wrong. Every VMFS datastore on a real SAN LUN reports `FC` regardless of fabric.

**Charged accuracy, not integrity**, per the `deepseek-v4-flash-0731` precedent — the rule is
stated openly and held consistently, not concealed. **Overrulable; this call is the difference
between 22 and FAIL.**

### H-1 (High) — the HBA-type path is dead code on any real host

`deviceTransport` (`vsphere.go:285-294`) is ordered: LUN table → NVMe table → `naa.` partition
retry → **adapter token → HBA type** → canonical-name fallback. The adapter step is the only
structurally correct signal in the implementation: it resolves `vmhbaN` out of the device path
and reads the concrete HBA type.

It is unreachable. `indexHostStorage` inserts **every** LUN's canonical name into
`lunTransport` with its classification, including `"unknown"`, and the step-1 guard is
`if cls, ok := hs.lunTransport[disk]; ok && cls != ""` — `"unknown"` is a non-empty string, so
the guard returns it and the adapter step never runs.

**Verified by differential probe** — the same iSCSI HBA and the same `mpx.vmhba33:C0:T0:L0`
LUN:

| host state | result |
|---|---|
| LUN present in `ScsiLun` (a real host) | **`unknown`** |
| LUN absent (adapter path reachable) | **`iSCSI`** |

The flip is the negative control: the adapter logic is correct and works; it is shadowed by an
early return that treats `"unknown"` as a resolved answer. Fixing C-1 alone would not help —
the correct path would still never execute.

## 5. Security findings

- `insecure` defaults **false** (`config.go:55`).
- Timeout plumbed; `ContainerView` destroyed via `defer` (`vsphere.go:794-798`).
- staticcheck clean; govulncheck: no vulnerabilities affecting the code.
- **Low** — gosec G115 ×2 (`vsphere.go:493,494`), int64→int32 on port counts. Triaged **not
  real**; a port count cannot approach int32 max.

## 6. Performance & scalability findings

**No N+1.** `spine gate go n-plus-one` (clients `Properties,RetrieveOne,FetchDVPorts,Retrieve`)
reports **no findings** — the only tree in this comparison set that is clean here. Retrieval is
a single `CreateContainerView` over the required kinds plus one `Retrieve` with an explicit
field list (`vsphere.go:794-807`), view destroyed on all paths.

## 7. Concurrency & resource findings

`go test ./... -race -count=1 -cover` → all packages ok, **no races**. No goroutines created.
Cancellation is **probed**, not assumed: `vsphere_sim_test.go:262-269` drives a cancelled
context into `ListVMs` and requires an `errors.Is(err, context.Canceled)` descendant.

## 8. Code quality findings

- `gofmt -l .` empty; `go vet` clean; `staticcheck` clean.
- Coverage: `format` 100%, `config` 96.6%, `vsphere` 81.8%, `cli` 40.7%, root 0.0%.
- `scripts/verify.sh` is the most thorough in the set: beyond running each subcommand it
  asserts **config/env/flag precedence** and that an unreachable endpoint and an unknown port
  group both produce clean wrapped errors.
- **M-1 (Medium)** — `pflag` imported directly, outside the allowed dependency set.
- **M-2 (Medium)** — the sim datastore assertion (`vsphere_sim_test.go:97-111`) accepts
  membership including `unknown`, so it passes for any implementation. Structural reason C-1
  shipped green.
- **L-1 (Low)** — `go.mod` declares `go 1.25.0` while govmomi is pinned to an **untagged
  master pseudo-version** (`v0.0.0-20260902050809-…`) rather than a release tag; the build is
  not reproducible against a published version.

## 9. Evidence reproduction

No `build.log` or `PROGRESS.md`, so no author claims to reconcile.

```
gofmt -l .                       -> (empty)
go build ./...                   -> exit 0
go vet ./...                     -> exit 0
go test ./... -race -count=1     -> ok (all 4 packages)
staticcheck ./...                -> (empty)
govulncheck ./...                -> 0 vulnerabilities affecting the code
gosec ./...                      -> 2 issues (both G115, triaged not real)
spine gate go tskip              -> no findings
spine gate go n-plus-one         -> no findings
make verify                      -> ALL CHECKS PASSED
```

`make verify` reproduces green end-to-end against a live vcsim. Datastores report `unknown`
there — vcsim's extents carry no `naa.` names, which is exactly why C-1 and H-1 are invisible
to the submitted suite.

## 10. Prioritized remediation

1. **(Critical)** Delete the `naa.`/`fc.` → FC rule (`vsphere.go:343`). Correct
   `transport_test.go:16,17,75` — the expected value for a bare `naa.` name is `unknown`.
2. **(Critical)** Fix the shadowing early return in `deviceTransport`: treat a cached
   `"unknown"` as *unresolved* and fall through to the adapter step, e.g.
   `if cls, ok := …; ok && cls != "" && cls != TransportUnknown`.
3. **(Critical)** Resolve the owning adapter structurally for SAN LUNs — walk
   `ScsiTopology`/`MultipathInfo` and map the `Adapter` **key** back to the HBA via
   `HostHostBusAdapter.Key` — so `naa.`-named LUNs reach the HBA-type logic at all.
4. **(Medium)** Drop the direct `pflag` import; reach flags through cobra.
5. **(Medium)** Assert a *specific* protocol in the sim datastore test, not membership
   including `unknown`.
6. **(Low)** Pin govmomi to a released tag.

## 11. Confidence & limitations

- **Verified by execution:** every finding above. C-1 and H-1 each rest on a differential
  probe with a negative control — the result flips when the shadowing input is removed — not
  on reading alone.
- **Not run: the behavioural mutation battery.** Disclosed rather than omitted; `battery_*`
  front matter left empty, consistent with every run in this comparison set.
- **No git history** in the run directory, so test-churn forensics could not be performed.
- **Audited in-context, without fresh-context subagent dispatch**, per the operator's standing
  instruction — the same method as every other run in this set.
- **Effort provenance is unreliable for this run.** The session records
  `thinkingLevel: xhigh, configured: xhigh`, but the operator identified a configuration defect
  under which no thinking level reaches the API while the system prompt asserts `xhigh`. The
  recorded level is intent, not request content. See the run record.
