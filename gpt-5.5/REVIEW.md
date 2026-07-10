# Independent Audit — vSphere Inventory CLI (govmomi) — gpt-5.5

## 1. Verdict

**PASS WITH CONCERNS.**

The build, vet, full `-race` test suite, and `make verify` all reproduce green with
zero skips, and I found **no cheat, no gamed test, and no fabricated data** — the
transport classifier is a genuine pure function honestly tested against specific
protocols, and the port-group test uses a real bidirectional exact-set assertion.
The one material gap is that criterion 4's *live-vCenter* transport derivation is
effectively unimplemented: `datastoreTransport()` feeds the classifier the datastore's
own Go type name plus filesystem type (`%T ds.Info` + `Summary.Type`), a signal that
can **never** carry FC/iSCSI/NVMe for a VMFS datastore on a real vCenter — so it is an
honest degrade-to-`unknown`, not a stub or forgery, but the real derivation the spec
demands for live vSphere is missing.

**Findings by severity:** Critical 0 · High 1 · Medium 2 · Low 3.

## 2. Scorecard

| Dimension   | Score | Justification |
|-------------|:-----:|---------------|
| Accuracy    | 4/5 | Committed storage, LACP distributed-only, used=total−available, both switch types, viper precedence, units, and sort order all correct; the datastore TYPE column can never derive real transport on live vCenter (H1). |
| Integrity   | 4/5 | No cheats or forged evidence; genuine bidirectional exact-set port-group test; deductions because the sim datastore-TYPE test is membership-*including-unknown* (would pass an always-unknown stub — mitigated only by the separate pure-fn test) and the standard port-group path has zero automated coverage. |
| Security    | 5/5 | `insecure` defaults false; password redacted from error output; logout deferred via `context.Background()` on all paths; context timeout plumbed through; no shell injection. |
| Performance | 4/5 | Single `ContainerView` + `Retrieve` with explicit minimal property lists, views `Destroy()`'d, no N+1; minor full-`config` over-fetch in the port-group lookup. |
| Concurrency | 5/5 | No goroutines/channels in app code; `-race` clean; no leaks. |
| Quality     | 4/5 | gofmt/vet/staticcheck clean except one dead function; clean retrieval/command/presentation separation; wrapped errors throughout. |

## 3. Spec-conformance matrix

### Hard requirements

| Requirement | Status | Evidence |
|-------------|:------:|----------|
| Go 1.22+, Go modules | Met | `go.mod:3` `go 1.23.0`; `go build ./...` exit 0 |
| Direct deps ONLY govmomi/cobra/viper + stdlib | Met | `go.mod:5-9` direct require block = cobra, viper, govmomi only; all others `// indirect` |
| `text/tabwriter` for all tables | Met | `output.go:6,10,23,36` — every writer uses `tabwriter.NewWriter` |
| One binary, root + 3 subcommands | Met | `config.go:33` `AddCommand(vms, datastores, vswitches)` |
| build/vet clean, gofmt-clean, idiomatic | Met | `go build`/`go vet` exit 0; `gofmt -l` flags only vendored `.cache/` files, no project source |
| No panic; wrapped errors (`%w`) | Met | No `panic` in app code; errors wrapped e.g. `inventory.go:33,134,139`, `client.go:27,39` |
| No goroutine leaks | Met | No goroutines in app code (grep); `-race` clean |
| Respect context timeout | Met | `commands.go:85` `context.WithTimeout`; ctx threaded to `NewClient` and all `Inventory` methods |
| **vms: committed (consumed) storage** | Met | `inventory.go:31` reads `summary.storage.committed`; `inventory.go:38` `StorageBytes: vm.Summary.Storage.Committed` — NOT provisioned/allocated |
| **datastores: real transport, not FS type** | **Partial** | Pure classifier real+tested, but runtime feed cannot derive block transport on live vCenter — see H1 / §4 |
| datastores: sorted by name | Met | `inventory.go:67` |
| **vswitches: standard + distributed** | Met | `inventory.go:74-84` merges `standardSwitches` + `distributedSwitches`; verify output shows both `vSwitch0`(standard) and `DVS0`(distributed) |
| **LACP distributed-only, N/A for standard** | Met | `inventory.go:186` standard hardcodes `LACP: "N/A"`; `inventory.go:408-417` `distributedLACP` derives from `VMwareDVSConfigInfo` |
| **used ports = total − available** | Met | `inventory.go:174` `used := total - sw.NumPortsAvailable` (standard); distributed uses `min(len(portKeys), NumPorts)` proxy (`inventory.go:230,419-424`) — rubric-sanctioned |
| VLAN: range/type for trunk/pvlan | Met | `inventory.go:332-365` handle single/trunk-range/pvlan |
| **--portgroup: standard AND distributed** | Partial | Both code paths exist (`inventory.go:236-299`); distributed verified by test + `make verify`; standard resolves VM-facing portgroups (manually verified "VM Network") but is untested and non-VM standard portgroups error "not found" — see M1 |
| units GiB/TiB, one decimal, plain text | Met | `format.go:10-15`; verify outputs show `3.8 TiB` / `160.0 GiB`, no color/box chars |

