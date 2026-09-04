# Independent Audit — omp-qwen-3.8-flash (vSphere Inventory CLI)

Audited 2026-09-01 against `govmomi-cli-audit-prompt.md`. Tree:
`omp-qwen-3.8-flash/`, module `github.com/ldh/vsphere-inventory`, govmomi v0.56.0,
Go 1.27.0 darwin/arm64.

## 1. Verdict

**PASS WITH CONCERNS — 1 Critical (accuracy), 2 High, 3 Medium, 1 Low.**

No cheat, no fabricated evidence, no disabled test: the suite is honest and in several
respects stronger than the comparison run's. But the criterion-4 transport derivation is
**broken in production in a way that emits an illegal value** — on a real ESXi host the
TYPE column renders as the empty string, not `unknown`, and a correct classification is
actively overwritten by a later code path. The Critical is charged to **accuracy, not
integrity**, so the auto-FAIL rule does not fire (precedent: `deepseek-v4-flash-0731`).

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 2 | 7 of 8 criteria met and verified. Criterion 4 unmet: production emits `""` and misclassifies local block adapters as NVMe. |
| Integrity | 4 | No skip/tautology/stub/forgery; exact-count and disjointness assertions. Deduction: the fixture cannot reach the transport path; one silent error-swallow. |
| Security | 4 | `insecure` defaults false; credentials stripped from URL and never logged; timeout plumbed. staticcheck + govulncheck + gosec all run and clean bar 2 benign G115. |
| Performance | 3 | Bulk ContainerView + PropertyCollector with explicit property lists, views destroyed — but a confirmed N+1 at two sites in the DVS path. |
| Concurrency | 5 | `-race` clean, zero goroutines created, context cancellation and deadline both asserted with `errors.Is`. |
| Quality | 4 | gofmt/vet/staticcheck clean, real three-layer separation, `%w` throughout, `cmd/` at 92.4% coverage. Deduction: the key/device confusion and the zero-value guard bug. |

**Total: 22 / 30.**

## 3. Spec-conformance matrix

| Requirement | Status | Evidence |
|---|---|---|
| 1. VM storage = consumed, not provisioned | met | `vms.go:52` reads `Summary.Storage.Committed`; `sim_vms_test.go:75-88` asserts committed > 0 |
| 2. Datastore used/available | met | `datastores.go:86-88`, `format.Used`; test asserts `capacity-available == used` exactly |
| 3. Datastore TYPE = real transport | **unmet** | See C-1 / H-1 below |
| 4. LACP distributed-only, standard = N/A | met | `vswitches.go:125` `LACP: LACPNA` on every standard row; verify output shows `N/A` |
| 5. used ports = total − available | met | `vswitches.go:92-99` |
| 6. `--portgroup` both std + dvs | met | `portgroups.go:86-93` both backing types; `BaseVirtualEthernetCard` gate; `TestVMsOnPortgroupIsolatesGroups` asserts on0==2 / on1==0 |
| 7. Viper precedence flag > env > config > default | met | `config.go:27-30` SetEnvPrefix/AutomaticEnv/SetDefault; `root.go:39-43` BindPFlag for all six keys; config pkg 100% covered |
| 8. Deps: govmomi + cobra + viper + stdlib | met | `go.mod` direct requires are exactly those three; no stray third-party import (`pflag` is indirect only, unlike the comparison run) |

## 4. Integrity & anti-cheat findings

**No cheat found.** `grep` for `t.Skip`/`SkipNow`/`//go:build ignore`/`recover()` returns
nothing; `spine gate go tskip` → *no findings*. No `PROGRESS.md` or `build.log` ships, so
there is no evidence-forgery surface. Assertions are real: exact counts (`want 3`, `want 2`),
a genuine disjointness control, and `errors.Is` on cancellation.

### C-1 (Critical, accuracy) — the transport column emits an empty string on real hosts

`hostDiskProtocols` builds `adapterProto` keyed by HBA **device name**
(`datastores.go:130`), then looks it up by `iface.Adapter` (`:161`) and `path.Adapter`
(`:180`) — both of which carry the HBA **key**. Ground truth is govmomi's own canned ESX
data: `simulator/esx/host_storage_device_info.go:17-18` pairs `Key:
"key-vim.host.ParallelScsiHba-vmhba0"` with `Device: "vmhba0"`, and `:166`/`:220` show both
`ScsiTopology.Adapter` and `MultipathInfo` `Path.Adapter` holding the key.

The mismatch alone would be the same High as the comparison run. What makes it Critical
here is the **zero value**: a Go map miss returns `""`, but the guards test against
`TransportUnknown` (`"unknown"`):

