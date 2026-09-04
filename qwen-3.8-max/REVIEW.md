# Independent Audit — qwen-3.8-max / vsphere-inventory

**Audited:** 2026-09-01 · Go 1.27.0 darwin/arm64 · govmomi v0.56.0
**Tree:** `qwen-3.8-max/` (source read-only; probes ran on a scratch copy under `scratchpad/mx`)
**Provenance:** opencode provider `alibaba-token-plan`, model id `qwen3.8-max` — hosted
endpoint, not local weights; serving precision undisclosed. 130 messages,
2026-09-01 12:40:52 → 13:26:50.
**Model identity:** the open-weight core is `Qwen/Qwen3.8-2.4T-A95B` (2.4T total / 95B
active MoE, 512 experts, 92 layers, BF16, native 262,144 ctx, custom `qwen3.8-max` license).
Per that card, *"Qwen3.8-Max is the official version based on Qwen3.8-2.4T-A95B with more
features, such as vision input & non-thinking support, 1M context length by default, official
built-in tools"* — and the open weights are text-only with thinking non-disableable. Weight
identity between the hosted endpoint and the open checkpoint is **not asserted**; the
"official built-in tools" difference is material for an agentic build task. This run measured
the API product, not the downloadable checkpoint.
**Scale context:** ~13× the total parameters and ~16× the active parameters of
`qwen3.8-flash` (180B/6B core), for a **1-point** score difference (25 vs 24) at matched
effort — see §11.
**Effort:** `variant = xhigh`, 56,949 reasoning tokens. **Directly comparable on effort to
`qwen3.8-flash`** (also `xhigh`, same provider); **not** comparable to
`deepseek-v4-flash-0731` (`high`).

## 1. Verdict

**PASS WITH CONCERNS** — 0 Critical, 0 High, 4 Medium, 0 Low.

The cleanest submission in this eval to date. No cheat of any kind: no `t.Skip`, no build-tag
fencing, no `recover()`, no tautological assertions, no stubbed or hardcoded values, no forged
evidence. `make verify` reproduces green end-to-end. Critically, this is the **first run in
the series whose transport classifier is both semantically correct and provably reachable from
production** — verified by injection, not inferred. All four concerns are Medium: a dependency
violation, two coverage gaps, and one weak assertion.

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 4 | 7 of 8 criteria met and independently verified, including the two that broke every prior run (transport derivation, both port-group paths). Sole deduction: `pflag` is a direct dependency outside the allowed set (criterion 8 partial). |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value; honest `unknown` degrade where the simulator models nothing. Unit tests assert **specific** protocols with real negative cases. Deduction: the datastore sim test still asserts membership over a set including `unknown` (M-1). |
| Security | 4 | `insecure` defaults false; no credential leakage; timeout plumbed at the root command; logout deliberately runs on a `context.WithoutCancel` with its own 10s budget so cleanup survives a main-context deadline. staticcheck + govulncheck clean. `gosec` not installed — disclosed. |
| Performance | 5 | No N+1 anywhere. One `containerRetrieve` helper (`ContainerView` + explicit property list + `defer v.Destroy`), and host storage devices are fetched **once** into a map rather than per datastore — the exact pattern the two prior runs got wrong. |
| Concurrency | 4 | `-race` clean; zero goroutines, nothing to leak; views destroyed via `defer`. Deduction: cancellation is plumbed but **never probed** by a test (M-4). |
| Quality | 4 | gofmt / vet / staticcheck clean; genuine layering (`internal/transport` pure, `internal/vclient` isolated, `cmd` presentation-only); errors wrapped. Costs: `cmd/` and `internal/vclient` at 0.0% coverage (M-3). |
| **Total** | **25 / 30** | |