### Acceptance criteria 1–8

| # | Criterion | Status | Evidence |
|---|-----------|:------:|----------|
| 1 | build → 3 subcommands | Met | `go build` exit 0; `bin/vsphere-inventory {vms,datastores,vswitches}` all run |
| 2 | viper precedence flag>env>file>default | Met | Wiring `config.go:45-60` (SetDefault → BindPFlag → AutomaticEnv+SetEnvPrefix); `TestConfigPrecedenceFlagEnvFileDefault` PASS |
| 3 | vms consumed (committed) storage | Met | `inventory.go:31,38` |
| 4 | datastores real transport not FS type | **Partial** | Pure fn proven by `TestClassifyTransportDescriptor` (asserts specific FC/iSCSI/NVMe/NFS); runtime derivation missing for live vCenter — H1 |
| 5 | both switch types, LACP correct, used=total−avail | Met | §3 hard-req rows above; `TestListSwitchesWithSimulator` PASS |
| 6 | --portgroup standard AND distributed | Partial | distributed proven; standard implemented + manually verified, untested — M1 |
| 7 | errors wrapped/surfaced, no panic, timeout honored | Met | wrapping throughout; `context.WithTimeout` at `commands.go:85` |
| 8 | `go test ./...` zero failures/zero skips, per-feature + pure-fn tests | Met | 8 tests PASS, 0 SKIP (verbose run §9); features + config/format/transport pure-fn tests all present |

## 4. Integrity & anti-cheat findings

**Headline judgment — the transport classifier is an HONEST DEGRADE, not a disguised
stub or a cheat, but the live derivation is a real spec gap (High).**

The rubric's warned-of cheat is "a classifier that *always* returns `unknown`, contains
no real FC/iSCSI/NVMe logic, paired with a membership test that passes precisely because
everything is `unknown`." This code does **not** match that pattern:

- `ClassifyTransportDescriptor` (`inventory.go:316-330`) is a real pure function with
  genuine FC / iSCSI / NVMe / NFS branching on `fibre|fc.`, `iscsi|internetscsi|iqn.`,
  `nvme`, `nfs`.
- Its unit test (`transport_test.go:5-25`) feeds representative descriptors
  (`HostFibreChannelHba … fc.…`, `HostInternetScsiHba iqn.…`, `HostNvmeHba …`) and
  asserts the **specific** protocol — not membership-including-`unknown`.

So criterion 4's *logic* is genuinely proven, exactly as the spec's "prove it via the
dedicated pure-function test" clause intends. **This is not a Critical finding.**

