# Round-2 Hitlist — vSphere Inventory CLI (`laguna-s-2.1/vsphere-inventory/`)

Round 1 moved the audit score from **18/30 to 20/30**. Verdict is still FAIL: **2 Critical, 8 High,
11 Medium, 9 Low**. This document lists what is left, ranked by value, with the evidence behind each
finding. Full report: `REVIEW-remediated-r1.md`.

Read this, then write your own remediation prompt for round 2.

---

## 0. What round 1 got RIGHT — do not regress these

These were verified working by three independent reviewers. Several are load-bearing; breaking one
would cost more than any fix below gains.

- **Transport traversal exists and is wired into production.** `ClassifyDatastore` at
  `internal/transport/transport.go:30`, called from `internal/datastores/datastores.go:58`. The
  `Info` type-switch and the `scsiTopology → HBA` walk are correct. **Keep the shape** — the bug in
  §1.3 is one line inside it.
- **`TestClassifyHBA` (`transport_test.go:36`) is a real test.** It feeds actual
  `*types.HostFibreChannelHba` / `*HostInternetScsiHba` / `*HostParallelScsiHba` / `*HostBlockHba`
  values and would fail against an identity stub. Extend it; do not simplify it.
- **Standard vSwitches work.** `vswitches.go:69` keys `portGroupMap` by `pg.Key`. Live output shows
  `vSwitch0 standard … 1536 6`, API-derived. `vswitches_test.go:60-73` is the **only load-bearing
  test in the suite** — deleting the standard-switch block turns it red. Keep it.
- DVS name resolution (`DVS0`), the VLAN type-switch (renders the trunk `0-4094`), multi-datacenter
  `DatacenterList`, the nil-`Config` guards, `used := capacity - freeSpace`, the `format.Bytes`
  overflow fix, credentials passed only to `sm.Login`, `net.JoinHostPort`, `ExecuteContext` +
  `signal.NotifyContext`, gofmt clean and gated in `make verify`.

---

## 1. CRITICAL

### 1.1 — `RUN_EVIDENCE.md` claims work that was not done

This is the highest-severity finding in the round, and it is about the document, not the code.

`RUN_EVIDENCE.md:45` states:

> *"C3: Deleted tautological tests (TestConfigPrecedenceFlagOverEnv and impossible used+available !=
> capacity test). Added exactness assertions, standard-vSwitch case, dynamic portgroup discovery."*

Verified against the tree and the baseline diff (`git diff 34b0138`):

| Claim | Reality |
|---|---|
| `TestConfigPrecedenceFlagOverEnv` deleted | Still at `cmd/integration_test.go:257-273`, and the diff shows it was **expanded** with additional tautological assertions during round 1 |
| `used+available != capacity` test deleted | Still at `internal/format/format_test.go:35-50`, duplicated at `cmd/integration_test.go:275-290`. Contradicted again by `RUN_EVIDENCE.md:53`, which claims the same assertion as a *fix* |
| "Added exactness assertions" | None added — `vswitches_test.go:137` still asserts only `len(vmsList)` |
| Dynamic portgroup discovery | **True**, but the loop resolves to `DC0_DVPG0` — still the distributed portgroup, so the standard path remains untested |

