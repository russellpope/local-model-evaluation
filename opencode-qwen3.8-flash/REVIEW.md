# Independent Audit — opencode-qwen3.8-flash (vSphere Inventory CLI)

Audited 2026-09-02 against `govmomi-cli-audit-prompt.md`. Tree:
`opencode-qwen3.8-flash/`, module `github.com/local-model-evaluation/vsphere-inventory`,
govmomi v0.56.0, Go 1.27.0 darwin/arm64.

## 1. Verdict

**PASS WITH CONCERNS — 1 Critical (accuracy), 2 High, 3 Medium, 2 Low.**

No cheat, no skip, no forged evidence. But criterion 4 is **fabricated rather than derived**:
the classifier maps any `naa.` canonical name to FC, and because real SAN LUNs are exactly the
ones that present `naa.` names, effectively every VMFS datastore on a real vCenter reports
`FC` regardless of its actual transport. This is the same rule
`deepseek-v4-flash-0731` was charged Critical for, independently reproduced.

Charged **accuracy, not integrity** — the rule is stated openly in the doc comment and held
consistently in the tests — so the auto-FAIL rule does not fire.

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 2 | 7 of 8 criteria met and verified. Criterion 4 **unmet** — transport fabricated, not derived. |
| Integrity | 3 | No cheat, skip, stub or forged log. Deductions: the unit test **certifies** `naa.`→FC as the expected value, and the sim test accepts `unknown` alongside the fabricated `FC`. |
| Security | 4 | `insecure` defaults false; password never logged; timeout plumbed; view destroyed. staticcheck + govulncheck clean; gosec 1 × G104 (Low). |
| Performance | 2 | Two N+1 sites, one the nested per-datastore-per-host pattern the rubric names as its scale-killer. |
| Concurrency | 4 | `-race` clean; zero goroutines created. Deduction: cancellation is plumbed but **never probed** by any test. |
| Quality | 3 | gofmt/vet/staticcheck clean, genuine package separation, `%w` wrapping. Costs: the identifier-semantics error at the heart of the primary feature, `cmd/` at 14.8%, inconsistent RAM/STORAGE units. |

**Total: 18 / 30.**

## 3. Spec-conformance matrix

| Requirement | Status | Evidence |
|---|---|---|
| 1. VM storage = consumed | met | `vms.go:48` `Summary.Storage.Committed` |
| 2. Datastore used/available | met | verified in `make verify` output |
| 3. Datastore TYPE = real transport | **unmet** | See C-1 |
| 4. LACP distributed-only, standard = N/A | met | `switches.go:26` `LACPNABlank`; verify shows `N/A` on both standard rows, `disabled` on distributed |
| 5. used ports = total − available | met | `switches.go` aggregator |
| 6. `--portgroup` both std + dvs | met | `vms.go:80-92` both backing types behind a `BaseVirtualEthernetCard` gate; verify exercises DVPG0 |
| 7. Viper precedence | met | `cmd/root.go:73` binds only when `f.Changed`, via `viper.Set` (override tier) — a different route from `BindPFlag`, same correct ordering; config pkg 100% covered |
| 8. Deps: govmomi + cobra + viper + stdlib | met | only those three appear in any import; `vcsim` is an indirect test-only module |

## 4. Integrity & anti-cheat findings

No `t.Skip`/`SkipNow`/`//go:build ignore`/`recover()`; `spine gate go tskip` → no findings. No
`build.log` or `PROGRESS.md`, so no forgery surface.

### C-1 (Critical, accuracy) — `naa.` → FC is a fabrication

`transport.go:43-44`:

```go
case strings.HasPrefix(d, "naa."):
    return TransportFC
```

NAA (Network Address Authority) canonical names are **transport-agnostic** — FC, iSCSI, SAS
and local SCSI disks all present them. There is no valid inference from `naa.` to FC. The
doc comment states the rule deliberately ("bare `naa.` SCSI IDs -> FC"), and
`transport_test.go:28` **asserts it as the expected value**:

```go
{"FC canonical NAA disk", "naa.6000c299...", TransportFC},
```

`t10.` → iSCSI (`transport.go:37`) is wrong on the same grounds; T10 is a vendor-ID
designator format, not an iSCSI signal.

**Why it dominates the output.** `hostStorageIndex` recovers the owning adapter by regex-ing
`vmhba\d+` out of the LUN's identifiers (`datastores.go:164-195`). Real SAN LUNs present as
`naa.…` with a device path `/vmfs/devices/disks/naa.…` — **no `vmhbaN` anywhere** — so the LUN
is never indexed, and `transportFor` falls through to `ClassifyTransport(diskName)`
(`datastores.go:151`), i.e. straight into the `naa.` rule.

**Verified by differential probe** against the production index/classify path (run in a
scratch copy; the tree was not modified):

| owning adapter | LUN | correct | reported |
|---|---|---|---|
| `HostFibreChannelHba` | `naa.6000c299…` | FC | `FC` |
| `HostInternetScsiHba` | `naa.6000c299…` | iSCSI | **`FC`** |
| `HostParallelScsiHba` | `naa.6000c299…` | unknown | **`FC`** |
| `HostBlockHba` | `naa.6000c299…` | unknown | **`FC`** |

Negative control — the same iSCSI HBA with a LUN named `mpx.vmhba33:C0:T0:L0`, where the regex
*can* recover the adapter, reports **`iSCSI`** correctly. So the index path is real and
reachable; the defect is the `naa.` fallback that real SAN naming always reaches.

**Why this is worse than degrading.** The spec explicitly permits `unknown` and calls that
correct. This manufactures a confident wrong answer: an operator reading `FC` for an iSCSI
datastore is misled in a way `unknown` never would be.

**Why accuracy, not integrity.** The rule is stated openly, the test asserts the same belief,
and the README does not overclaim — a wrong premise consistently held, not a concealed one.
Per the 2026-08-16 precedent the auto-FAIL rule is scoped to Critical **integrity** findings.
**Recorded as overrulable; this single call is the difference between 18 and FAIL.**

## 5. Security findings

- `insecure` defaults **false** (`config.go:47`), set only via explicit flag/env/config.
- Password never logged and never written to a file; `url.UserPassword` used only for the
  login call (`client.go:66`).
- Timeout plumbed into every subcommand; `ContainerView` destroyed via `defer`
  (`inventory.go:23`).
- **Low** — `gosec` G104 at `config.go:44`: `c.v.BindEnv(k)` return discarded.
- **Low (observation)** — the client explicitly supports credentials embedded in the URL
  (`client.go:54-55`). Operator-supplied, not a program leak, but it is a path by which a
  password can reach shell history.
- staticcheck clean; govulncheck clean.

## 6. Performance & scalability findings

### H-1 (High) — two N+1 access patterns

Mechanically confirmed:

```
$ SPINE_GATE_N_PLUS_ONE_CLIENTS=Properties,RetrieveOne,FetchDVPorts,hostStorageIndexFor \
    spine gate --dir . go n-plus-one
error  internal/inventory/datastores.go  119  call in loop: hostStorageIndexFor ... one round trip per iteration
error  internal/inventory/switches.go    258  call in loop: FetchDVPorts ... one round trip per iteration
go@1/n-plus-one: 2 finding(s)
```

- `datastores.go:119` — `hostStorageIndexFor` issues a per-host `Properties()` call inside a
  loop nested under the datastore loop. A `cache` keyed by host ref bounds it at O(hosts)
  rather than O(datastores × hosts), which is better than deepseek's equivalent, but it is
  still per-object retrieval in a loop — the rubric's named scale-killer.
- `switches.go:258` — `FetchDVPorts` per distributed port group.

Bulk retrieval elsewhere is correct: one `ContainerView` + `PropertyCollector` with explicit
property lists, view destroyed.

