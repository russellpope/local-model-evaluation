# qwen3.8-27b-bf16 — Mutation Battery

**Submission:** `qwen3.8-27b-bf16/vsphere-inventory` (govmomi v0.46.3, Go 1.26.5, darwin/arm64)
**Method:** clean `git archive` extraction into an isolated temp tree; every mutation applied,
grep-verified, tested, reverted, and the revert grep-verified.
**Date:** 2026-08-14

**Headline: 55 mutations, 22 CAUGHT, 33 SURVIVED (60% survival).**

---

## Negative control — START

Isolated extraction verified byte-identical to the submission first
(`diff -r` clean; the only difference is the pre-built `vsphere-inventory` binary in the original).

```
$ go build ./...
go build ./...  -> exit 0
$ go vet ./...
go vet ./...    -> exit 0
$ go test ./...
?   	vsphere-inventory	[no test files]
?   	vsphere-inventory/cmd	[no test files]
ok  	vsphere-inventory/config	0.252s
ok  	vsphere-inventory/inventory	1.238s
go test exit=0
```

Green. Downstream results are valid.

## Negative control — END

After all 55 mutations and reverts:

```
$ diff -r . <submission>          # (source identical; only the original's binary differs)
(no differences)
$ go build ./...
go build ./...  -> exit 0
$ go vet ./...
go vet ./...    -> exit 0
$ go test ./...
ok  	vsphere-inventory/config	(cached)
ok  	vsphere-inventory/inventory	(cached)
go test exit=0
$ grep -rn 'MUTANT' . --include='*.go'
grep MUTANT rc=1 (1 = no matches, revert clean)
```

Green. Reverts were clean. Additionally the driver re-verified `landed=True` and
`revert_clean=True` for **all 55** mutations individually — no mutation silently failed to apply.

> Integrity note: one mutation (M29, `Ports: int(sw.NumPorts),`) was initially **refused** by the
> driver because its anchor occurred twice in `switch.go`. It was re-run with an explicit
> occurrence count of 2 so both emit sites were patched. Without that guard it would have been a
> fake result. A second full re-run was performed in a private directory after a concurrent
> process clobbered the first working tree; all numbers below come from that single clean run.

---

## What vcsim actually produces (measured, not assumed)

Instrumented `ListDatastores` / `ListVMs` / `ListSwitches` against the submission's own test model
(`simulator.VPX()`, 1 DC / 1 host / 2 DS / 3 VM / 1 PG). This is the ground truth that explains
almost every survivor:

```
INSTR-DS: name=LocalDS_0 TYPE=unknown cap=10995116277760 avail=10962904023040 used=32212254720
INSTR-DS: name=LocalDS_1 TYPE=unknown cap=10995116277760 avail=10995116277760 used=0
INSTR-VM: name=DC0_H0_VM0 vcpu=1 ram=33554432 storage=32
INSTR-VM: name=DC0_H0_VM1 vcpu=1 ram=33554432 storage=32
INSTR-VM: name=DC0_H0_VM2 vcpu=1 ram=33554432 storage=32
INSTR-SW: sw=DVS0     type=distributed pg=DC0_DVPG0         vlan=0      uplinks=unknown lacp=disabled ports=0 used=0
INSTR-SW: sw=DVS0     type=distributed pg=DVS0-DVUplinks-8  vlan=0-4094 uplinks=unknown lacp=disabled ports=0 used=0
INSTR-SW: sw=vSwitch0 type=standard    pg=VM Network        vlan=0      uplinks=unknown lacp=N/A      ports=0 used=0
```

Four facts follow:

1. **`ClassifyTransport` is never called from the production path.** The instrumentation probe
   placed immediately before the call site never fired, and the coverage profile confirms
   `datastore.go:215` (`if t := ClassifyTransport(...)`) has **hit count 0**. The classifier is
   exercised *only* by its standalone table test.
2. **The transport traversal dead-ends at volume matching.** vcsim's VMFS volumes are named
   `datastore1` / `OSDATA-deadbeef-…` while the datastores are `LocalDS_0/1`, and the datastore URL
   is not a `vmfs/<uuid>` path so `vmfsUUID` returns `""`. The guard
   `if vol.name != ds.Name && (uuid == "" || vol.uuid != uuid) { continue }` therefore skips every
   volume. (Their extents are the literal string `____simulated_volumes_____` anyway.) HBA data
   *is* present — `scsiLun=2, hbas=3, ScsiTopology != nil` — but is never reached.