```go
p = adapterProto[iface.Adapter]   // miss -> ""
if p == TransportUnknown { continue }   // "" != "unknown" -> does NOT fire
out[name] = p                            // writes ""
```

So `""` propagates into `diskProto` → `volProto` → `DatastoreInfo.Transport`, and the TYPE
cell renders empty. `""` is **not** one of the five legal values and is not the spec's
graceful degrade.

**Verified by differential probe against the production `Datastores()` path**, not by
reading. Probes were run in a scratch copy; the tree was not modified.

| scenario | `Adapter` = HBA key (real) | `Adapter` = device name |
|---|---|---|
| ScsiTopology + iSCSI target transport | `iSCSI` | `iSCSI` |
| ScsiTopology + generic target transport | **`""`** | `iSCSI` |
| MultipathInfo only | **`""`** | `iSCSI` |

The flip is the negative control: the traversal is real and reachable, and it is the
identifier space that is wrong.

**Worse — section 3 clobbers section 2.** A real host populates *both* ScsiTopology and
MultipathInfo. Section 2 can classify correctly via target transport; section 3 then
re-walks the same LUNs and overwrites with the zero value:

| input | result |
|---|---|
| ScsiTopology only | `iSCSI` |
| ScsiTopology + MultipathInfo (a real host) | **`""`** |

Multipathing is standard on exactly the FC/iSCSI SANs criterion 4 targets, so a correctly
derived protocol is destroyed on the common configuration.

End-to-end against govmomi's canned ESX host data (VMware's own model of a real ESXi host):
`datastore LocalDS_0 TYPE=""`.

**Why the suite misses it:** vcsim's default model populates no `ScsiTopology` and no
`MultipathInfo`, so the defective code path never executes under `make verify` — datastores
correctly report `unknown`. Note the membership assertion at
`sim_datastores_test.go:26-37` *would* catch `""` (it is not in the legal set); the blind
spot is the **fixture**, not the assertion. That is a meaningful difference from the
comparison run, whose assertion could not fail at all.

### H-1 (High) — local block adapters classified as NVMe

`targetTransportKind` maps `*types.HostBlockAdapterTargetTransport` → `"nvme"`
(`transport.go:111-112`). A block adapter target is a local block device, not an NVMe
fabric target. Confirmed against canned ESX data: adapter
`key-vim.host.BlockHba-vmhba1` → kind `"nvme"` → `ClassifyTransport` → **`NVMe`**. A local
disk would be reported as NVMe-attached — a fabricated protocol, in the same family as
`deepseek-v4-flash-0731`'s `naa.`→FC rule, though narrower in blast radius.

To the model's credit, the *opposite* error is explicitly avoided: `transport.go:41-43`
documents that `naa.` alone is ambiguous across FC/iSCSI/SAS and never classifies on it —
precisely the fabrication deepseek shipped.

## 5. Security findings

- `insecure` defaults **false** (`config.go:29`), surfaced only via explicit flag/env.
- Credentials never logged; URL userinfo explicitly stripped (`client.go:80` and
  `ParseURL`); auth errors quote the username only, never the password.
- Context timeout plumbed through all calls; `Logout` deferred on all exit paths
  (`client.go:34-38`).
- `verify.sh` quotes every interpolation; the port-group name is extracted by column
  offsets and passed as a single quoted argv element — no shell injection.
- **gosec: 2 × G115** (`vswitches.go:99,325`) int→int32 conversions on port counts.
  Triaged **not real**: a port count cannot approach `int32` max. Reported as Low.
- staticcheck: clean. govulncheck: no vulnerabilities affecting the code.

## 6. Performance & scalability findings

### H-2 (High) — N+1 in the distributed-switch path

Two client round trips **per DVS port group**:
`retrieveOneAll` (`vswitches.go:215`) and `dvsUsedPorts` → `FetchDVPorts`
(`vswitches.go:231`), both inside the `for _, ref := range pgRefs` loop. Mechanically
confirmed:

```
$ SPINE_GATE_N_PLUS_ONE_CLIENTS=FetchDVPorts,retrieveOneAll,... spine gate --dir . go n-plus-one
error  internal/inventory/vswitches.go  215  call in loop: retrieveOneAll ... one round trip per iteration
error  internal/inventory/vswitches.go  231  call in loop: dvsUsedPorts ... one round trip per iteration
go@1/n-plus-one: 2 finding(s)
```

Fine against 3 simulator port groups; collapses on a vCenter with thousands.
`FetchDVPorts` accepts a list of portgroup keys, so one call per DVS is achievable.

**Everything else is correct**: `retrieveAll` uses a single `ContainerView` +
`PropertyCollector` with explicit minimal property lists, and the view is destroyed via
`defer` (`vms.go:19-23`). No unbounded accumulation.

