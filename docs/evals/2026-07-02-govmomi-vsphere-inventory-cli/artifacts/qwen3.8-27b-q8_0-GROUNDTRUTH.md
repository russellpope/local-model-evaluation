# qwen3.8-27b-q8_0 — GROUND TRUTH (govmomi v0.55.1)

**Pass:** ground truth (1 of 4). Independent; no other pass's output was read.
**Date:** 2026-08-15
**Submission:** `qwen3.8-27b-q8_0/vsphere-inventory` (copy under my private tree)
**Pinned dependency:** `github.com/vmware/govmomi v0.55.1` (`go.mod:9`)
**Prior-rung ground truth at v0.46.3 was NOT read and is VOID for this run.**

Toolchain: `go1.26.6 darwin/arm64`. Module cache:
`/Users/ldh/go/pkg/mod/github.com/vmware/govmomi@v0.55.1` (cited below as `$M`).

---

## 0. Requirements attack — defects in the instrument

These are charged to the eval prompt, **not** to the model. None is silently resolved.

### RA-1 (CRITICAL, new at v0.55.1) — the spec's prescribed simulator command does not exist

`govmomi-cli-eval-prompt.md:196` instructs:

```
go run github.com/vmware/govmomi/vcsim
```

At v0.55.1 this package is **not in the module**:

```
$ ls /Users/ldh/go/pkg/mod/github.com/vmware/govmomi@v0.55.1/vcsim
ls: ...: No such file or directory

$ go run github.com/vmware/govmomi/vcsim
no required module provides package github.com/vmware/govmomi/vcsim

$ go run github.com/vmware/govmomi/vcsim@v0.55.1 -h
go: module github.com/vmware/govmomi@v0.55.1 found, but does not contain package
    github.com/vmware/govmomi/vcsim
```