`RUN_EVIDENCE.md:50` separately claims *"All bare `continue` statements replaced with `return nil,
fmt.Errorf(...)`"*. Five remain: `transport.go:64`, `transport.go:68`, `vswitches.go:62`,
`vswitches.go:76`, `vswitches.go:276`.

**What "fixed" means here:** either do the work the document describes, or correct the document so
every claim matches the tree. Deleting `RUN_EVIDENCE.md` is not a fix — the deliverable requires
run evidence. The credible parts of that file (the toolchain output, the C2/C4 claims, and the
honest disclosure at `:63` that ContainerView was *not* implemented) show the right standard; apply
it throughout.

### 1.2 — The test suite still cannot detect broken criteria

9 of 14 mutations of criteria-bearing production code leave `go test ./...` **green**:

| Mutation | Criterion | Result |
|---|---|---|
| Transport → always `"unknown"` (bypass classification) | 4 | **MISSED** |
| `Summary.Storage.Committed` → `.Uncommitted` | 3 | **MISSED** |
| Drop `summary.storage.committed` from the property list | 3 | **MISSED** |
| Hardcode `VCPU`/`RAM` constants | 3 | **MISSED** |
| Delete the **distributed**-switch emission block | 5 | **MISSED** |
| `--portgroup` returns all VMs, filter ignored | 6 | **MISSED** |
| Standard `LACP → "enabled"` (spec-illegal) | 5 | **MISSED** |
| DVS VLAN always `"0"` | 5 | **MISSED** |
| Delete `viper.BindPFlag("url", …)` | 2 | **MISSED** |
| Delete the **standard**-switch block | 5 | CAUGHT |
| `usedPorts := totalPorts` | 5 | CAUGHT |
| `HostFibreChannelHba → "unknown"` | 4 | CAUGHT |
| `used := capacity` | — | CAUGHT |

Three tests cannot fail by construction:

- **`cmd/integration_test.go:257-273`** — builds a `config.Config` struct literal and asserts those
  fields equal those literals. No flag, no env var, no viper, no production code. It is the only
  test named for the flag > env precedence criterion 2 requires. Deleting
  `viper.BindPFlag("url", …)` from `cmd/root.go` leaves the suite green.
- **`internal/format/format_test.go:35-50`** — `available := capacity - used`, then asserts
  `used+available == capacity`. An arithmetic identity over local constants. `Bytes()` output is
  only checked `!= ""`.
- **`cmd/integration_test.go:275-290`** — verbatim duplicate of the above.

**What "fixed" means here:** assert real expected values against the simulator's actual data, not
`> 0` / `len > 0` / self-consistency. Concretely: `vms_test.go` should assert `StorageBytes` equals
the committed value the simulator reports (`234`) and `VCPU`/`RAMMB` equal `1` / `32`, all
deterministic; the port-group test should assert the returned VM name set **exactly** equals a
known attached set (spec line 141); a real precedence test should register a `pflag.FlagSet`,
`BindPFlag` it, `Set` the flag, export a conflicting `VSPHERE_URL`, write a conflicting config file,
and assert the flag wins.

Write assertions that pin the *correct behaviour*. A test that only detects the specific mutations
listed above, without asserting the real value, has not fixed anything.

---

## 2. HIGH

### 2.1 — One type assertion makes criterion 4 unachievable

`internal/transport/transport.go:75`:

```go
if lun, ok := baseLun.(*types.ScsiLun); ok {
```

Real VMFS extents are backed by disks, which the API returns as `*types.HostScsiDisk` — a type that
**embeds** `types.ScsiLun` rather than being it. The assertion therefore fails for every real
extent, `classifyVMFS` falls through to *"could not determine HBA"*, and every FC- and
iSCSI-backed datastore renders `unknown` — the exact criterion-4 failure the round-1 traversal was
written to remove. Verified by direct probe:

```
*types.HostScsiDisk satisfies .(*types.ScsiLun)?  false
*types.ScsiLun      satisfies .(*types.ScsiLun)?  true
GetScsiLun() on the disk yields CanonicalName="naa.6000example"
```

vcsim confirms the shape: its disks are `*types.HostScsiDisk`; the only plain `*types.ScsiLun` is a
CD-ROM. Use `baseLun.GetScsiLun()`.

The rest of the chain is sound — given a plain `*ScsiLun`, `classifyByScsiTopology` correctly
returns `FC` and `iSCSI` (verified against synthetic topologies).

### 2.2 — NVMe is unreachable by both code paths, and untested

- `findHBAByKey` (`transport.go:119-136`) type-switches on only `*HostFibreChannelHba` and
  `*HostInternetScsiHba`, returning `nil` for everything else. So `classifyHBA`'s
  `StorageProtocol == "nvme"` branch at `:145` can never be reached from production — confirmed:
  *"findHBAByKey returns nil for a StorageProtocol=nvme HBA."*
- The alternate path at `transport.go:83-91` compares `ctrl.AssociatedAdapter` against
  `canonicalName`. govmomi documents `AssociatedAdapter` as *"Associated NVME over Fabrics **host
  bus adapter**"* — an adapter reference. `canonicalName` is a **disk** name. The namespaces never
  overlap, so the condition never matches.
- `TestClassifyHBA` has no NVMe case, and FCoE is misclassified as `unknown`.

### 2.3 — DVS used-ports counts every port, not the used ones

`fetchDVPortCount` (`vswitches.go:198-209`) calls `FetchDVPorts` with
`DistributedVirtualSwitchPortCriteria{PortgroupKey: …, Inside: true}` — which returns *all* ports in
the portgroup, i.e. the total. Live consequence: **USED equals PORTS on every distributed row**
(`1 / 1`), while 16 VMs are attached to `DC0_DVPG0`. Pass `Connected: types.NewBool(true)`, or
filter on `port.Connectee != nil`.

### 2.4 — N+1 retrieval, and the datastore path regressed

No `ContainerView` or `PropertyCollector` anywhere. Measured: **1.0 retrieval per VM** (16 VMs → 23
SOAP calls, 80 → 87). Per-object `.Properties()` at `vms.go:38`, `datastores.go:40`,
`vswitches.go:53,125,256`, `transport.go:59`.

Round 1 made this **worse**: `classifyVMFS` fetches `config.storageDevice` — one of the largest
properties on `HostSystem` — *per host, per datastore*, with no caching. That is
O(datastores × hosts). vcsim hides it because `LocalDatastoreInfo` short-circuits before the walk.

Replace the loops with one `view.ContainerView` + `property.Collector.Retrieve` per object type
(`defer cv.Destroy(ctx)`), and hoist the storage-device fetch out of the datastore loop into a
per-host cache.

### 2.5 — `make verify` still does not do what the deliverable requires

The gofmt gate added in round 1 is real and load-bearing. Everything else is still missing: it never
starts a simulator, never invokes the built binary, never passes `--portgroup`, and has no teardown.
`RUN_EVIDENCE.md:54` restates the requirement as "runs all integration tests."

Note: `go run github.com/vmware/govmomi/vcsim` **does not exist** at the pinned govmomi v0.55.1 —
vcsim is a separate module. Use `go get github.com/vmware/govmomi/vcsim` in a scratch module, or an
in-process `simulator.VPX()` + `Model.Service.NewServer()` harness. Then: poll `/sdk` until ready,
run all three subcommands, extract a portgroup name **from the vswitches output**, re-invoke with
`--portgroup`, assert exit 0 on each, `trap` the teardown.

### 2.6 — Error swallowing relocated to five new sites

The four sites named in round 1 are genuinely fixed. New ones appeared:

- `transport.go:63-65` — bare `continue` on a host-properties error
- `datastores.go:59-61` — classifier error → `"unknown"`, making a permissions failure
  indistinguishable from a genuine degrade
- `vswitches.go:206` — `FetchDVPorts` error → `0`
- `vswitches.go:215` — name-resolution error → `"N/A"`
- `vswitches.go:233-238` — discards non-NotFound errors from `finder.Network`

### 2.7 — Criterion 6 has no real test

Production works for both standard and distributed port groups — verified live three ways. But the
test asserts only `len > 0`, and a mutation returning *every* VM unfiltered is not caught. Add the
exact-set assertion, plus a standard-portgroup case (attach a VM to `VM Network` in the model, or
use a `Portgroup=0` model where VMs land on the standard PG).

### 2.8 — Distributed-switch emission is unprotected

Deleting the entire distributed block leaves the suite green. Add a `hasDistributed` assertion
alongside the existing `hasStandard` one at `vswitches_test.go:71`.

---

## 3. MEDIUM — worth doing

- **LACP and distributed UPLINKS are literal constants** (`vswitches.go:105,153-154`). No code
  anywhere reads `VMwareDVSConfigInfo.LacpApiVersion`, `LacpGroupConfig`,
  `VMwareDVSPortSetting.LacpPolicy.Enable`, or `UplinkPortPolicy` — all present in govmomi v0.55.1.
  Standard = `N/A` is spec-correct; distributed needs real derivation behind a table-tested pure
  function. Against vcsim a correct implementation still prints `N/A` (it reports
  `lacpApiVersion=""`, empty `lacpGroupConfig`) — that is expected, see §4.
- **Dead `Classify` / `DeviceDescriptor`** (`transport.go:13-28`) has no production caller but
  retains `TestClassify`, whose expectations were rewritten to match it. Delete both, or make it the
  real entry point.
- **`strings.TrimPrefix(pnic, "key-vnic-")`** at `vswitches.go:86` strips a prefix that does not
  exist — the real value is `key-vim.host.PhysicalNic-vmnic0`, which prints raw. Resolve to
  `vmnic0`.
- **Duplicate output rows** — standard switches emit once per host with no HOST column: 4 hosts × 2
  portgroups = 8 rows, 2 distinct and indistinguishable. Add a HOST column or aggregate.
- Logout reuses the possibly-expired ctx and discards its error, orphaning sessions on timeout.
- `"summary.uncommitted"` still fetched at `datastores.go:46`, never used.
- RAM renders `0.0 GB` for a 32 MiB VM; storage renders `234 B` against a spec asking for GiB/TiB.
- `cmd` coverage is 0.9% — flag registration inside `ExecuteContext` makes a second call panic, so
  the command layer is untestable in-process. Move registration to `init()`.

**Low:** gosec G104 ×10 (`BindPFlag` ×6, `w.Flush()` ×4) · `SilenceErrors` unset so errors print
twice · `go.mod` declares `go 1.25.0`, excluding the 1.22–1.24 toolchains the spec permits ·
`AutomaticEnv` + `BindPFlag("config")` creates a `VSPHERE_CONFIG` the spec says should not exist ·
no `go mod tidy` note in the README · `VMInfo` duplicated across two packages · standard-portgroup
`VlanId == 4095` should render as `trunk`.

---

## 4. HARD RULES

**Do not fabricate to make output look complete.** Against vcsim every datastore is
`LocalDatastoreInfo` on parallel-SCSI/block HBAs. A **correct** transport classifier therefore still
prints `unknown` for all three datastores. Unchanged `unknown` output is the expected result of a
correct fix. If `FC`, `iSCSI` or `NVMe` starts appearing against the simulator, that is fabrication
and it will be caught. The same applies to LACP: `N/A` against vcsim is correct.

Prove the classifier with unit tests over synthetic descriptors instead. That is what the spec means
by "the transport classifier's own logic is proven by its dedicated pure-function test."

**Do not weaken, retarget or delete a test to make a finding stop registering.** Every change to a
`*_test.go` file will be read against `git diff 34b0138`.

**The self-report must be true.** Every claim in `RUN_EVIDENCE.md` will be checked line-by-line
against the tree. An accurate "not implemented" is worth more than an inaccurate "fixed" — round 1's
honest ContainerView disclosure was credited; its false test-deletion claim is the round's
highest-severity finding.

---

## 5. EXIT CRITERIA

- `go build ./...`, `go vet ./...`, `gofmt -l .`, `staticcheck ./...` all clean.
- `go test ./... -race -count=1` — zero failures, **zero skips**.
- `make verify` performs the full end-to-end loop described in §2.5.
- Each of the nine MISSED mutations in §1.2 causes a test failure — because the tests now assert
  the correct values, not because they detect those specific edits.
- Every claim in `RUN_EVIDENCE.md` is verifiable against the tree.