3. **Every port count is 0.** vcsim's `vSwitch0` reports `NumPorts=0, NumPortsAvailable=0, Pnic=[]`;
   the DVS reports `MaxPorts=0, NumPorts=0`. So `PORTS`/`USED`/`UPLINKS` are structurally
   zero/unknown, and the test's only guard (`Used <= Ports`) is `0 <= 0`.
4. **VM values are near-degenerate.** RAM is 32 MiB, committed storage is **32 bytes** (renders as
   `0.0GiB`), vCPU is 1. Assertions of the form `> 0` / `>= 0` are satisfied by almost anything.

---

## Mutation table

`file:line` shown is the grep-verified mutated line taken from the post-patch tree.

### CAUGHT (22)

| ID | Mutation | Grep proof | Caught by |
|----|----------|-----------|-----------|
| M01 | ClassifyTransport short-circuits: always FC | `transport.go:41: return TransportFC` | TestClassifyTransport |
| M02 | ClassifyTransport short-circuits: always unknown | `transport.go:41: return TransportUnknown` | TestClassifyTransport |
| M03 | Invert FC ↔ iSCSI in the HBA-kind switch | `transport.go:54: return TransportISCSI` / `:56: return TransportFC` | TestClassifyTransport |
| M04 | Delete the NFS-wins-over-HBA rule | `transport.go:41: // NFS rule deleted` | TestClassifyTransport |
| M05 | GiB divisor `1<<30` → `1<<29` | `format.go:8: gib = int64(1) << 29` | TestFormatBytes |
| M06 | Rounding `%.1f` → `%.0f` | `format.go:22: fmt.Sprintf("%.0fGiB", gibf)` | TestFormatBytes |
| M07 | FormatBytes returns a constant | `format.go:15: return "1.0GiB"` | TestFormatBytes |
| M08 | UsedBytes returns the zero value | `format.go:28: return 0` | TestUsedBytes, TestListDatastores |
| M14 | ListVMs sorts descending | `vm.go:45: … infos[i].Name > infos[j].Name` | TestListVMs |
| M15 | Datastore capacity hardcoded to 1 TiB | `datastore.go:51: CapacityBytes: 1 << 40` | TestListDatastores |
| M17 | Datastore used = capacity (UsedBytes bypassed) | `datastore.go:47: used := ds.Summary.Capacity` | TestListDatastores |
| M33 | vmInPortgroup always true | `portgroup.go:85: return true` | TestVMsInPortgroup |
| M34 | vmInPortgroup always false | `portgroup.go:85: return false` | TestVMsInPortgroup |
| M35 | Portgroup name match ignores the requested name | `portgroup.go:45: if objs[i].Name != ""` | TestVMsInPortgroup |
| M37 | Precedence swap: flags demoted to the DEFAULT layer | `config.go:71: if f.Changed {` | TestPrecedence |
| M38 | Precedence swap: env layer disabled | `config.go:57: // v.AutomaticEnv()` | TestPrecedence, TestDefaults, TestLoadEnvOnly |
| M39 | Load returns DefaultTimeout, ignores resolved value | `config.go:95: cfg.Timeout = DefaultTimeout` | TestPrecedence, TestLoadEnvOnly |
| M40 | Load skips the required-URL check | `config.go:97: // check deleted` | TestLoadRequiresURL |
| M43 | standardSwitches returns nothing | `switch.go:54: return nil, nil` | TestListSwitches |
| M44 | distributedSwitches returns nothing | `switch.go:163: return nil, nil` | TestListSwitches, TestVMsInPortgroup |
| M53 | Drop `config.hardware.memoryMB` from the property list | `vm.go:71: // memoryMB property dropped` | TestListVMs |
| M54 | Drop the `network` property from the property list | `vm.go:74: // network property dropped` | TestVMsInPortgroup |

### SURVIVED (33)