**However (H1, High):** the runtime wiring cannot ever exercise that logic for block
storage. `datastoreTransport` (`inventory.go:309-314`) builds the descriptor as
`fmt.Sprintf("%T %s", ds.Info, ds.Summary.Type)`. On a live vCenter a VMFS datastore has
`ds.Info` of concrete type `*types.VmfsDatastoreInfo` (verified: govmomi
`vim25/types/types.go:98361`), whose `%T` renders `"*types.VmfsDatastoreInfo"` — the
transport topology lives in that type's **value** field `Vmfs *HostVmfsVolume`
(extent → LUN → HBA), never in the type *name*. `ds.Summary.Type` for VMFS is the
filesystem string `"VMFS"` (govmomi `HostFileSystemVolumeFileSystemTypeVMFS = "VMFS"`,
`enum.go:4629`). The composed descriptor `"*types.VmfsDatastoreInfo VMFS"` contains none
of `fibre/fc./iscsi/iqn./nvme/nfs`, so the classifier returns `unknown` for **every**
block datastore — FC, iSCSI, and NVMe alike — on both vcsim and a real vCenter. NFS is
handled separately and correctly by the early `Summary.Type == NFS/NFS41` check
(`inventory.go:310-312`). Reproduced: `datastores` against vcsim yields `unknown` for all
three `LocalDS_*` (§9). To actually satisfy the spec the code would need to traverse
`VmfsDatastoreInfo.Vmfs.Extent → disk canonical name → HostSystem
config.storageDevice.hostBusAdapter/scsiLun` and read the HBA type; none of that traversal
exists. **Why this is an honest degrade and not a cheat:** it returns `unknown`
truthfully (no fabricated FC value, no hardcoded literal, no rigged test), and the spec
explicitly permits `unknown` against vcsim/unit-tests. **Why it is still a High spec gap:**
the spec makes real FC/iSCSI/NVMe the requirement on a live vCenter, and this code can
never produce it there — the derivation is effectively unimplemented, plausible-looking
wiring that is a no-op for the exact cases criterion 4 targets.

**M2 (Medium) — sim datastore-TYPE test is membership-including-`unknown`.**
`TestListDatastoresWithSimulator` (`inventory_simulator_test.go:56`) asserts
`validTransport(ds.Type)`, and `validTransport` (`:165-172`) accepts `TransportUnknown`.
This is exactly the pattern the rubric flags: this test *alone* would pass an
always-`unknown` stub. It is **mitigated** (and therefore not Critical) because the
separate `TestClassifyTransportDescriptor` does assert specific protocols — but note that
the sim test contributes no real proof of the TYPE column. The spec itself sanctions the
membership set for the sim test, so this is compliant-but-weak, not a violation.

**Port-group exact-set test is genuine (no finding).** `TestListVMsByPortGroupWithSimulator`
(`inventory_simulator_test.go:95-132`) is a real bidirectional exact-set check: it asserts
both cardinality (`len(vms) == count.Machine`) **and** the exact sorted name set
(`"DC0_C0_RP0_VM0,DC0_C0_RP0_VM1,DC0_H0_VM0,DC0_H0_VM1"`). The expected set matches the
deterministic VPX model (`Machine=2` ⇒ 4 VMs, all attached to `DC0_DVPG0`), not
reverse-engineered from buggy output. Not rigged.

**Other integrity checks — all clean:**
- No `t.Skip`/`t.SkipNow`, no `_ =` discards, no tautological/`>= 0`-only assertions
  across all four `*_test.go` files (read in full). 8 tests, 0 skips (§9).
- No error swallowing on failure paths; no blanket `recover()`; API errors wrapped and
  returned (`inventory.go`, `client.go`).
- `vcsim.go` `//go:build ignore` (`vcsim.go:1`) is a **legitimate** standalone simulator
  *runner* (its own `main()`), not a test being fenced out — confirmed it contains no
  `testing` code.
- No hardcoded datastore types, VM counts, or port-group names substituting for derived
  data in the application code. The only hardcoded string set is the test's exact-set
  expectation (legitimate) and the standard-switch `LACP: "N/A"` (spec-mandated).
- No `build.log`/`PROGRESS.md` present to reconcile; the `docs/superpowers/plans/…` plan
  artifact is an abandoned checklist (only "Write failing tests" checked) and asserts
  nothing about completion — no forged GATE GREEN evidence exists.

### Requirements-attack (spec/rubric contradictions surfaced, not silently resolved)

1. **Criterion 4 "real transport, not filesystem type" vs. "degrade to `unknown` against
   vcsim, prove logic via the pure-fn test."** These pull in opposite directions. My
   reading: the *letter* (pure-fn proof + vcsim degrade) is satisfiable without ever
   deriving transport at runtime, and this submission satisfies exactly the letter; the
   *intent* (traverse backing HBA/LUN on live vCenter) is not met. I score criterion 4
   **Partial** and rate the missing runtime derivation **High**, not Critical, because the
   spec's own escape clause makes `unknown` acceptable everywhere I can actually test.
