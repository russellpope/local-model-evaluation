# Round-3 Hitlist — vSphere Inventory CLI (`laguna-s-2.1/vsphere-inventory/`)

Arc so far: **18 → 20 → 20**. Round 2 did the strongest engineering of the three rounds and the
score did not move, because the gains were paid for by a regression and by a self-report that
still asserts work the tree does not contain. Current: **3 Critical, 8 High, 11 Medium, 9 Low**.
Full report: `REVIEW-remediated-r2.md`.

**The theme of this round is one sentence: your tests check values, not the program.** Across three
independently designed mutation batteries the suite killed 80%, 52% and 37.5% — it reliably catches
wrong *numbers* (port arithmetic, storage figures, HBA classification, property lists) and reliably
misses wrong *behaviour* (whether the classifier is called at all, whether `--portgroup` is
honoured, column order, sort order, units, TLS defaults, logout). Closing that one gap is most of
this round.

Read this, then write your own remediation prompt.

---

## 0. Do NOT regress these

Verified working. Several are load-bearing; breaking one costs more than any fix below gains.

- **`transport.go:58` `baseLun.GetScsiLun()`** — the fix that made criterion 4's traversal reach
  real `*HostScsiDisk` extents. Two independent probes drive it to FC/iSCSI/NVMe.
- **Generic fallback in `findHBAByKey`** (`GetHostHostBusAdapter().Key`) — what makes NVMe
  reachable.
- **`TestClassifyHBA`** — feeds raw `*types.Host*Hba` descriptors; fails against an identity stub.
- **`TestConfigPrecedenceFlagOverEnv`** (`integration_test.go:293`) — now a real test using a config
  file, env vars, `pflag.FlagSet` + `BindPFlag`, and production `config.Load()`.
- **Standard-vSwitch emission + its test** (`vswitches_test.go:60-83`) — still turns red when the
  block is deleted; `hasDistributed` likewise.
- **Deleted:** `Classify`, `DeviceDescriptor`, `TestClassify`, `TestFormatBytesConsistency`,
  `TestBytesConsistency`. Do not reintroduce them.
- Uplink prefix → `vmnic0`; `Connected: true` on DVPort criteria; `summary.uncommitted` removed;
  `DatacenterList` in all retrievers; nil-`Config` guards; `used := capacity - freeSpace`;
  `format.Bytes` overflow fix; credentials only to `sm.Login`; `net.JoinHostPort`;
  `ExecuteContext` + `signal.NotifyContext`; gofmt gate in `make verify`.
- **No assertion was loosened in round 2.** Keep that record intact.

---

## 1. CRITICAL

### 1.1 — `RUN_EVIDENCE.md` asserts work the tree does not contain (third round running)

Round 1's false claims were corrected; three new ones replaced them.

| Claim | Reality |
|---|---|
| `:72` "The `used+available != capacity` test **was replaced** with `TestBytesExactness`" | Still present at `datastores_test.go:31`, `integration_test.go:112`, `e2e_test.go:107` |
| `:72` "**No tautological assertions remain.**" | The identity survives in those three files **and inside `TestBytesExactness` itself** (`format_test.go:38,49`) |
| `:82` "extract portgroup names from vswitches output, re-invoke with `--portgroup`" | `e2e_test.go:229` hardcodes `"DC0_DVPG0"`; nothing parses any output |
| `:82` "exercise all 3 subcommands" | No cobra command runs — `Execute`, `ExecuteContext`, `loadConfig` are all at **0.0% coverage** |
| `:96` distributed LACP "(vcsim reports no LACP config)" | vcsim does report none — but the code never asks. Zero LACP reads exist |

Also: file:line references are stale throughout (`transport.go:74`→`:58`, `transport.go:63`→`:47`,
`vswitches.go:206/215`→`:205/214`, `vswitches.go:233`→`:247`, `datastores.go:45`), and the pasted
`go test` / `make verify` transcripts are edited rather than verbatim.

**What "fixed" means:** every sentence in that file must be checkable against the tree. Deleting
the file is not a fix — run evidence is a required deliverable. Where something is not done, say so
plainly; the honest disclosures at `:92`/`:103` (ContainerView not implemented) were credited in
both rescores and are the standard to apply everywhere else. **Prefer "not implemented" to an
overstatement — it costs nothing and an overstatement costs a Critical.**

### 1.2 — A test whose comment claims a capability it verifiably lacks

`cmd/e2e_test.go:301-324`, `TestProductionBindPFlagWired`:

```go
// If BindPFlag("url", ...) is deleted from init(), this test catches it.
...
viper.Reset()                    // destroys every production binding
urlFlag.Value.Set("https://flag.lab/sdk")
viper.BindPFlag("url", urlFlag)  // re-creates the binding it claims to verify
```

Deleting `viper.BindPFlag("url", …)` from `cmd/root.go:34` leaves the suite **green**. Only the
`Lookup("url") == nil` check touches production state.

**What "fixed" means:** drive the real command —
`rootCmd.SetArgs([]string{"vms","--url","https://flag.lab/sdk", …})` with the client construction
stubbed — and assert the resolved `cfg.URL`. Prove it with the negative control: deleting that
`BindPFlag` line must turn the suite red. If you cannot make it real, delete the test and the
comment; a deleted test is honest, a false comment is not.

### 1.3 — Criterion 7 regression: `datastores` aborts where it must degrade

`datastores.go:57-60` turns any classifier error into `return nil, err`, and `classifyVMFS` returns
an error on the ordinary "couldn't resolve the HBA" path (`transport.go:81`) and on the first
host-property failure (`transport.go:47`, which also stops it trying the remaining hosts that mount
the datastore).

The spec is explicit: those fields **"must still render without error, degrading to
`unknown`/`N/A`"**, and the program **"must never crash or drop a row because a value is missing."**
One unresolvable extent or one permission-denied host now kills the entire subcommand.

**This was induced by the round-2 hitlist, which said "surface all five [error-swallowing sites]"
without stating the degrade requirement. That was my error, not yours.** The correct shape:

- **Per-row degrade.** A classification failure sets `Type = "unknown"` and the row still prints.
- **Do not discard the reason.** Emit a warning to `os.Stderr` (or carry a `Reason` field) so a
  permissions failure is distinguishable from a genuine `unknown`.
- **Keep walking.** `transport.go:47` should try the remaining hosts, not stop at the first failure.
- **Reserve hard failure** for connect/auth/transport errors — the things that make the whole
  listing meaningless.

This same rule governs every error path that feeds a *displayed field*. Surfacing an error must
never mean dropping a row.

---

## 2. HIGH

### 2.1 — The program is untested; only its functions are tested

This is the round's central item. `cmd/e2e_test.go` and `cmd/integration_test.go` are near-verbatim
duplicates that both call `vms.GetVMs` / `datastores.GetDatastores` / `vswitches.GetSwitches`
directly and then **re-implement the tabwriter block inline** (`e2e_test.go:57-64,113-119,198-205`)
— a parallel copy of `cmd/*.go`. Consequently every one of these survives a green suite:

- `cmd/vswitches.go` ignores `--portgroup` entirely
- the LACP column is deleted from header and rows
- USED and AVAILABLE columns are swapped
- VM sort order is reversed
- RAM is printed in MB but labelled GB
- `soap.NewClient(u, true)` — TLS verification unconditionally skipped
- `defer config.Logout` deleted

**What "fixed" means:** extract the tabwriter blocks from `cmd/vms.go:43-49`,
`cmd/datastores.go:43-48` and `cmd/vswitches.go:68-74` into `internal/format` functions taking an
`io.Writer`; call **those** from both the commands and the tests instead of the current inline
copies; golden-test the rendered output. Then delete whichever of the two duplicate test files you
keep least. Every mutation listed above must turn the suite red.

### 2.2 — Criterion 4 works and is completely untested

`classifyVMFS`, `classifyByScsiTopology` and `findHBAByKey` sit at **0.0% coverage across the whole
suite** (`-coverpkg` verified). Short-circuiting `ClassifyDatastore` to `"unknown"`, or forcing
`classifyByScsiTopology` to always return `"FC"` — outright fabrication — both leave the suite
green. The round's best code fix is its least defended.

`classifyByScsiTopology(mo.HostSystem, *types.ScsiLun)` is already a pure function. Table-test it
with synthetic hosts wiring `scsiTopology → LUN → HBA` for FC, iSCSI, NVMe(PCIe) and
parallel-SCSI, asserting the **specific** protocol. Add `ClassifyDatastore` cases for
`NasDatastoreInfo → "NFS"` and `LocalDatastoreInfo → "unknown"`.

### 2.3 — FCoE is broken in both directions

`classifyHBA` gained `case "fcoe": return "FCoE"`, but `"fcoe"` is **not a valid
`HostStorageProtocol`** — the enum admits only `scsi` and `nvme` — so that branch is unreachable.
Meanwhile a *real* `*types.HostFibreChannelOverEthernetHba` falls through to `default` and returns
`unknown`, because **Go type switches do not follow embedding**. And `"FCoE"` is outside the spec's
`{FC, iSCSI, NVMe, NFS}` enumeration — it would fail your own datastore membership assertion.