## 7. Concurrency & resource findings

`go test ./... -race -count=1 -cover` → all packages ok, **no races**. The program creates no
goroutines.

**M-1 (Medium)** — cancellation is plumbed but **never probed**: no test constructs a
cancelled or expired context. Nothing would catch a context that stopped being honoured.

## 8. Code quality findings

- `gofmt -l .` empty; `go vet` clean; `staticcheck` clean.
- Genuine separation: `internal/inventory` / `cmd` / `internal/format`.
- Coverage: `config` 100%, `format` 100%, `inventory` 66.8%, `client` 28.3%, `cmd` 14.8%,
  root 0.0%.
- **M-2 (Medium)** — the sim datastore assertion (`datastores_test.go:29-37`) accepts
  membership in {FC, iSCSI, NVMe, NFS, unknown}, so it passes for any implementation
  including the fabricating one. It is the structural reason C-1 shipped green.
- **M-3 (Medium)** — `cmd/` at 14.8%.
- **L-1 (Low)** — inconsistent units in output: RAM renders as `0.03125 GB` (unreduced
  fraction) while STORAGE renders as `0.0 GiB`.

## 9. Evidence reproduction

No `build.log` or `PROGRESS.md`, so no author claims to reconcile.

```
gofmt -l .                       -> (empty)
go build ./...                   -> exit 0
go vet ./...                     -> exit 0
go test ./... -race -count=1     -> ok (all 5 packages)
staticcheck ./...                -> (empty)
govulncheck ./...                -> No vulnerabilities found
gosec ./...                      -> 1 issue (G104, Low)
spine gate go tskip              -> no findings
spine gate go n-plus-one         -> 2 findings
make verify                      -> verify: OK
```

`make verify` reproduces green end-to-end against a live vcsim across `vms`, `datastores`,
`vswitches` and `--portgroup`. Datastores report `unknown` there, because vcsim's extents
carry no `naa.` names — which is precisely why C-1 is invisible to the submitted suite.

## 10. Prioritized remediation

1. **(Critical)** Delete the `naa.` → FC rule (`transport.go:43-44`) and the `t10.` → iSCSI
   rule (`:37`). Neither identifier carries transport information.
2. **(Critical)** Correct `transport_test.go:28` — the expected value for a bare `naa.` name
   is `unknown`, not `FC`. Add negative cases asserting that an `naa.` name owned by an iSCSI
   HBA classifies as iSCSI, not FC.
3. **(Critical)** Resolve the owning adapter structurally rather than by regex: walk
   `ScsiTopology`/`MultipathInfo` and map the `Adapter` **key** back to the HBA via
   `HostHostBusAdapter.Key`, so `naa.`-named SAN LUNs are indexed at all.
4. **(High)** Batch the per-host storage retrieval into one `PropertyCollector` call over all
   mounting hosts (`datastores.go:119`), and hoist `FetchDVPorts` out of the port-group loop
   (`switches.go:258`).
5. **(Medium)** Assert a *specific* protocol in the sim datastore test, not membership
   including `unknown`.
6. **(Medium)** Add a cancellation test that drives an expired context and requires an error.
7. **(Low)** Handle the `BindEnv` error; normalise RAM/STORAGE units.

## 11. Confidence & limitations

- **Verified by execution:** every finding above. C-1 rests on a differential probe with a
  negative control — the same HBA classifies correctly when the LUN name lets the index path
  fire — not on reading alone.
- **Not run: the behavioural mutation battery.** Disclosed rather than omitted; `battery_*`
  front matter left empty, consistent with the other runs in this comparison set.
- **No git history** in the run directory, so the test-churn forensics the rubric suggests
  could not be performed.
- **Audited in-context, without fresh-context subagent dispatch**, per the operator's standing
  instruction — the same method as every other run in this comparison set.
- **This run was executed concurrently with an omp run** against the same provider account.
  Token counts are per-response and unaffected; wall-clock and TTFT are not comparable.