## 3. Spec-conformance matrix

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | Build → binary, 3 subcommands | **met** | build/vet/gofmt clean; all three ran live against vcsim (§9) |
| 2 | Viper precedence flag > env > file > default | **met** | `config.go:28-61`; six tests incl. `TestFlagOverridesEnvAndFile`, `TestEnvOverridesFile`, `TestDefaults` |
| 3 | `vms` consumed (committed) storage | **met** | `vms.go:44` `vm.Summary.Storage.Committed` |
| 4 | `datastores` real transport, not filesystem | **met** | `transport.go` type-switches on concrete govmomi types (`*types.HostFibreChannelTargetTransport`, `*types.HostInternetScsiTargetTransport`) — **no string heuristics**. Device→LUN→`ScsiTopology`→`target.Transport`; NVMe via namespace topology; NFS from filesystem type only (correct — NFS *is* the transport). **Reachability proven by injection (§4)** |
| 5 | Both switch types; LACP vDS-only; used = total − available | **met** | `switches.go:83` standard ⇒ `N/A`; `:193` DVS LACP from `LacpGroupConfig`; `:116-117` `NumPorts - NumPortsAvailable` with a bounds guard |
| 6 | `--portgroup` standard *and* distributed | **met** | Three-way resolution: `Network`, `DistributedVirtualPortgroup`, and host `Config.Network.Portgroup` (`portgroup.go:44-80`). **Both** tested with exact name sets (`TestVMsOnDistributedPortgroup` via `reflect.DeepEqual`, `TestVMsOnStandardPortgroup`) |
| 7 | Errors wrapped, no panics, timeout honoured | **met** | `%w` throughout; `context.WithTimeout` at `root.go:49` covering all subcommands. Not probed by a test (M-4) |
| 8 | Deps: govmomi, cobra, viper, stdlib only | **partial** | `github.com/spf13/pflag` directly imported (`config.go:9`) and a direct `require`. Mitigating: it is cobra's own flag library |

## 4. Integrity & anti-cheat findings

**No cheat found.** Greps for `t.Skip`, `SkipNow`, `//go:build ignore`, `recover()` return
nothing. No error path returns `nil` to force green. No `PROGRESS.md` or `build.log` exists,
so no claim could be forged.

**The transport classifier is correct by construction.** Unlike both prior runs, it performs
no string matching on device identifiers. `FromTargetTransport` and `FromHBA` type-switch on
concrete govmomi types, and `scsiTransportForDevice` resolves extent → `ScsiLun` (by
`DeviceName` or `CanonicalName`) → `ScsiTopology` → `target.Transport`. Reading the
**target transport** is the semantically right field and sidesteps the adapter key/device
confusion that made `qwen3.8-flash`'s traversal inert.

The unit tests assert **specific** protocols with genuine negative controls —
`HostParallelScsiTargetTransport` ⇒ `unknown`, `HostBlockAdapterTargetTransport` ⇒ `unknown`,
`nil` ⇒ `unknown` — and a composite `ClassifyDatastore` table covers NFS, an FC extent, an
iSCSI extent and an NVMe namespace with exact expected values.

**Reachability verified, not assumed.** Injecting an iSCSI HBA, a matching `ScsiLun`, a
`ScsiTopology` target carrying `HostInternetScsiTargetTransport`, and a VMFS extent into vcsim,
then calling the production `ListDatastores`:

```
datastore "LocalDS_0" -> "iSCSI"     PROBE: traversal REAL and reachable
```

This is the probe that failed on `qwen3.8-flash` (returned `unknown`) and produced a
fabricated `FC` on `deepseek-v4-flash-0731`.

**M-1 (Medium) — the datastore sim test still cannot fail on transport.**
`datastores_test.go:19,40` asserts `ds.Transport` ∈ {FC, iSCSI, NVMe, NFS, **unknown**}, which
passes for any implementation. Charged Medium rather than High (as on the two prior runs)
because the composite `ClassifyDatastore` table test does prove specific-protocol behaviour on
realistic `HostStorageDeviceInfo` structures; only the ~12 lines of `classifyDatastoreTransport`
wiring are left unproven by the suite — and the auditor probe confirms that wiring is correct
today. The gap is regression risk, not an unproven feature.

## 5. Security findings

- `insecure` defaults to `false` (`config.go:28`), reaching `govmomi.NewClient` only via
  resolved config (`vclient/client.go:57`); the untrusted-certificate branch is gated on
  `!cfg.Insecure` and produces an actionable message.
- No credential logging, no credentials in URLs or files.
- Timeout plumbed once at the root command, covering every subcommand. Logout runs under
  `context.WithoutCancel(...)` with a separate 10s budget — cleanup still completes when the
  operation context has already expired. A thoughtful detail no other submission here got.
- `staticcheck ./...` clean; `govulncheck ./...` → 0 vulnerabilities affecting called code.
- **Coverage gap:** `gosec` not installed, not run.

## 6. Performance & scalability findings

**No N+1 anywhere — the strongest performance result in this eval.** All retrieval funnels
through one helper (`view.go:11-19`): `CreateContainerView` → `defer v.Destroy(ctx)` →
`v.Retrieve` with an explicit property list. Host storage devices are fetched **once** for all
hosts into a `map[ManagedObjectReference]*HostStorageDeviceInfo`
(`datastores.go:50-63`) and looked up per datastore, rather than a `RetrieveOne` per mounting
host per datastore. Property lists are minimal and targeted (`config.storageDevice`,
`config.name`, `name`, `summary`, `info`, `host`).

## 7. Concurrency & resource findings