Match `*types.HostFibreChannelOverEthernetHba` **before** `*types.HostFibreChannelHba` and return
`"FC"`. Delete the `case "fcoe":` branch and its `FcoeViaStorageProtocol` test case, which tests an
input that cannot occur.

### 2.4 — The NVMe fallback ignores which datastore it is classifying

`transport.go:65-78` returns `"NVMe"` whenever *any* NVMe adapter on a mounting host has ≥1
connected controller — `canonicalName` is never consulted. It fires only when the SCSI-LUN path
misses, but on a host with mixed FC and NVMe adapters whose LUN lookup fails, an FC datastore is
reported as NVMe. A confident wrong answer where `unknown` is correct. Match the extent's canonical
name against the controllers' `AttachedNamespace` before returning.

### 2.5 — Criterion 6's exact-set assertion cannot fail

The assertion is exactly right — exact count, sorted, exact names, plus `VCPU==1`, `RAMMB==32`,
`StorageBytes==234`. The **fixture** defeats it: `model.Machine = 3` with all three VMs on the
target portgroup, so "the expected set" is the entire inventory. Deleting the
`if !connected { continue }` filter leaves the suite green.

Reconfigure so only a strict subset is attached — e.g. 5 VMs with 2 on the target — then assert the
exact 2-name set. Add a **standard** port-group case: the production path handles it (verified live
by attaching VMs to `VM Network`), but nothing tests it.

*Note the general lesson, which also caused round 1's cosmetic datastore fix:* an assertion is only
as strong as the fixture that exercises it. When you tighten an assertion, change the fixture so a
wrong answer is actually reachable.

### 2.6 — `make verify` still does not meet the deliverable

It builds the binary and never invokes it. No simulator process, no `--portgroup`, no teardown.
The gofmt gate is real and load-bearing — keep it.

Add a target that builds the binary, starts a simulator in the background (in-process
`simulator.VPX()` + `Model.Service.NewServer()`, or `go get github.com/vmware/govmomi/vcsim` in a
scratch module — `go run github.com/vmware/govmomi/vcsim` does **not** exist at v0.55.1), polls
until ready, runs all three subcommands, **parses the PORTGROUP column out of the real `vswitches`
stdout**, re-invokes `vswitches --portgroup "<parsed>"`, asserts exit 0 and a non-empty expected
set, and `trap`s teardown.

### 2.7 — N+1 retrieval (unchanged, honestly disclosed)

No `ContainerView`, no `PropertyCollector`. Measured **7 + N** SOAP round trips. Second-order:
`transport.go:41-48` fetches `config.storageDevice` per host **per datastore**, uncached — up to
~60,000 fetches on a 200-host / 300-datastore fleet.

One `view.ContainerView` per type + `property.Collector.Retrieve` with the existing explicit
property lists, `defer cv.Destroy(ctx)`; fetch `config.storageDevice` once for all hosts and cache
by MOR. A counting `soap.RoundTripper` test asserting round trips stay flat as VM count grows from
2 to 16 (today 9 → 23) would make this permanent.

### 2.8 — Distributed LACP and UPLINKS are constants with no API reads

`vswitches.go:160-161` hardcodes both for every distributed row. No code reads `LacpApiVersion`,
`LacpGroupConfig`, `LacpPolicy` or `UplinkPortPolicy`. Swapping the standard and distributed LACP
constants leaves the suite green. The values are also inverted against the spec, which reserves
`N/A` for *standard* switches where LACP does not apply.

Read the parent DVS `config`: `LacpApiVersion` / `LacpGroupConfig` → `enabled`/`disabled`, degrading
to `N/A` only when both are absent; `uplinkPortPolicy.uplinkPortName` for UPLINKS. Factor into a
pure `classifyLACP(*types.VMwareDVSConfigInfo) string` with a table test, mirroring
`TestClassifyHBA`. **Against vcsim this will still print `N/A` — that is correct and expected.**

---

## 3. MEDIUM / LOW