### M-1 (Medium) — silent error-swallow fabricates a zero

`vswitches.go:231-234`: on `FetchDVPorts` error, `used = 0` with the error discarded. A
transport failure is presented as a real count of zero connected ports. Openly commented as
best-effort, so not charged as a cheat.

## 7. Concurrency & resource findings

`go test ./... -race -count=1 -cover` → **all packages ok, no races**. The program creates
no goroutines. Cancellation is *probed*, not assumed: `TestVMsContextCancelled` and
`TestVMsContextDeadline` drive cancelled/expired contexts and require
`errors.Is(err, context.Canceled)` / `context.DeadlineExceeded`.

## 8. Code quality findings

- `gofmt -l .` empty; `go vet` clean; `staticcheck` clean.
- Genuine three-layer separation: `internal/inventory` (retrieval) → `cmd` (wiring) →
  `internal/format` (presentation). No tangled `RunE` closures.
- Coverage: `cmd` 92.4%, `config` 100%, `format` 100%, `inventory` 80.9%, `client` 18.5%,
  root `main` 0.0%.
- **M-2 (Medium)** — `client` at 18.5%: the connection/auth path is largely untested.
- **L-1 (Low)** — `verify.sh` asserts only `ROWS >= 1` for `--portgroup`; it would pass on a
  single spurious row.

## 9. Evidence reproduction

No `build.log` or `PROGRESS.md` exists, so there are no author claims to reconcile — nothing
was asserted that could be forged.

```
gofmt -l .                       -> (empty)
go build ./...                   -> exit 0
go vet ./...                     -> exit 0
go test ./... -race -count=1     -> ok (all 5 packages)
staticcheck ./...                -> (empty)
govulncheck ./...                -> No vulnerabilities found
gosec ./...                      -> 2 issues (both G115, triaged not real)
spine gate go tskip              -> no findings
make verify                      -> ==> verify: ALL CHECKS PASSED
```

`make verify` reproduces green end-to-end against a live vcsim, exercising `vms`,
`datastores`, `vswitches`, and `--portgroup` on both switch types. Datastores report
`unknown` there — the correct degrade for vcsim, and the reason C-1 is invisible to the
submitted suite.

## 10. Prioritized remediation

1. **(Critical)** Key `adapterProto` by HBA **key** as well as device name, or resolve
   `iface.Adapter`/`path.Adapter` through the HBA `Key`→`Device` map before lookup
   (`datastores.go:130,161,180`).
2. **(Critical)** Fix the zero-value guards: test `if p == "" || p == TransportUnknown`
   at `datastores.go:163` and `:185`, so a map miss can never be written as a protocol.
3. **(Critical)** Make section 3 (multipath) refuse to downgrade an already-resolved
   entry — only write `out[name]` when the existing value is empty or `unknown`.
4. **(High)** Remove the `HostBlockAdapterTargetTransport` → `"nvme"` mapping
   (`transport.go:111-112`); a block adapter target is not NVMe.
5. **(High)** Hoist `FetchDVPorts` out of the per-portgroup loop — one call per DVS with
   all portgroup keys — and batch the portgroup property retrieval (`vswitches.go:215,231`).
6. **(Medium)** Propagate or at least mark the `dvsUsedPorts` failure instead of
   reporting `0` (`vswitches.go:231-234`).
7. **(Medium)** Add a fixture that injects `ScsiTopology` + `MultipathInfo` into vcsim so
   the transport path is exercised at all; assert a *specific* protocol, not membership.
8. **(Low)** Strengthen the `--portgroup` assertion in `verify.sh` to an expected count.

## 11. Confidence & limitations

- **Verified by execution:** every finding above. C-1 and H-1 rest on differential probes
  with negative controls (the result flips when the identifier space is corrected), plus
  govmomi's own canned ESX fixtures — not on reading alone.
- **Not run: the behavioural mutation battery.** Disclosed rather than omitted;
  `battery_*` front-matter left empty. The comparison run (`qwen-3.8-flash`) also has no
  battery data, so omitting it keeps the harness comparison method-matched.
- **No git history** in the run directory, so the test-churn forensics the rubric suggests
  could not be performed; anti-cheat conclusions rest on the final tree state only.
- **Audited in-context, without fresh-context subagent dispatch**, per the operator's
  standing instruction not to use the Agent tool. Same method as the `qwen-3.8-flash`
  comparison run, so the two are method-matched; it is a deviation from the skill's default.
- **Transport fidelity ultimately needs a live vCenter.** The probes establish behaviour
  against VMware's own canned ESX representation, which is the best available proxy.