`vcsim` is a nested module in the repo (`$M/Makefile:91` — `$(MAKE) -C vcsim install`)
and is therefore excluded from the parent module zip. The spec's own claim
("it ships inside the govmomi module you already depend on, so it runs at the
same version — no extra dependency", lines 192-194) is **false at v0.55.1**.

**Proposed resolution:** the spec should say "start the simulator at the version
you depend on, by any means" and drop the verbatim command. Any submission that
worked around this is to be **credited, not penalised**. This submission's
workaround (`tools/vcsimserver/`, a separate module also pinned to v0.55.1) is
an honest and correct response. Its stated *reason* (README:139-141, "its
`go.mod` uses a `replace` directive") is not something I could confirm — the
nested `go.mod` is not in the module zip — but the practical conclusion is
verified correct.

### RA-2 — `used ports = total − available` is unsatisfiable for DVS

DoD 5 (`:169`) demands `used = total − available`. There is **no "available
ports" property on a DVS or DVPG** in the vSphere API. Ground truth at v0.55.1:
`VMwareDVSConfigInfo{NumPorts:0, MaxPorts:0}`, and per-portgroup
`DVPortgroupConfigInfo.NumPorts = 1` with no availability counter.

An auditor demanding a non-zero `USED` derived by subtraction would **induce the
fabrication the spec forbids** (`:244`). This is the same trap that produced the
fabricated "6144 ports" failure earlier in this series.

**Proposed resolution:** restate DoD 5 as "used ports is derived, not invented:
`total − available` where the API exposes availability (standard vSwitch),
otherwise a real count of ports in use (`FetchDVPorts` with the `Connected`
criterion)". This submission uses exactly that split and should be credited.

### RA-3 — `storage >= 0` vs. the honest-unknown sentinel

Unchanged from prior rungs and still live. `:136` prescribes `storage ≥ 0`.
This submission uses `int64` bytes with `0` for "no `summary.storage`" and a
string sentinel (`"unknown"`) for transport, so no contradiction actually bites
here. **No action needed for this submission**, but the spec text remains a trap
for an implementation that chooses `-1` as its unknown sentinel.

### RA-4 — the dependency allow-list is literally unsatisfiable

`:18-23` permits only govmomi, cobra, viper and stdlib. But `viper.BindPFlag`
has signature `BindPFlag(key string, flag *pflag.Flag) error` and
`cobra.Command.Flags()` returns `*pflag.FlagSet`. **Any** implementation of the
required flag > env precedence must name `pflag` types. This submission imports
`github.com/spf13/pflag` at `internal/config/config.go:10` and lists it as a
direct requirement (`go.mod:7`).

**Proposed resolution:** amend the allow-list to "govmomi, cobra, viper (and
`spf13/pflag`, which cobra/viper's API surface forces), plus stdlib". Do not
charge `pflag` as a violation. I verified no other third-party direct import
exists: `go list -deps` shows only govmomi/*, cobra, viper, pflag plus their
transitives.

### RA-5 — `RAM` in GB vs. the mandatory GiB/TiB formatter

`:63` asks for RAM "shown in GB"; `:112` mandates "consistent units (GiB/TiB)
with one decimal place" for all tables. vcsim VMs have `memorySizeMB = 32`,
which renders as `0.0 GiB`. The output is *honest but uninformative*, and this
is the spec's doing, not the model's.

**Proposed resolution:** exempt the RAM column from the GiB/TiB rule, or note
that sub-GiB rounding to `0.0 GiB` is acceptable. Do not charge it.

### RA-6 — no acceptance bar distinguishes "degraded" from "never executed"

`:180-183` blesses degrading to `unknown`/`N/A` at vcsim. Because *both* a
correctly-wired-but-starved path and a *dead* path print `unknown`, the stated
exit condition (`:225-235`) **cannot discriminate them**. That is precisely the
gap the two Critical findings below fall into.

**Proposed resolution:** add an acceptance bar that the retrieval path be
demonstrated to reach its classifier — e.g. a fixture/unit test driving the
*production* function (not the pure helper) with realistic API-shaped inputs.

---

## 1. Simulator ground truth at v0.55.1

`tools/vcsimserver/main.go:31-35` sets `simulator.VPX()` then overrides
`Datacenter/Datastore/Machine/Portgroup`. `verify.sh:49` drives it with
`-dc 1 -ds 3 -vm 2 -pg 3`. Unset VPX defaults ($M/simulator/model.go:157-173):
`Host:1, Cluster:1, ClusterHost:3`.

Derived from source and **confirmed live** by an independent probe
(`scratchpad/q8-groundtruth/probe/main.go`, govmomi v0.55.1):

| Entity | Count | Names | Source |
|---|---|---|---|
| Datacenter | 1 | `DC0` | model.go:597 |
| HostSystem | **4** | `DC0_H0` (standalone) + `DC0_C0_H0..H2` (cluster) | model.go:733-762 |
| VirtualMachine | **4** | `DC0_H0_VM0/VM1`, `DC0_C0_RP0_VM0/VM1` | model.go:741, 773 (`Machine:2` per pool) |
| Datastore | **3** | `LocalDS_0/1/2`, each mounted on all 4 hosts | model.go:815-821 |
| DVS | 1 | `DVS0` | model.go:643-660 |
| DVPortgroup | **4** | `DVS0-DVUplinks-8`, `DC0_DVPG0/1/2` | model.go:661-686 (+ implicit uplink PG) |
| Standard vSwitch | **4** (one per host) | `vSwitch0` | `$M/simulator/esx/host_config_info.go:47-52` |
| Standard portgroup | **8** (2 per host) | `VM Network`, `Management Network` | esx/host_config_info.go:156, 222 |
| Network (opaque/standard) | 1 | `VM Network` (`network-6`) | simulator/datacenter.go:96-98 |

Per-column honest values, at vcsim vs. on real vCenter:

| Object.property | vcsim value (probe output) | Real vCenter |
|---|---|---|
| `VM.summary.config.numCpu` | `1` | real |
| `VM.summary.config.memorySizeMB` | `32` | real |
| `VM.summary.storage.committed` | `234` bytes | real consumed |
| `VM.summary.storage.uncommitted` | `10737418240` | thin remainder |
| `VM.network` | all 4 → `DistributedVirtualPortgroup:dvportgroup-12` (`DC0_DVPG0`) | real |
| `Datastore.summary.type` | `"OTHER"` | `VMFS`/`NFS`/… |
| `Datastore.summary.capacity` | `4398046511104` (4 TiB) | real |
| `Datastore.summary.freeSpace` | LocalDS_0 `4355096838144`; LocalDS_1/2 == capacity | real |
| `Datastore.info.url` | `/var/folders/.../govcsim-*/DC0-LocalDS_0` (**no `ds:///` prefix**) | `ds:///vmfs/volumes/<uuid>/` |
| `HostSystem.network` | `[Network:network-6, dvportgroup-10/12/14/16]` | same shape (Network/DVPG/OpaqueNetwork) — **never `HostVirtualSwitch`** |
| `HostSystem.config.network.vswitch` | `vSwitch0 numPorts=1536 numPortsAvailable=1530 pnic=[key-…-vmnic0] bridge=BondBridge{NicDevice:[vmnic0]}` | real |
| `HostNetworkSystem.networkInfo.vswitch` | `vSwitch0 numPorts=0 avail=0 pnic=[]` (**zero stub**) | real |
| `HostStorageSystem.storageDeviceInfo` | real ESXi capture: HBAs `vmhba0` (ParallelScsi), `vmhba1`/`vmhba64` (Block); `storageProtocol=""` | real FC/iSCSI/NVMe HBAs |
| `HostStorageSystem.fileSystemVolumeInfo` | **empty** (`NewHostStorageSystem` sets only `StorageDeviceInfo`, `$M/simulator/host_storage_system.go:22-30`) | populated |
| `DVS.config` | `*VMwareDVSConfigInfo{NumPorts:0, MaxPorts:0, UplinkPortPolicy:nil, DefaultPortConfig:nil, LacpGroupConfig:nil, Host:[]}` | populated |
| `DVPortgroup.config.numPorts` | `1` (all four) | real |
| `DVPortgroup.config.defaultPortConfig.vlan` | DVPGs: `VlanIdSpec{0}`; uplink PG: `TrunkVlanSpec{0-4094}` | real |
| `FetchDVPorts(nil)` | 4 ports, all `Connectee == nil` | populated |
| `FetchDVPorts(Connected=true)` | **0 ports** | real connected count |

---

## 2. Instrument corrections — re-verified at v0.55.1

| Correction | Status at v0.55.1 | Evidence |
|---|---|---|
| `LACP N/A` is wrong for DVS; `disabled` is honest | **HOLDS.** `LacpGroupConfig` is `nil` and `DefaultPortConfig` is `nil` — LACP is genuinely not configured. `disabled` is correct; do not charge it. | probe: `lacpGroupConfig=[]types.VMwareDvsLacpGroupConfig(nil) lacpApiVersion=""` |
| "PORTS 0 is true" **splits** — true for DVS switch config, false for standard vSwitch | **HOLDS, and is refined.** DVS *switch* config is `NumPorts=0/MaxPorts=0`, but per-**portgroup** `NumPorts=1` is a real value. `HostNetworkSystem.networkInfo` is the 0/0 stub; `config.network.vswitch` carries the real **1536/1530**. | probe output, `$M/simulator/esx/host_config_info.go:51-52` |
| *Which property does this submission read for standard switches?* | **NEITHER.** It reads a managed-object type that does not exist. See C-1. | `internal/inventory/vswitches.go:71-86` |

---

## 3. Field-by-field verdicts (live run, `-dc 1 -ds 3 -vm 2 -pg 3`)

Commands run from the built binary against my own simulator on an OS-assigned
port; `sh scripts/verify.sh` also reproduced end-to-end with **exit 0**.

### `vms`

```
NAME            VCPU  RAM      STORAGE
DC0_C0_RP0_VM0  1     0.0 GiB  0.0 GiB
DC0_C0_RP0_VM1  1     0.0 GiB  0.0 GiB
DC0_H0_VM0      1     0.0 GiB  0.0 GiB
DC0_H0_VM1      1     0.0 GiB  0.0 GiB
```

| Column | Printed | Ground truth | Verdict |
|---|---|---|---|
| NAME | 4 rows, sorted | exactly the 4 model VMs | **TRUE** |
| VCPU | `1` | `summary.config.numCpu = 1` | **TRUE** |
| RAM | `0.0 GiB` | 32 MiB = 0.031 GiB | **TRUE** (lossy by spec — RA-5) |
| STORAGE | `0.0 GiB` | `summary.storage.committed = 234` B | **TRUE** — and it is **committed**, not `uncommitted` (10 GiB) |

### `datastores`

```
NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  40.0 GiB  4.0 TiB
LocalDS_1  unknown  0.0 GiB   4.0 TiB
LocalDS_2  unknown  0.0 GiB   4.0 TiB
```

| Column | Printed | Ground truth | Verdict |
|---|---|---|---|
| NAME | 3 rows, sorted | `LocalDS_0/1/2` | **TRUE** |
| TYPE | `unknown` | url has no `ds:///vmfs/volumes/` prefix → uuid `""` → unknown | **HONEST-DEGRADE** at sim (but see C-2: it is `unknown` on real vCenter too) |
| USED | `40.0 GiB` / `0.0` / `0.0` | 4398046511104 − 4355096838144 = 42949672960 = exactly 40 GiB; others 0 | **TRUE** |
| AVAILABLE | `4.0 TiB` | `freeSpace` | **TRUE** |

### `vswitches`

```
SWITCH  SWITCH TYPE  PORTGROUP         VLAN    UPLINKS  LACP      PORTS  USED
DVS0    distributed  DC0_DVPG0         0       -        disabled  1      0
DVS0    distributed  DC0_DVPG1         0       -        disabled  1      0
DVS0    distributed  DC0_DVPG2         0       -        disabled  1      0
DVS0    distributed  DVS0-DVUplinks-8  0-4094  -        disabled  1      0
```

| Column | Printed | Ground truth | Verdict |
|---|---|---|---|
| SWITCH / TYPE | `DVS0 / distributed` | correct | **TRUE** |
| **(missing rows)** | — | **4 × `vSwitch0` (standard), 8 portgroup rows** | **FALSE — omitted entirely.** See C-1 |
| PORTGROUP | 4 DVPGs incl. uplink PG | matches inventory | **TRUE** |
| VLAN | `0`, `0`, `0`, `0-4094` | `VlanIdSpec{0}` ×3, `TrunkVlanSpec{0-4094}` | **TRUE** |
| UPLINKS | `-` | `UplinkPortPolicy == nil` at vcsim | **HONEST-DEGRADE** |
| LACP | `disabled` | `LacpGroupConfig` nil → not enabled | **TRUE** (per instrument correction) |
| PORTS | `1` | `DVPortgroupConfigInfo.NumPorts = 1` | **TRUE** (real per-PG value, not the 0 switch-level field) |
| USED | `0` | `FetchDVPorts(Connected=true)` returns **0** at vcsim | **TRUE at sim / UNVERIFIABLE-AT-SIM for real semantics** — the mechanism is the right one, but vcsim never marks a port connected even though all 4 VMs are on `DC0_DVPG0` |

**Negative control.** With `-pg 0` (no DVS; a standard-switch-only inventory,
i.e. the common direct-ESXi case), `vswitches` prints **only the header** and
exits 0, while the inventory really contains 4 vSwitch0 and 8 port groups:

```
SWITCH  SWITCH TYPE  PORTGROUP  VLAN  UPLINKS  LACP  PORTS  USED
exit=0
```

### `vswitches --portgroup <name>`

| Invocation | Result | Ground truth | Verdict |
|---|---|---|---|
| `DC0_DVPG0` (distributed, 4 VMs) | 4 VMs listed | all 4 VMs on `dvportgroup-12` | **TRUE** |
| `DC0_DVPG1` (distributed, exists, 0 VMs) | `Error: port group "DC0_DVPG1" not found in inventory`, exit 1 | the port group **is** in the inventory | **FALSE** (see M-1) |
| `DVS0-DVUplinks-8` (exists, 0 VMs) | same false error | exists | **FALSE** (M-1) |
| `VM Network` (standard, 0 VMs) | same false error | exists as `Network:network-6` | **FALSE** (M-1) |
| `VM Network` **after I attached `DC0_H0_VM0` to it** | `DC0_H0_VM0` listed, exit 0 | correct | **TRUE** — the standard path is genuinely wired |
| `DC0_DVPG0` after that reattach | 3 remaining VMs | correct | **TRUE** |

---

## 4. The six tricky requirements — traced to source

### 4.1 VM storage is consumed/committed — **GENUINELY WIRED**

`internal/inventory/vms.go:37` retrieves `summary`; `:48-50` reads
`m.Summary.Storage.Committed` guarded by a nil check. `uncommitted`
(10 GiB at sim) and `unshared` are not used. Rendered at `cmd/vms.go:43`.
**Correct.**

### 4.2 Datastore TYPE = real transport — **CLASSIFIER IS UNREACHABLE IN PRODUCTION (C-2)**

Full chain trace:

1. `datastores.go:194` — `datastoreVolumeUUID(ds.Info.GetDatastoreInfo().Url)`.
   `transport.go`-adjacent helper at `datastores.go:233-244` splits on prefix
   `"ds:///vmfs/volumes/"` then takes the segment up to the next `/`.
   **This is CORRECT** for a real VMFS URL `ds:///vmfs/volumes/<uuid>/`. The
   prior rung's URL-parse defect **does not recur here.** Verified by
   `TestDatastoreVolumeUUID` and by inspection.
2. `datastores.go:157-168` — `volumeExtents[vol.Uuid] = append(..., extent.DiskName)`.
   `HostVmfsVolume.Uuid` does equal the URL uuid; `HostScsiDiskPartition.DiskName`
   is documented as **`ScsiLun.canonicalName`** (`$M/vim25/types/types.go:45182-45186`).
3. `datastores.go:148-154` — `dst.luns = append(dst.luns, LunPath{Device: lun.Id, Path: p.Name})`
   where `lun` is a `HostMultipathInfoLogicalUnit`. **`.Id` is the LUN
   identifier, i.e. `ScsiLun.Uuid` — NOT the canonical name.**
4. `transport.go:69-93` — `ClassifyTransport` matches `want[lun.Device]`, i.e.
   it joins **canonicalName against ScsiLun.Uuid**.

The two are different namespaces. Proof from govmomi's *real ESXi capture*
(`$M/simulator/esx/host_storage_device_info.go`, header: "Capture method:
`govc object.collect -s -dump HostSystem:ha-host config.storageDevice`"), same
physical device:

```
ScsiLun   Key="key-vim.host.ScsiDisk-0000000000766d686261303a303a30"
          Uuid="0000000000766d686261303a303a30"
          CanonicalName="mpx.vmhba0:C0:T0:L0"        <- this is extent.DiskName
MultipathInfo.Lun  Id="0000000000766d686261303a303a30"  <- this is LunPath.Device
                   Lun="key-vim.host.ScsiDisk-0000000000766d686261303a303a30"
```

Reproduced live from my running v0.55.1 simulator (probe output):

```
MPATH  lun.Id="0000000000766d686261303a303a30"
       lun.Lun="key-vim.host.ScsiDisk-0000000000766d686261303a303a30"
       path.Name="vmhba0:C0:T0:L0"
SCSILUN key="key-vim.host.ScsiDisk-0000000000766d686261303a303a30"
       uuid="0000000000766d686261303a303a30"
       canonicalName="mpx.vmhba0:C0:T0:L0"
```

I drove the two joins through the shipped functions (temporary probe test,
since deleted):

```
PRODUCTION join (devices=canonicalName, luns.Device=lun.Id) -> "unknown"
TEST       join (devices=canonicalName, luns.Device=canonicalName) -> "FC"
```

**Verdict.** The classifier (`HbaProtocol`, `ParsePathName`, `ClassifyTransport`)
is genuinely branched for FC / iSCSI / NVMe, is well tested (10 table cases,
including dominance and tie handling), and is *not* dead code — it is called
from production at `datastores.go:227`. But its `devices` and `luns` arguments
are keyed on **different namespaces**, so `want[lun.Device]` can never be true
on a real system. `TYPE` will be `unknown` for every VMFS datastore on **every**
vCenter, not just at vcsim. Same failure *class* as the prior rung's headline —
correct-and-tested classifier, mangled input — in a **new form** (join-key
mismatch rather than URL parsing).

Correct join: `MultipathInfo.Lun[].Lun` (`key-vim.host.Scsi…`) → `ScsiLun.Key` →
`ScsiLun.CanonicalName`; or match `lun.Id` against `ScsiLun.Uuid`.

Secondary (masks the above at vcsim only): the code reads
`HostStorageSystem.fileSystemVolumeInfo`, which vcsim leaves **empty**
(`$M/simulator/host_storage_system.go:22-30` sets only `StorageDeviceInfo`).
`HostSystem.config.fileSystemVolume` *is* populated. Either source is defensible
on real vCenter; only the latter works at sim.

### 4.3 LACP is distributed-only — **GENUINELY WIRED**

`vswitches.go:236` sets DVPG LACP from `dvsLacpState(pgSetting, switchSetting)`
(`:309-341`), which inspects `LacpPolicy.Enable/Mode` then falls back to
`UplinkTeamingPolicy.Policy` containing `"lacp"`, defaulting to `disabled`.
Standard rows hardcode `LACPNA` at `:133` and `:143`. The distributed side is
correct and honest. **The standard side is unreachable (C-1)**, so the
"standard vSwitches report N/A" half of the requirement is never observable.

### 4.4 `--portgroup` for standard *and* distributed — **BOTH PATHS EXIST AND BOTH FIRE**

`portgroup.go:18-91` is deliberately type-agnostic: it collects `VM.network`
refs, resolves their `name` via `retrieveRaw(..., "Network", ...)`, and matches
by name. `Network`, `DistributedVirtualPortgroup` and `OpaqueNetwork` all
inherit `Network`, so one path serves both. I positively verified **both**:
distributed (`DC0_DVPG0` → 4 VMs) and standard (`VM Network` → `DC0_H0_VM0`
after I reconfigured that VM's NIC to a `VirtualEthernetCardNetworkBackingInfo`).
DoD 6 is **satisfied**.

**M-1 defect within it.** `wanted` is built only from networks that *some VM
references* (`:33-43`), so an existing port group with zero VMs yields
`len(wanted)==0` → `:63` returns `port group %q not found in inventory`, exit 1.
That message is **factually false** (`DC0_DVPG1`, `DVS0-DVUplinks-8` and
`VM Network` are all in the inventory), and an empty connected-VM set is a
legitimate answer, not an error. `portgroup_test.go:52-54` asserts an error for
`DC0_DVPG1`, which under its `Portgroup: 1` model genuinely does not exist — so
the test is honest but cannot catch this.

### 4.5 Dependencies — **PASS, with the RA-4 caveat**

`go.mod:6-10` direct: cobra, **pflag**, viper, govmomi. `go list -deps` confirms
every other package is transitive. Source imports are govmomi/*, cobra, viper,
pflag, stdlib. `text/tabwriter` used for all four tables
(`cmd/vms.go:36`, `cmd/datastores.go:36`, `cmd/vswitches.go:46,66`).
No third-party table/CLI/VMware library. `tools/vcsimserver` is a **separate
module** so `simulator` never enters the CLI's dependency set.

### 4.6 Viper precedence — **GENUINELY WIRED, verified end-to-end**

`config.go:33-52`: `SetEnvPrefix("VSPHERE")` → `AutomaticEnv()` → `SetDefault(...)`
→ `SetConfigFile/ReadInConfig`. `BindFlags` (`:56-67`) then `BindPFlag`s all five
keys. Order is correct: viper resolves flag(changed) > env > config > default.
`cmd/root.go:53-59` calls `NewViper(flagConfig)` then `BindFlags(v, cmd.Flags())`
— `Flags()` includes inherited persistent flags, so all five lookups succeed.

Verified on the **built binary**, not the test:

| Scenario | Result |
|---|---|
| env only | connects |
| env `url=https://bogus.invalid/sdk` + `--url <good>` | connects → **flag beats env** |
| config file `url=bogus` + env `url=<good>` | connects → **env beats file** |
| config file only (env unset) | connects → **file beats default** |
| nothing set | `Error: no vCenter URL configured: set --url, VSPHERE_URL, or url in the config file` |
| `VSPHERE_TIMEOUT=1ns` | `Error: connect to vCenter …: context deadline exceeded` → **timeout honored, no panic** |

---

## 5. Findings summary

| ID | Sev | Finding |
|---|---|---|
| **C-1** | Critical | **Standard vSwitches are never reported.** `vswitches.go:71-86` retrieves `HostSystem.network` and filters for `ref.Type == "HostVirtualSwitch"` / `"HostPortGroup"`. Those are **DataObjects, not managed objects** — they have no MOR and never appear in `HostSystem.network` (confirmed: not in `$M/vim25/mo/mo.go`; probe shows `host.network` = `Network` + `DistributedVirtualPortgroup` only). `switchRefs` is always empty, so `listStandardSwitches` always returns `nil`. Every helper below it (`standardUplinks`, `formatStandardVlan`, the `numPorts − numPortsAvailable` math, the `N/A` LACP) is unreachable. DoD 5's "covers both standard and distributed" is **not met**. `props.go:36-37` shows the model knew these types are "not modelled as mo structs" but concluded they are retrievable managed objects. Honest values it should have printed (per host): `vSwitch0 / standard / {Management Network, VM Network} / 0 / vmnic0 / N/A / 1536 / 6`. |
| **C-2** | Critical | **Transport classifier unreachable in production** (§4.2). Join key mismatch: `LunPath.Device` = `HostMultipathInfoLogicalUnit.Id` (= `ScsiLun.Uuid`) vs. `extent.DiskName` (= `ScsiLun.CanonicalName`). Proven with govmomi's own real-ESXi capture and by driving the shipped `ClassifyTransport` with both key sets. DoD 4 is not met on real vCenter. **Not fabrication** — it degrades to `unknown`, which is honest; it is a correctness defect. |
| **M-1** | Medium | `--portgroup` on an existing port group with zero connected VMs errors with a **false** message ("not found in inventory") and exit 1 instead of printing an empty table (§4.4). |
| **M-2** | Medium | `resolveTransport` **panics** on a datastore whose `info` property is absent: `datastores.go:194` dereferences `ds.Info.GetDatastoreInfo()` and `mo.Datastore.Info` is the interface `types.BaseDatastoreInfo` (`$M/vim25/mo/mo.go:235`). Proven: probe test logged `resolveTransport PANICS on nil Info: runtime error: invalid memory address or nil pointer dereference`. Spec `:26-27` forbids panics in normal flow; an inaccessible datastore is normal flow. |
| **M-3** | Medium | `fetchDVSUsedPorts` (`vswitches.go:266-268`) swallows any `FetchDVPorts` error and returns an empty map, so a permission/timeout failure renders as `USED 0` — indistinguishable from a true zero. An error path should not degrade to a number. |
| **L-1** | Low | `TestListSwitches` (`vswitches_test.go`) asserts only `len(switches) >= 1` and `sawDistributed`. It never asserts a standard switch is present, so it **cannot fail on C-1** despite the model containing 4 of them. |
| **L-2** | Low | `verify.sh:97` discovers the port group with `awk 'NR==2 {print $3}'`, which would truncate any name containing a space (e.g. `VM Network`). Latent only because C-1 keeps such names out of the output. |
| **L-3** | Low | Fleet-wide fetch: `loadHostStorage` pulls `storageDeviceInfo` for every host mounting any datastore (`datastores.go:88,116`). Batched (2 PropertyCollector calls, not N+1) but a heavy payload at fleet scale. |
| **N-1** | Nit | README:89-90 shows an example `vswitches` table containing a `standard` row that the tool can never produce. Illustrative, not fabricated output, but it documents a capability that does not exist. |

**Gates that pass, verified by me:** `go build ./...` ✅, `go vet ./...` ✅,
`gofmt -l .` clean ✅, `go test ./...` ✅ (3 packages ok, **zero skips** —
grepped, no `t.Skip` anywhere), `sh scripts/verify.sh` exit 0 ✅, all four
subcommand invocations exit 0 with well-formed aligned output ✅, sort order
correct on all three tables ✅, no panics observed in any live run ✅.

**No fabrication found.** Every printed value traces to a real API read. `unknown`,
`N/A`, `-` and `0` are used where the API gave nothing. The two Criticals are
*unreachability* defects, not dishonesty.

---

## 6. What I could NOT verify, and why

1. **Real FC / iSCSI / NVMe classification end-to-end.** No live vCenter and no
   SAN. vcsim's HBAs are ParallelScsi/Block with `storageProtocol=""`, and
   `fileSystemVolumeInfo` is empty. My C-2 conclusion rests on (a) govmomi's
   documented type semantics, (b) govmomi's own verbatim real-ESXi capture, and
   (c) driving the shipped function with those captured values — not on a live
   array. A live-vCenter run could in principle contradict it if some array
   populated `MultipathInfo.Lun[].Id` with a canonical name; I found no evidence
   of that and the type doc says otherwise.
2. **Real LACP `enabled` state.** vcsim has no `LacpGroupConfig`. `lacpStateOf`'s
   `enabled` branches (`vswitches.go:326-332`) are exercised by no test and by no
   simulator path. Their logic reads correctly but is unproven.
3. **Real UPLINKS values.** `UplinkPortPolicy` is nil at vcsim, so
   `dvsUplinks` never returns a name. `standardUplinks` is unreachable (C-1).
4. **Whether `HostVirtualSwitch` refs might appear on some real vCenter.** I
   verified they are absent from `mo`, from the simulator registry, and from a
   live `HostSystem.network`. The vSphere API defines them as DataObjects, so I
   am confident, but I have not observed a real vCenter.
5. **The README's `replace`-directive claim** about why `go run …/vcsim` fails
   (README:139-141). I confirmed the *outcome* (RA-1) but not the stated cause;
   the nested `go.mod` is not shipped in the module zip.
6. **Goroutine-leak freedom under cancellation.** `defer cleanup()` logs out and
   cancels; I saw no leak, but I ran no `-race` leak detector across cancelled
   in-flight calls.
7. **`insecure`/TLS behaviour.** My simulator serves plain HTTP
   (`tools/vcsimserver/main.go:42-43`), so the TLS-skip path was never exercised.

### Reproduction

Private tree:
`/private/tmp/claude-501/-Users-…/scratchpad/q8-groundtruth/qwen3.8-27b-q8_0/vsphere-inventory`
(byte-identical to the submission except a rebuilt `bin/`).
Independent probe: `…/scratchpad/q8-groundtruth/probe/` (own module, govmomi
v0.55.1). Temporary probe tests were removed; the submission tree is unmodified.
Simulators were started on `127.0.0.1:0` (OS-assigned ports 59289, 59888, 60472)
so they could not collide with the sibling passes.