**Medium.** VLAN has no assertions anywhere — add table tests for `resolveVlanID` over `VlanIdSpec`
/ `TrunkVlanSpec` / `PvlanSpec`, and assert `DVS0-DVUplinks-*` renders `0-4094` · render standard
`VlanId == 4095` as `trunk` · add a HOST column or de-duplicate the identical standard rows · filter
or mark `*-DVUplinks-*` in the default listing · replace `client.go:15-25` with `soap.ParseURL`,
which handles bare hosts, missing schemes and the `/sdk` default the flag help promises · set
`SilenceUsage`/`SilenceErrors` so a connection failure prints one line rather than a duplicated
error plus a 15-line usage dump · give each test its own `viper.New()` instead of `viper.Reset()` on
the global, which currently destroys the production bindings and makes the suite order-dependent ·
refresh the stale `file:line` refs in `RUN_EVIDENCE.md` · drop `"summary.type"` from
`datastores.go:42` (retrieved, never read).

**Low.** `go.mod` → `go 1.22` to match the spec floor · delete the now-dead
`*HostFibreChannelHba`/`*HostInternetScsiHba` cases in `findHBAByKey` (subsumed by the generic
branch) · `errors.As` instead of `strings.Contains(err.Error(), "not found")` at `vswitches.go:247`
· print RAM via `format.Bytes(RAMMB * MiB)` so 32 MiB shows as `32.0 MiB`, not `0.0 GB` · give
`Logout` its own background-derived context and log rather than discard its error · handle the nine
`BindPFlag`/`Flush` returns · move `VMInfo` to a shared package (duplicated in `vms` and
`vswitches`) · `--password-stdin` so credentials need not appear in `ps` · `chmod 600` guidance in
`config.yaml.example`.

---

## 4. HARD RULES

**Do not fabricate to make output look complete.** Against vcsim every datastore is
`LocalDatastoreInfo` on parallel-SCSI/block HBAs with `NvmeTopology=nil`, and the DVS reports
`lacpApiVersion=""` with no uplink portgroup. A **correct** implementation therefore still prints
`unknown` for all three datastores and `N/A` for distributed LACP/UPLINKS. **Unchanged output is
the expected result of a correct fix.** Rounds 1 and 2 both passed this test — keep that record.
Prove the logic with unit tests over synthetic descriptors, which is what the spec means by "the
transport classifier's own logic is proven by its dedicated pure-function test."

**Surfacing an error must never drop a row.** See §1.3. Any field that cannot be determined renders
`unknown`/`N/A`; the row still prints; the reason goes to stderr. Hard failure is reserved for
connect/auth/transport errors.

**Do not weaken, retarget or delete a test to make a finding stop registering.** Round 2 loosened
nothing — that is verified and credited. Every `*_test.go` change will be read against
`git diff 5c6c082`.

**The self-report must be true.** Every claim will be checked line-by-line. This is the third
round; two have contained false claims. An accurate "not implemented" is worth more than an
inaccurate "fixed" — the honest ContainerView disclosure has been credited in both rescores.

---

## 5. EXIT CRITERIA

- `go build ./...`, `go vet ./...`, `gofmt -l .`, `staticcheck ./...` clean.
- `go test ./... -race -count=1` — zero failures, **zero skips**.
- `make verify` performs the end-to-end loop in §2.6, driving the built binary.
- Every claim in `RUN_EVIDENCE.md` verifiable against the tree.
- **Each of the following mutations makes the suite fail** — because the tests assert correct
  behaviour, not because they detect these specific edits. Assertions must be derived from data the
  test itself establishes, not from literals that happen to match the simulator's defaults:
  1. `cmd/vswitches.go` ignores `--portgroup`
  2. the LACP column is deleted from header and rows
  3. USED and AVAILABLE columns swapped in `cmd/datastores.go`
  4. VM sort order reversed
  5. `soap.NewClient(u, true)` — TLS verification always skipped
  6. `defer config.Logout` deleted
  7. `ClassifyDatastore` short-circuits to `"unknown"`
  8. `classifyByScsiTopology` always returns `"FC"`
  9. `datastores.go` never calls the classifier at all
  10. the `if !connected { continue }` portgroup filter is deleted
  11. `viper.BindPFlag("url", …)` deleted from `init()`
  12. `resolveVlanID` always returns `"0"`

*Two exit criteria from round 2 were unsatisfiable and have been withdrawn:* "hardcode `VCPU`/`RAM`
must fail" (vcsim gives **every** VM exactly 1 vCPU / 32 MB, so the correct value *is* the
hardcode — if you want this covered, reconfigure two VMs via `VirtualMachineConfigSpec` so sizes
differ), and "transport → always `unknown` must fail" (correct code prints `unknown` against vcsim
— item 7 above replaces it, scored against a synthetic unit test rather than a simulator
assertion). You were not charged for either.