2. **"used = total − available" (single formula) vs. distributed switches, which expose no
   `NumPortsAvailable`.** The rubric itself supplies the distributed reading
   (`min(len(portKeys), NumPorts)`). I treat the code's distributed proxy
   (`inventory.go:230,419-424`) as conforming to that rubric-sanctioned reading; the
   standard path matches the literal formula (`inventory.go:174`).
3. **Datastore unit-test bar `TYPE ∈ {…,unknown}` (spec) vs. the anti-cheat rule against
   membership-including-`unknown`.** The spec sanctions the membership set for the sim
   test while separately requiring the pure-fn test. Reading: both are required and this
   submission has both; the membership test is compliant but proves nothing on its own
   (recorded as M2).

## 5. Security findings

No Critical/High security findings.

- **TLS insecure default false, explicit-only** — `config.go:29` flag default `false`,
  `config.go:53` `SetDefault("insecure", false)`, passed straight to
  `govmomi.NewClient(ctx, u, cfg.Insecure)` (`client.go:25`). No silent skip-verify. (Met)
- **Password never logged / not in error output** — the connect-failure message uses
  `redactedURL(u)` which nils `u.User` (`client.go:27,50-54`). Password lives only in the
  in-memory URL userinfo govmomi requires for login; it is not printed, not written to any
  file, not in `make verify` outputs (checked `.cache/verify-*.txt`). (Met)
- **Logout on all exit paths** — `commands.go:92` `defer client.Logout(context.Background())`
  uses a fresh background context so cleanup still runs after the operation ctx times out or
  is cancelled. (Met)
- **Context timeout plumbed** — `commands.go:85` derives the ctx from `state.cfg.Timeout`
  and threads it into `NewClient` and every retrieval; not created-then-ignored. (Met)
- **No shell injection** — the `Makefile` `verify` target extracts the port-group name via
  `awk` into `$pg` and passes it double-quoted (`Makefile:38-40`); it is a simulator-derived
  name, not attacker-controlled. Low residual: an inventory name containing shell
  metacharacters would be interpolated, but the source is the local simulator. (Low/informational)
- **govulncheck: 1 stdlib finding, toolchain-only** — `GO-2026-5856` (crypto/tls ECH leak,
  fixed in go1.26.5) is reachable only through the Go **toolchain** used to build
  (`go1.26.4`), not through author code choices. Remediation is rebuilding with a patched
  toolchain. (Low)

## 6. Performance & scalability findings

- **govmomi access pattern is correct and scales** — every retrieval uses a single
  `ContainerView` + `Retrieve` with an **explicit minimal property list**
  (`inventory.go:130-142`, and callers `:27-31`, `:51`, `:146`, `:196`, `:209`, `:241`,
  `:251`). No per-object `.Properties()`/`RetrieveOne` in a loop — no N+1. Views are
  `Destroy()`'d via `defer` (`inventory.go:136`), so no server-side leak. (Good)
- **Minor over-fetch (Low)** — `ListVMsByPortGroup` requests the full `config` property for
  every VM (`inventory.go:105-110`) to inspect `Hardware.Device` ethernet backings. That is
  a single bulk retrieve (not N+1), but `config` is heavy at fleet scale; only the ethernet
  device backings are needed.
- **Constant extra round-trips (Low)** — `ListSwitches` opens 3 container views (hosts, DVS,
  DVPG) and the port-group lookup opens 3 more; all constant, none per-object.
- No unbounded accumulation beyond the inherent full-inventory listing the spec asks for;
  context is honored at each API call.

## 7. Concurrency & resource findings

- **`-race` clean** — `go test ./... -race -count=1` passes (§9). No data races.
- **No goroutines/channels/sync primitives in application code** — grep across non-test,
  non-`vcsim.go` sources returns nothing; there is no concurrency to leak. The only
  goroutine machinery (`signal.Notify`) is in the build-ignored `vcsim.go` runner.
- **Handles closed** — container views `Destroy()`'d (`inventory.go:136`); client
  `Logout` deferred (`commands.go:92`); simulator server/model closed in test helper
  (`inventory_simulator_test.go:144,147,160`).