| ID | Mutation | Grep proof | Test that should have caught it, and why it cannot |
|----|----------|-----------|-----------------------------------------------------|
| M09 | VM RAM hardcoded to a plausible 4 GiB | `vm.go:54: info.RAMBytes = 4 * gib` | **TestListVMs** asserts only `RAMBytes > 0`. No expected value; real value is 32 MiB, so any positive constant passes. |
| M10 | VM vCPU hardcoded to 2 | `vm.go:53: info.VCPU = 2` | **TestListVMs** asserts only `VCPU > 0`. Real value is 1; the assertion cannot distinguish 1 from 2. |
| M11 | committedBytes body deleted → 0 | `vm.go:88: return 0` | **TestListVMs** asserts `StorageBytes >= 0`. 0 satisfies it. DoD #3 ("consumed, not provisioned") is therefore untested — the *entire* committed-storage derivation can be deleted. |
| M12 | committedBytes hardcoded to 42 GiB | `vm.go:88: return 42 * gib` | Same assertion. Real value is 32 bytes; a fabricated 42 GiB is indistinguishable to the suite. |
| M13 | collectVMs swallows the API error, returns nil | `vm.go:78: return nil, nil` | No test induces a property-collector failure. The error branch is never entered, so nothing can observe it being swallowed. |
| M16 | Capacity/available/used **all** fabricated, self-consistent | `datastore.go:52-54: 1<<40 / 400<<30 / 624<<30` | **TestListDatastores** checks only internal consistency (`used+available == capacity`, `available <= capacity`). Any self-consistent triple passes. This is the spec's own prescribed assertion (see requirements attack). |
| M18 | Datastore TYPE hardcoded to FC; real traversal bypassed | `datastore.go:39: _, err = hostStorages(…)` / `:50: Type: TransportFC` | **TestListDatastores** checks only `Type ∈ {FC,iSCSI,NVMe,NFS,unknown}` — a set-membership assertion satisfied by any constant. |
| M19 | datastoreTransport body deleted → always unknown | `datastore.go:196: return TransportUnknown` | Same. `unknown` is the real vcsim answer anyway, so the honest and the gutted implementation are observationally identical. |
| M20 | vmfsUUID body deleted → always `""` | `datastore.go:227: return ""` | Untested; and vcsim already yields `""`, so the function is a no-op on the tested path. |
| M21 | hostStorages swallows the API error | `datastore.go:89: return nil, nil` | No failure injection; error branch never entered. |
| M22 | hbaDescriptor forces every HBA class to fibrechannel | `datastore.go:163: Kind: "fibrechannel"` | Never observed: `ClassifyTransport` is not reached from production (coverage hit count 0 at the call site). |
| M23 | **Standard portgroup name-based fallback matching removed** | `switch.go:121: _ = pgByName` | **TestListSwitches**. Verified effect: the standard row degrades from `pg=VM Network vlan=0` to `pg=- vlan=-`. The test asserts only that *some* row has `SwitchType=="standard"`; the degradation row still qualifies. No assertion on any standard PORTGROUP or VLAN value exists. |
| M24 | Key-based portgroup index never populated | `switch.go:101: _ = pg.Key` | Same test. vcsim references portgroups by **name** (`sw.Portgroup=[VM Network]` vs `pgByKey=[key-vim.host.PortGroup-VM Network]`), so the key path never fires under test — it is dead code in the tested configuration. |
| M25 | Standard portgroup VLAN hardcoded to 100 | `switch.go:150: VLAN: "100"` | **TestListSwitches** uses `validVLAN()`, which only checks the string *parses as* an integer or range. Any numeral passes. |
| M26 | dvpgVLAN body deleted → always `"0"` | `switch.go:285: return "0"` | Same. `"0"` parses. The trunk-range logic (DoD "show the range or type") is entirely unasserted. |
| M27 | DVS total ports hardcoded to 6144 | `switch.go:231: total := 6144` | **TestListSwitches** asserts only `Used <= Ports`. Real values are 0/0; a fabricated 6144 still satisfies it. |
| M28 | DVS used ports hardcoded to 8 | `switch.go:232: used := 8` | Same — and note 8 ≤ 6144 in the baseline too. Survives even though DoD #5 demands `used = total − available`. |
| M29 | Standard vSwitch total ports hardcoded to 1024 (both sites) | `switch.go:139,153: Ports: 1024` | Same. |
| M30 | Standard vSwitch used ports hardcoded to 0 | `switch.go:115: used := int64(0)` | Same. Real value is already 0, so the `used = total − available` computation is unobservable. |
| M31 | DVS LACP always reported enabled | `switch.go:219: lacp := "enabled"` | **TestListSwitches** checks `LACP ∈ {enabled,disabled,N/A}` and separately that *standard* rows are `N/A`. The distributed value is a free choice. |
| M32 | DVS uplink names dropped | `switch.go:214: _ = ref` | No test touches the `Uplinks` field at all. Baseline is already `unknown`. |
| M36 | viewRefs swallows the `Find` error, returns nil | `collect.go:34: return nil, nil` | No failure injection; error branch never entered. |
| M41 | parseVCenterURL builds an unusable `ftp://` URL | `root.go:65: s = "ftp://" + s` | **No test exists.** `cmd/` has 0.0% coverage. |
| M42 | runWithClient never attaches credentials | `root.go:90: if false && cfg.Username != ""` | **No test exists.** This is the exact defect class that failed a prior model (govmomi empty-userinfo login); nothing in this suite would catch its return. |
| M45 | newHostStorage body deleted (no volumes/LUNs/HBAs) | `datastore.go:104: return hs` | The whole host-storage model builder can be removed with no test failure: its only consumer, `datastoreTransport`, already produces `unknown` under vcsim. |
| M46 | joinOrUnknown always returns `"unknown"` | `switch.go:311: return "unknown"` | `Uplinks` is never asserted by any test. |
| M47 | hbaDescriptor returns the zero descriptor | `datastore.go:161: return HBADescriptor{}` | Never observed (call site unreached). |
| M48 | ListDatastores never fetches host storage at all | `datastore.go:39: var hosts []*hostStorage` | Deletes an entire API round trip plus its error handling; `Type` is already `unknown`, so output is unchanged. |
| M49 | VMFS volume↔datastore matching disabled | `datastore.go:203: if true { continue }` | The match already never succeeds under vcsim (names/UUIDs don't line up), so disabling it changes nothing observable. |
| M50 | LUN→adapter topology walk deleted | `datastore.go:121: _ = lun` | Unreachable from the tested path. |
| M51 | VMFS volume matching **inverted** | `datastore.go:202: if vol.name == ds.Name \|\| …` | Inverting the predicate makes non-matching volumes "match", but their extents are `____simulated_volumes_____` with no disk entry, so the result is still `unknown`. |
| M52 | networkViewTypes reduced to `Network` only | `portgroup.go:20` | Survives because vcsim's `DistributedVirtualPortgroup` objects are *also* returned by the `Network` view type, so the explicit subtype listing is redundant in the tested model. DoD #6's "must work for distributed port groups" is thus not proven by the subtype list. |
| M55 | DVS portgroup retrieval drops `config` (the VLAN source) | `switch.go:244: []string{"name"}` | **TestListSwitches**: with `config` unretrieved, `pg.Config.DefaultPortConfig` is nil and `dvpgVLAN` returns `"0"`, which `validVLAN` accepts. |

---

## Coverage

`go test -coverpkg=./... ./...` — total **66.6%** of statements.

**Source files with ZERO test coverage:**

| File | Coverage |
|------|----------|
| `main.go` | 0.0% |
| `cmd/root.go` (`Execute`, `resolveConfig`, `parseVCenterURL`, `runWithClient`) | 0.0% |
| `cmd/vms.go` (`printVMs`) | 0.0% |
| `cmd/datastores.go` (`printDatastores`) | 0.0% |
| `cmd/vswitches.go` (`printSwitches`, `printPortgroupVMs`) | 0.0% |

The entire `cmd` package — connection handling, credential attachment, URL normalization, timeout
context derivation, logout, and **all tabwriter output formatting** — is untested. The spec's
output requirements (aligned columns, header row, plain greppable text, consistent units) have no
automated verification whatsoever; M41 and M42 confirm this empirically.

**Does any test exercise the production code path, or only a parallel helper?**

Mixed, and the split matters:

- **Genuine production-path tests:** `TestListVMs`, `TestListDatastores`, `TestListSwitches`,
  `TestVMsInPortgroup` all call the real exported functions against a real in-process vCenter.
  They are not parallel helpers. M43/M44/M53/M54 prove they are load-bearing for *structure*
  (row counts, presence of both switch types, which properties are retrieved).
- **The one place where a parallel helper substitutes for the production path is the transport
  classifier.** `TestClassifyTransport` is a pure table test of `ClassifyTransport`, and it is the
  *only* thing that exercises it — the coverage profile shows the production call site
  (`datastore.go:215`) at hit count 0, and the instrumentation probe confirms it never fires.
  M01–M04 are caught by the pure test; M18/M19/M22/M45/M47/M48/M49/M50/M51 — every mutation to the
  wiring that would connect that classifier to real API data — survive. The classifier is proven
  correct in isolation and proven **unconnected** in practice, at least under vcsim.

To be fair to the model: the spec *explicitly* sanctions this split ("because vcsim cannot model
storage transport, factor the FC/iSCSI/NVMe decision into a pure function… This is how you actually
prove criterion 4's logic"). The submission followed instructions. What the mutation battery shows
is that the instruction buys much less assurance than it appears to — the traversal that feeds the
classifier is completely unverified, and could be deleted wholesale without a red test.

---

## Requirements attack — defects in the eval prompt itself

Read of `qwen3.8-27b-bf16/govmomi-cli-eval-prompt.md`. Each is surfaced with a proposed
resolution; **none is silently resolved.** Instrument defects are charged to the instrument.

**R1 — `unknown` is simultaneously required and excluded from the `TYPE` enumeration.**
§Subcommand 2 states TYPE is "one of `FC`, `iSCSI`, `NVMe`, or `NFS`" — `unknown` is absent.
§Unit tests item 2 requires TYPE ∈ `FC/iSCSI/NVMe/NFS/**unknown**`, and §Simulator fidelity says
degrading to `unknown` is "expected and acceptable". A model reading only §Subcommand 2 would be
pushed toward fabricating a protocol.
*Proposed resolution:* amend §Subcommand 2 to "one of `FC`, `iSCSI`, `NVMe`, `NFS`, or `unknown`
when the transport cannot be truthfully derived from the API".

**R2 — the prescribed VM storage assertion punishes the honest degradation the spec demands.**
§Unit tests item 1 requires asserting "storage ≥ 0". §Simulator fidelity and §Do not fabricate
require unpopulated fields to degrade to `unknown`. The submission's honest sentinel for unknown
committed storage is `StorageUnknown = -1` (`vm.go:16`), which renders as `"unknown"` — and which
`storage >= 0` would **fail**. The spec's test rule and its honesty rule are in direct conflict; the
only way to satisfy both is for the unknown case never to arise, which is luck, not design.
*Proposed resolution:* restate as "storage ≥ 0, or the documented unknown sentinel, and the
rendered value is `unknown` in that case".

**R3 — the prescribed datastore assertion is satisfied by any internally consistent fabrication.**
§Unit tests item 2 asks only that "`used + available` is consistent with capacity (within rounding)
and `available ≤ capacity`". M16 proves this is decorative: replacing all three fields with
invented constants passes. The spec is *prescribing* a tautology-prone assertion, so the model
cannot be charged for writing exactly what was asked.
*Proposed resolution:* add "and at least one field must be tied to a value obtained independently
of the function under test (e.g. the datastore capacity configured in the simulator model, or a
second direct property query)".

**R4 — the same defect in the VM assertions.** §Unit tests item 1 specifies `vCPU > 0`, `RAM > 0`,
`storage ≥ 0` — three range checks with no expected values. M09–M12 all survive. Given vcsim's
values (1 vCPU, 32 MiB RAM, 32 bytes storage), no range assertion can distinguish real from
fabricated.
*Proposed resolution:* require at least one exact-value assertion against the configured model
(e.g. assert RAM equals the simulator's configured `memoryMB × 1 MiB` for a named VM).

**R5 — DoD #5 demands `used ports = total − available`, but vcsim reports 0/0 for every switch.**
Measured: `vSwitch0 NumPorts=0 NumPortsAvailable=0`; DVS `MaxPorts=0 NumPorts=0`. The subtraction is
therefore unobservable, and M27–M30 all survive. **This is the impossible-honest-non-zero trap.** An
auditor who demands a non-zero `PORTS` value as evidence that criterion 5 works will be demanding
something vcsim cannot supply — and will induce exactly the fabrication the spec forbids. (This is
the same failure mode that produced the auditor-induced fabricated DVS port count in a prior run.)
*Proposed resolution:* state explicitly in §Simulator fidelity that `PORTS`/`USED` of `0`/`0`
against vcsim is the correct honest result and must not be scored as a defect; move any non-zero
port-count expectation to the live-vCenter section.

**R6 — DoD #6 requires the `--portgroup` lookup to work for standard port groups, but the
prescribed model cannot demonstrate it.** In `simulator.VPX()` all VMs attach to the DVPG; the
standard `VM Network` has no VMs. The submission's test can therefore only assert the *empty* case
for a standard port group — a negative test. §Unit tests item 4's "assert the lookup returns exactly
that set" is only achievable for the distributed case without additional model wiring the prompt
never mentions.
*Proposed resolution:* either instruct the model to reconfigure at least one VM's NIC onto a
standard port group inside the test, or downgrade criterion 6's standard-portgroup evidence to
"same code path, verified by construction" and say so.

**R7 — LACP `disabled` vs `unknown` is ambiguous.** §Subcommand 3 permits `N/A` *or* `disabled` for
standard switches; §Simulator fidelity says LACP "may legitimately return `unknown`/`N/A`"; §Do not
fabricate forbids asserting facts not derived from the API. The submission reports `disabled` for a
DVS whose `LacpGroupConfig` is empty. That is defensible (an empty LACP group list is a real
API-derived negative), but a strict reading of "do not fabricate" would call it an unearned claim.
M31 shows the test cannot tell either way.
*Proposed resolution:* state whether an empty `LacpGroupConfig` on a `VMwareDVSConfigInfo` counts as
API-derived `disabled` (recommended) or must render `unknown`, so auditors do not charge a
correct implementation.

**R8 — RAM units contradict the global formatting rule.** §Subcommand 1 says RAM is "shown in
**GB**"; §Output & formatting rules mandates "consistent units (**GiB**/TiB) with one decimal
place". The submission uses GiB.
*Proposed resolution:* change §Subcommand 1 to GiB.

**R9 — the spec sanctions the exact test/production split that hides the transport bug.**
§Unit tests, transport classifier: "factor the FC/iSCSI/NVMe decision into a pure function… **This
is how you actually prove criterion 4's logic**." The battery shows the pure test proves the
classifier and nothing else: the HBA→LUN→volume→extent traversal that feeds it can be deleted
entirely (M45, M48, M49, M50) with a green suite. The prompt overstates what the pure test proves.
*Proposed resolution:* soften to "…proves the classifier's decision table. The traversal that
supplies its inputs is not covered by vcsim; state explicitly how it was verified (live vCenter, or
a hand-built `mo.HostSystem` fixture) and consider requiring a fixture-driven test of
`datastoreTransport` with a synthetic host storage topology."

---

## Single most damaging survivor

**M23 — removing the name-based portgroup fallback in `standardSwitches` (`switch.go:121`).**

Its blast radius is the largest of any survivor and it is the only survivor that visibly corrupts
user-facing output rather than merely leaving an unverifiable field unverified. With M23 applied
the standard-switch row degrades from

```
vSwitch0  standard  VM Network  0  unknown  N/A  0  0
```

to

```
vSwitch0  standard  -           -  unknown  N/A  0  0
```

— the `PORTGROUP` and `VLAN` columns for every standard vSwitch are lost, i.e. a documented,
required feature of subcommand 3 stops working — and `go test ./...` stays fully green. It survives
because `TestListSwitches` asserts only that *some* row carries `SwitchType == "standard"`, and the
`Portgroup: "-"` degradation row satisfies that. There is no assertion anywhere in the suite that
binds a standard switch to a named port group.

It is more damaging than the transport survivors (M18/M19/M45/M48–M51) because those all resolve to
`unknown` under vcsim either way — the spec explicitly blesses that outcome, so they are latent
risks on a live vCenter rather than present regressions. M23 breaks output the simulator *can*
render correctly today.

**Runner-up: M42** (`runWithClient` never attaches credentials, `cmd/root.go:90`) — a silent
authentication break in a package with 0.0% coverage. Nothing in the suite could ever detect it,
and it is the precise defect class that has sunk prior submissions in this eval series.

---

## Reproduction

```
cd /Users/ldh/Projects/github.com/local-model-evaluation
git archive $(git write-tree) qwen3.8-27b-bf16/vsphere-inventory | tar -x -C <tempdir>
# driver + raw results (apply / grep-verify / test / revert / grep-verify per mutation):
#   <scratchpad>/mutbattery-iso/driver.py
#   <scratchpad>/mutbattery-iso/results_final.json   (55 records, all landed=true, revert_clean=true)
```