`go test -race -count=1 ./...` — clean, all packages. Zero goroutines created; no channels, no
background work, nothing to leak. Every container view is destroyed via `defer`.

**M-4 (Medium)** — no test drives an expired or cancelled context through a list call. The
timeout is plumbed (verified by reading `root.go:49`) but its behaviour is unobserved.

## 8. Code quality findings

- gofmt / vet / staticcheck all clean.
- Layering is the cleanest in the series: `internal/transport` is pure (no govmomi retrieval),
  `internal/vclient` isolates connection handling, `internal/inventory` returns typed structs,
  `cmd` does only wiring and presentation.
- UPLINKS renders **friendly NIC names** (`vmnic0`), preferring
  `Spec.Policy.NicTeaming.NicOrder.ActiveNic` and falling back to `Pnic` — verified in live
  output. `deepseek-v4-flash-0731` leaked raw managed-object keys here.
- **M-2 (Medium)** — direct `pflag` dependency (criterion 8).
- **M-3 (Medium)** — `cmd/` and `internal/vclient` at 0.0% coverage.

## 9. Evidence reproduction

No author `build.log` or `PROGRESS.md` exists; nothing to reconcile. Produced fresh:

```
go build ./...     exit 0
go vet ./...       exit 0
gofmt -l .         (empty)
staticcheck ./...  (empty)
govulncheck ./...  0 vulnerabilities affecting called code
go test -race -count=1 ./...
    ?   vsphere-inventory                  [no test files]
    ?   vsphere-inventory/cmd              [no test files]
    ok  vsphere-inventory/internal/config     1.331s
    ok  vsphere-inventory/internal/format     1.543s
    ok  vsphere-inventory/internal/inventory  4.605s
    ok  vsphere-inventory/internal/transport  1.902s
    ?   vsphere-inventory/internal/vclient [no test files]
make verify        >> verify OK
```

Live run against vcsim (`-vm 4 -ds 2 -pg 2`):

```
SWITCH    SWITCH TYPE  PORTGROUP           VLAN    UPLINKS  LACP      PORTS  USED
vSwitch0  standard     VM Network          0       vmnic0   N/A       1536   6     <- friendly NIC name
DVS0      distributed  DC0_DVPG0           0       N/A      disabled  1      1

NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  80.0 GiB  3.9 TiB    <- honest degrade; vcsim populates no extents

--portgroup "VM Network" -> 0 rows, exit 0   (correct: all sim NICs are on DVPG0)
```

Auditor probe (scratch copy, not part of the submission):

```
inject iSCSI HBA + ScsiLun + ScsiTopology(target.Transport = HostInternetScsiTargetTransport)
+ VMFS extent, then call production ListDatastores:
    LocalDS_0 -> "iSCSI"        PASS -- traversal real and reachable
```

## 10. Prioritized remediation

1. **(Medium, M-1)** Replace the membership assertion at `datastores_test.go:19,40` with an
   injection test asserting the specific protocol through `ListDatastores`, so the wiring
   between `classifyDatastoreTransport` and `ClassifyDatastore` is regression-protected.
2. **(Medium, M-2)** Drop the direct `pflag` import at `config.go:9`; take the `*pflag.FlagSet`
   Cobra already provides.
3. **(Medium, M-3)** Add coverage for `cmd/` (flag→config precedence at the Cobra layer, one
   table-rendering golden test) and `internal/vclient`.
4. **(Medium, M-4)** Add a cancellation test: drive an expired context through a list call and
   assert a wrapped `context deadline exceeded`.

## 11. Confidence & limitations

- **Real-hardware fidelity untested.** Transport correctness rests on govmomi type semantics
  plus simulator injection; a live vCenter would settle it definitively.
- **`gosec` not run** (could not install). Static security coverage is staticcheck +
  govulncheck only.
- **No git history** in the run directory, so test-churn forensics could not be performed;
  anti-cheat conclusions rest on the final tree state.
- **Effort is controlled against `qwen3.8-flash` but not against the rest of the field.** Local
  runs in this eval carry no effort variant at all, so cross-family comparisons remain
  confounded on effort, precision, and host.
- **The 25-vs-24 result may say more about the rubric than the models.** `qwen3.8-max` carries
  ~13× the total and ~16× the active parameters of `qwen3.8-flash`, ran at matched `xhigh`
  effort on the same provider one day apart, and scored **one point higher**. Its four residual
  findings are all mundane (a dependency violation, two coverage gaps, one weak assertion) —
  none is a capability failure. The most economical reading is that this task saturates near
  the top of the rubric, so the instrument cannot resolve differences between frontier-class
  models. That is a benchmark finding, not a model finding, and it argues for harder
  discriminators in v2 rather than for a Max-over-Flash capability claim.