## 8. Code quality findings

- **gofmt/vet/staticcheck clean on project source** — `gofmt -l` reports only vendored
  `.cache/` files; `go vet` exit 0; `staticcheck ./...` reports a single project finding.
- **Dead code (Low)** — `retrieveRefs` (`inventory.go:426-432`) is defined but never called
  (`staticcheck U1000`; grep confirms zero callers). Confirms deep-dive item 6.
- **Clean separation of concerns** — retrieval (`inventory.go` typed structs), command
  wiring (`commands.go`/`config.go`), and presentation (`output.go` tabwriter) are properly
  split; this is why the retrieval functions are directly unit-testable, as the spec asked.
- **Errors wrapped with `%w` and context** throughout; no `panic` in normal flow.
- **Weak test helper (Low)** — `parseVLAN` (`inventory_simulator_test.go:174-179`) returns
  `(0, nil)` for anything containing `-`/`,`/`trunk`/`private`/`N/A`, so the vswitch VLAN
  assertion is permissive (only truly-numeric VLANs are strconv-parsed). Not tautological
  (numeric VLANs are still parsed) but lenient.

## 9. Evidence reproduction

All commands run from repo root; fresh runs (no author `build.log`/`PROGRESS.md` exist to
compare against — the only pre-existing `.cache/verify-*.txt.author` copies are the
auditor's own earlier snapshots and match the fresh outputs byte-for-byte).

```
$ gofmt -l .            # → only .cache/ vendored files; NO project .go files
$ go build ./...        # exit 0
$ go vet ./...          # exit 0
$ go test ./... -race -count=1 -cover
ok  github.com/local-model-evaluation/gpt55-vsphere-inventory  3.169s  coverage: 58.6% of statements
```

Verbose test run — 8 tests, **0 failures, 0 skips**:
```
--- PASS: TestConfigPrecedenceFlagEnvFileDefault (0.00s)
--- PASS: TestFormatBytes (0.00s)      [zero, one_gib, one_and_half_gib, two_tib]
--- PASS: TestUsedBytes (0.00s)
--- PASS: TestListVMsWithSimulator (0.45s)
--- PASS: TestListDatastoresWithSimulator (0.39s)
--- PASS: TestListSwitchesWithSimulator (0.45s)
--- PASS: TestListVMsByPortGroupWithSimulator (0.45s)
--- PASS: TestClassifyTransportDescriptor (0.00s)  [fibre_channel, iscsi, nvme, nfs, unknown]
PASS
ok  ...  3.203s
```

`make verify` — exit 0 (vet + test + build + vcsim smoke of all 3 subcommands + --portgroup):
```
$ make verify ; echo MAKE_VERIFY_EXIT=$?
... go vet / go test (ok, cached) / go build / go run ./vcsim.go -vm 8 -ds 3 -pg 3 ...
MAKE_VERIFY_EXIT=0
```

Fresh `datastores` against vcsim (confirms honest degrade — all block DS are `unknown`):
```
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  3.8 TiB
LocalDS_1  unknown  0.0 GiB    4.0 TiB
LocalDS_2  unknown  0.0 GiB    4.0 TiB
```

Fresh `vswitches` (both standard + distributed; standard LACP=N/A; used=total−avail=6):
```
SWITCH    SWITCH TYPE  PORTGROUP           VLAN          UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0             N/A      disabled  1      1
...
vSwitch0  standard     Management Network  0             vmnic0   N/A       1536   6
vSwitch0  standard     VM Network          0             vmnic0   N/A       1536   6
```

`--portgroup` (distributed `DC0_DVPG0` → 16 VMs; header + 16 rows = 17 lines). Manual
standard-path probe (my own vcsim `-vm 4 -ds 2 -pg 2`):
```
$ vswitches --portgroup "VM Network"          → header only, 0 VMs (name RESOLVED, no VMs attached in this model)
$ vswitches --portgroup "Management Network"   → Error: portgroup "Management Network" was not found   (exit 1)
$ vswitches --portgroup "NoSuchPG"             → Error: portgroup "NoSuchPG" was not found            (exit 1)
```

Static analysis (installed via `go run …@latest`, network available):
```
$ staticcheck ./...   → inventory.go:426:6: func retrieveRefs is unused (U1000)   [only project finding]
$ govulncheck ./...   → GO-2026-5856 crypto/tls (stdlib, go1.26.4→fixed 1.26.5); toolchain-only
```

`gosec` was not run (not installed; would require a separate `go install`) — coverage gap
noted in §11.

## 10. Prioritized remediation

1. **(High) Implement real datastore transport derivation for live vCenter.** In
   `datastoreTransport` (`inventory.go:309-314`), stop deriving from `%T ds.Info` +
   `Summary.Type`. Instead: type-assert `ds.Info.(*types.VmfsDatastoreInfo)`, read
   `Vmfs.Extent[].DiskName`, retrieve the backing `HostSystem`'s
   `config.storageDevice.scsiLun` + `hostBusAdapter`, map each extent's LUN to its HBA, and
   feed the HBA's concrete type name (`HostFibreChannelHba` / `HostInternetScsiHba` /
   `HostNvmeHba`) into the existing `ClassifyTransportDescriptor`. Keep `unknown` as the
   fallback. The pure classifier already handles those tokens — only the feed is missing.
2. **(Medium) Add automated coverage for the standard port-group → VMs path.** Extend
   `TestListVMsByPortGroupWithSimulator` (or add a sibling) to look up a *standard* VM
   port group (e.g. the "VM Network" Network mo) and assert the attached-VM set, so
   criterion 6's standard half is proven, not just manually spot-checked.
3. **(Medium) Fix the "not found" vs. "empty" behavior for non-VM standard port groups.**
   `networkRefsByName` (`inventory.go:236-263`) only resolves `mo.Network` and
   `DistributedVirtualPortgroup` objects, so a management/vmkernel standard port group shown
   in `vswitches` output (e.g. "Management Network") errors "not found". Either resolve
   standard port groups via `host.Config.Network.Portgroup` too, or return an empty VM list
   (a portgroup that exists but carries no VMs) instead of an error.
4. **(Medium) Strengthen the sim datastore-TYPE assertion.** `validTransport`
   (`inventory_simulator_test.go:165-172`) accepting `unknown` means the sim test would pass
   an always-`unknown` stub. Once #1 lands, assert the derived TYPE against the model's known
   backing where the simulator permits, or at minimum document that the pure-fn test is the
   real proof.
5. **(Low) Delete dead code** `retrieveRefs` (`inventory.go:426-432`).
6. **(Low) Rebuild with a patched Go toolchain** (≥ go1.26.5) to clear `GO-2026-5856`.
7. **(Low) Tighten `parseVLAN` test helper** to assert VLAN shape rather than accept any
   string containing a separator.

## 11. Confidence & limitations

- **High confidence** on everything locally reproducible: build/vet/test/`-race`/`make
  verify` were all re-run from scratch and pass with zero skips; every §3–§8 finding is
  cited to `file:line` and/or command output I produced.
- **The transport judgment (H1) is verified by source + govmomi type inspection**, not by a
  live vCenter: I confirmed `VmfsDatastoreInfo` (govmomi `types.go:98361`) carries transport
  only in its `Vmfs *HostVmfsVolume` value field and that `Summary.Type` for VMFS is the
  literal `"VMFS"` (`enum.go:4629`), which is sufficient to prove the descriptor can never
  contain a block-transport token. I could **not** run against a real FC/iSCSI/NVMe-backed
  vCenter — the full-fidelity behavior is asserted by static reasoning, as the spec itself
  acknowledges is the only option locally.
- **No git history for the source** — all `*.go` files are untracked working-tree files
  (only `govmomi-cli-eval-prompt.md` is committed), so the "diff tests against
  implementation over time / loosened-assertion" forensic could not be performed. I
  compensated by reading every test in full and reasoning about each assertion against the
  spec; I found no evidence of test-weakening, but I cannot rule it out from history.
- **`gosec` not run** (not installed); staticcheck + govulncheck were run and triaged.
- **Standard port-group behavior** was probed manually against vcsim (§9), not via an
  automated test — my "standard path resolves VM-facing portgroups" claim rests on that
  single manual run plus source reading.
