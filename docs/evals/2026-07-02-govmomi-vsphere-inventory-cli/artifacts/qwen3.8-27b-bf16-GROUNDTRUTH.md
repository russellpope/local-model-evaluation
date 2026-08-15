# qwen3.8-27b-bf16 — vcsim GROUND TRUTH at govmomi v0.46.3

Date: 2026-08-14
Submission: `/Users/ldh/Projects/github.com/local-model-evaluation/qwen3.8-27b-bf16/vsphere-inventory` (read-only; unmodified — `git status --short` clean at end of run)
Pinned govmomi: **v0.46.3** (new to this repo; prior runs used v0.50.0 / v0.55.1 — nothing carries over)
Simulator model: `vcsim -l 127.0.0.1:8989 -vm 8 -ds 3 -pg 3` (the invocation the task spec recommends)

**Purpose of this document:** establish what vcsim v0.46.3 actually returns, so a later audit pass cannot charge the model for reporting a true value. Every value below was observed. Nothing is inferred from prior runs.

---

## 1. How vcsim was stood up at v0.46.3, and what failed

**Nothing failed.** At v0.46.3, `vcsim` is *not* a nested module — there is no `vcsim/go.mod`:

```
$ ls ~/go/pkg/mod/github.com/vmware/govmomi@v0.46.3/vcsim/
main.go   Makefile   README.md
$ ls ~/go/pkg/mod/github.com/vmware/govmomi@v0.46.3/vcsim/go.mod
ls: .../vcsim/go.mod: No such file or directory
```

So the route the spec prescribes works directly, from the submission's own module, with no replace directives and **no go.mod/go.sum drift**:

```
$ cd <extracted-submission> && GOFLAGS=-mod=mod go run github.com/vmware/govmomi/vcsim -h
Usage of .../vcsim:
  -ds int        Number of local datastores (default 1)
  -host int      Number of hosts per cluster (default 3)
  -pg int        Number of port groups (default 1)
  -vm int        Number of virtual machines per resource pool (default 2)
  ...
EXIT=0

$ go build -o $S/vcsim github.com/vmware/govmomi/vcsim   # BUILD OK, 36 MB
$ diff <orig go.mod> <extracted go.mod>   # no output
$ diff <orig go.sum> <extracted go.sum>   # no drift
```

Recorded for future passes: **the `go run github.com/vmware/govmomi/vcsim` failure mode does NOT apply at v0.46.3.** The spec's instructions are executable as written.

Environment: `go1.26.5 darwin/arm64`. Port 8989 confirmed free before start; vcsim killed and port confirmed free after. `llama-server` on :1234 untouched.

Reported `About`: `ApiVersion=6.5 ApiType=VirtualCenter Version=6.5.0`.

Inventory shape produced: **16 VMs** (8 × `DC0_H0_VM*` standalone + 8 × `DC0_C0_RP0_VM*` clustered), **4 hosts** (`DC0_H0`, `DC0_C0_H0/H1/H2`), **3 datastores**, **1 DVS**, **4 host network systems**.

Note `-vm 8` yields **16** VMs, not 8 — it is per-resource-pool. Any later pass expecting 8 rows is wrong.

---

## 2. Observed simulator values (raw evidence)

Probe programs (written outside the submission, in scratch, against govmomi v0.46.3):
`scratchpad/probe/main.go` (full enumeration), `scratchpad/probe2/main.go` (link-by-link transport traversal).

### 2.1 VMs — all 16 identical

```
-- VM DC0_H0_VM0 (vm-70)
   numCPU=1 memoryMB=32 guestFullName="otherGuest"
   memoryMB as bytes=33554432 -> GiB=0.0312
   summary.storage.Committed=0 Uncommitted=10737418240 Unshared=0
   summary.runtime.powerState="poweredOn"
   layoutEx.file count=7 totalSize=32
      key=0 type="nvram"          name="[LocalDS_0] DC0_H0_VM0.nvram"          size=0
      key=1 type="config"         name="[LocalDS_0] DC0_H0_VM0.vmx"            size=0
      key=2 type="diskExtent"     name="[LocalDS_0] disk1-flat.vmdk"           size=0
      key=3 type="diskDescriptor" name="[LocalDS_0] disk1.vmdk"                size=0
      key=4 type="log"            name="[LocalDS_0] vmware.log"                size=32
      key=5 type="diskExtent"     name="[LocalDS_0] DC0_H0_VM0/disk1-flat.vmdk" size=0
      key=6 type="diskDescriptor" name="[LocalDS_0] DC0_H0_VM0/disk1.vmdk"      size=0
   network refs (1): DistributedVirtualPortgroup:dvportgroup-12

$ grep 'numCPU='                 probe-out.txt | sort | uniq -c
  16    numCPU=1 memoryMB=32 guestFullName="otherGuest"
$ grep 'summary.storage.Committed' probe-out.txt | sort | uniq -c
  16    summary.storage.Committed=0 Uncommitted=10737418240 Unshared=0
$ grep 'layoutEx.file count'     probe-out.txt | sort | uniq -c
  16    layoutEx.file count=7 totalSize=32
$ grep 'network refs'            probe-out.txt | sort | uniq -c
  16    network refs (1): DistributedVirtualPortgroup:dvportgroup-12
```

Confirmed ground-truth facts:

| Fact | Observed |
|---|---|
| `memoryMB` | **32** → 33 554 432 B → 0.0312 GiB → renders **`0.0GiB`**. The prior-run heuristic **HOLDS** at v0.46.3. |
| `Summary.Storage` | **non-nil**, but `Committed` = **0** (not nil). `Uncommitted` = 10 GiB. |
| `LayoutEx.File` | 7 files, **total 32 bytes** (only `vmware.log` is non-zero). |
| `NumCPU` | 1 |
| power state | `poweredOn` (all 16) |
| guest OS | `otherGuest` |
| network attachment | **all 16 VMs on `dvportgroup-12` = `DC0_DVPG0`** (a *distributed* portgroup) |

Consequence for the submission's `committedBytes()`: `Summary.Storage != nil` but `Committed == 0`, so it is not `> 0`; it falls through to the `LayoutEx` sum = **32 bytes**, which is `> 0`, so it returns 32. `FormatBytes(32)` → **`0.0GiB`**. The `StorageUnknown` (-1) sentinel is *not* reached. Both paths render `0.0GiB` here, so the fallback is not observable at vcsim.

### 2.2 Datastores — LocalDatastoreInfo, type `OTHER` (NOT VMFS)

```
-- DS LocalDS_0 (datastore-63)
   summary.Type="OTHER" Capacity=10995116277760 FreeSpace=10823317585920 used=171798691840
   cap GiB=10240.00 free GiB=10080.00 used GiB=160.00
   info concrete type = *types.LocalDatastoreInfo
   info.Url="/var/folders/.../T/govcsim-DC0-LocalDS_0-1531423619"
-- DS LocalDS_1 (datastore-65)
   summary.Type="OTHER" Capacity=10995116277760 FreeSpace=10995116277760 used=0
-- DS LocalDS_2 (datastore-67)
   summary.Type="OTHER" Capacity=10995116277760 FreeSpace=10995116277760 used=0
```

- All three are `*types.LocalDatastoreInfo`. **There is no `VmfsDatastoreInfo`, no VMFS extent, no `DiskName` on any datastore object.**
- `summary.Type` is **`"OTHER"`** — neither `VMFS` nor `NFS`.
- Datastore `Url` is a filesystem path with **no `vmfs/` component** → the submission's `vmfsUUID()` correctly returns `""`.
- Capacity = 10 TiB exactly. `LocalDS_0` used = 160 GiB (16 VMs × 10 GiB uncommitted); `LocalDS_1`/`LocalDS_2` used = 0.

### 2.3 Host storage — HBAs, LUNs, and the `ScsiTopology.Interface.Adapter` question

All 4 hosts identical:

```
-- HOST DC0_H0 (host-25)
   HostBusAdapter count = 3
      *types.HostParallelScsiHba key="key-vim.host.ParallelScsiHba-vmhba0" device="vmhba0"
            model="PVSCSI SCSI Controller" driver="pvscsi" storageProtocol="" bus=3 status="unknown"
      *types.HostBlockHba key="key-vim.host.BlockHba-vmhba1"  device="vmhba1"
            model="PIIX4 for 430TX/440BX/MX IDE Controller" driver="vmkata" storageProtocol="" bus=0
      *types.HostBlockHba key="key-vim.host.BlockHba-vmhba64" device="vmhba64"
            model="PIIX4 for 430TX/440BX/MX IDE Controller" driver="vmkata" storageProtocol="" bus=0
   ScsiLun count = 2
      *types.ScsiLun      canonicalName="mpx.vmhba1:C0:T0:L0" deviceType="cdrom" lunType="cdrom"
      *types.HostScsiDisk canonicalName="mpx.vmhba0:C0:T0:L0" deviceType="disk"  lunType="disk"
   ScsiTopology.Adapter count = 3
      *** ScsiTopology.Interface.Adapter = "key-vim.host.ParallelScsiHba-vmhba0"
      *** ScsiTopology.Interface.Adapter = "key-vim.host.BlockHba-vmhba1"
      *** ScsiTopology.Interface.Adapter = "key-vim.host.BlockHba-vmhba64"
   fileSystemVolume.MountInfo count = 4
      *types.HostVmfsVolume name="datastore1" type="VMFS" uuid="deadbeef-...-cdef00000003" extents=1
            *** extent.DiskName="____simulated_volumes_____" partition=8
      *types.HostVmfsVolume name="OSDATA-deadbeef-...-cdef00000002" type="OTHER" extents=1
            *** extent.DiskName="____simulated_volumes_____" partition=7
      *types.HostVfatVolume name="BOOTBANK1" / "BOOTBANK2"
```

**ANSWER to the `ScsiTopology.Interface.Adapter` question: it is the HBA KEY** (`"key-vim.host.BlockHba-vmhba1"`), **not** the device name (`"vmhba1"`).

This matters: the submission indexes `hbaByName` by **both** `base.Device` **and** `base.Key` (`inventory/datastore.go:135-139`). That dual-index is what makes the LUN→adapter→HBA join succeed. A later pass must **not** charge this as redundant or wrong — indexing by key is load-bearing at v0.46.3.

Also note: **every HBA has `storageProtocol=""`** (empty). No FC, iSCSI, or NVMe HBA exists. Models/drivers are `PVSCSI/pvscsi` and `PIIX4 IDE/vmkata`.

### 2.4 Where the transport traversal actually breaks (link-by-link)

This is the decisive evidence that the transport classifier is **wired, not dead code**:

```
=== LINK 1: does any host VMFS volume name match a Datastore name? ===
Datastore names:        [LocalDS_0 LocalDS_1 LocalDS_2]
Host VMFS volume names: [datastore1 OSDATA-deadbeef-01234567-89ab-cdef00000002]
  RESULT: NO OVERLAP -> traversal stops at link 1

=== LINK 1b: do datastore URLs carry a vmfs/<uuid> path? ===
  LocalDS_0: url="/var/folders/.../govcsim-DC0-LocalDS_0-..."  (contains "vmfs/": false)

=== LINK 2: does any VMFS extent DiskName match a ScsiLun canonicalName? ===
  host DC0_H0: HostScsiDisk canonicalNames=[mpx.vmhba0:C0:T0:L0]
               vmfs extent DiskNames=[____simulated_volumes_____ ____simulated_volumes_____]
     NO MATCH for extent "____simulated_volumes_____" -> link 2 fails

=== LINK 3+4: for the real HostScsiDisk, does LUN->adapter->HBA resolve? ===
  host DC0_H0: lun "key-vim.host.ScsiDisk-0000...3a30" -> adapter "key-vim.host.ParallelScsiHba-vmhba0"
     link 4 OK -> HBA *types.HostParallelScsiHba model="PVSCSI SCSI Controller" driver="pvscsi" proto=""
     => traversal reaches ClassifyTransport; classifier input has empty StorageProtocol
```

**Links 3 and 4 WORK live.** The failure is entirely in links 1–2, and it is a *vcsim data* limitation, not a code defect:
- vcsim's `Datastore` objects (`LocalDS_*`) have no relationship to its host `HostVmfsVolume` objects (`datastore1`, `OSDATA-*`) — different names, and the datastore is Local, not VMFS.
- vcsim's VMFS extent `DiskName` is the literal placeholder `"____simulated_volumes_____"`, which matches no canonical name.

Therefore **`TYPE unknown` is the TRUE and only honest value** for all three datastores at v0.46.3. Additionally, even if links 1–2 were satisfied, every HBA has `storageProtocol=""` with a parallel-SCSI/IDE kind, so `ClassifyTransport` would still correctly return `unknown`.

### 2.5 Standard vSwitches

All 4 host network systems identical:

```
-- NetworkSystem hostnetworksystem-19
   Pnic count=0
   Vnic count=0
   Vswitch count=1
      vswitch key="" name="vSwitch0" numPorts=0 numPortsAvailable=0 mtu=0
         *** vswitch.Pnic (0): []string(nil)
         *** vswitch.Portgroup (1): []string{"VM Network"}
   Portgroup count=2
      *** pg key="key-vim.host.PortGroup-VM Network"         spec.Name="VM Network"         spec.VlanId=0 spec.VswitchName="vSwitch0"
      *** pg key="key-vim.host.PortGroup-Management Network" spec.Name="Management Network" spec.VlanId=0 spec.VswitchName="vSwitch0"
```

| Field | Observed truth |
|---|---|
| `NumPorts` / `NumPortsAvailable` | **0 / 0** → PORTS 0, USED 0 are TRUE |
| `vswitch.Pnic` | **nil / empty** → UPLINKS `unknown` is TRUE |
| `HostNetworkInfo.Pnic` / `.Vnic` | **both count 0** — no physical or vmkernel NICs modelled at all |
| **How `vswitch.Portgroup` references portgroups** | **BY NAME** (`"VM Network"`), *not* by key (`"key-vim.host.PortGroup-VM Network"`). The submission's `pgByName` fallback is load-bearing and correct. |
| VLAN | `spec.VlanId=0` |
| `vswitch.Key` | empty string `""` |

**Important asymmetry:** `vSwitch0.Portgroup` lists only `"VM Network"`, yet the host exposes **two** `HostPortGroup`s both carrying `Spec.VswitchName="vSwitch0"`. `"Management Network"` is therefore reachable via `Spec.VswitchName` but absent from `vswitch.Portgroup`. See §5 D6.

### 2.6 Distributed switch

```
container view Find() returned 1 refs:
   ref Type="DistributedVirtualSwitch" Value="dvs-8"

*** view.Retrieve(kind=["DistributedVirtualSwitch"])        -> err=<nil> count=1
*** view.Retrieve(kind=["VmwareDistributedVirtualSwitch"])  -> err=<nil> count=0

-- DVS DVS0 (Type=DistributedVirtualSwitch Value=dvs-8)
   config concrete type = *types.VMwareDVSConfigInfo
   *** cfg.NumPorts=0 cfg.MaxPorts=0 cfg.NumStandalonePorts=0
   cfg.UplinkPortgroup count=0
   cfg.UplinkPortPolicy = NIL
   *** VMwareDVSConfigInfo.LacpGroupConfig count=0 LacpApiVersion=""
   portgroup refs count=4
      DVPG name="DVS0-DVUplinks-8" ref=dvportgroup-10 numPorts=1 type="earlyBinding"
         Vlan = *types.VmwareDistributedVirtualSwitchTrunkVlanSpec VlanId:[{Start:0, End:4094}]
      DVPG name="DC0_DVPG0" ref=dvportgroup-12 numPorts=1 -> VlanIdSpec VlanId:0
      DVPG name="DC0_DVPG1" ref=dvportgroup-14 numPorts=1 -> VlanIdSpec VlanId:0
      DVPG name="DC0_DVPG2" ref=dvportgroup-16 numPorts=1 -> VlanIdSpec VlanId:0
```

| Field | Observed truth |
|---|---|
| DVS managed-object **Type** | **`DistributedVirtualSwitch`**, *not* `VmwareDistributedVirtualSwitch`. The submission's `Retrieve(kind=["DistributedVirtualSwitch"])` filter therefore **matches (count=1)**. Verified by running both filters. |
| `Config` concrete type | **IS `*types.VMwareDVSConfigInfo`** → the submission's LACP branch is entered (not the `N/A` else-branch) |
| `LacpGroupConfig` | **empty (count 0)** → honest value is **`disabled`** |
| `cfg.NumPorts` / `cfg.MaxPorts` | **0 / 0** → PORTS 0, USED 0 are TRUE. **The prior-run heuristic HOLDS: 0 is the true port count; a NON-ZERO value would be the fabrication signature.** |
| `cfg.UplinkPortgroup` | **empty (count 0)** → UPLINKS `unknown` is TRUE for the field read |
| `cfg.UplinkPortPolicy` | **NIL** — the other uplink-name source is also empty |
| DVPGs | 4: `DVS0-DVUplinks-8` (trunk **0-4094**), `DC0_DVPG0/1/2` (VLAN **0**) |

Caveat recorded: the uplink portgroup object `DVS0-DVUplinks-8` **does exist** and is reachable via `dvs.Portgroup`, even though `cfg.UplinkPortgroup` is empty. So uplink information is arguably present in the inventory; `unknown` is honest w.r.t. the field the submission reads, but is an incompleteness rather than the only possible answer.

### 2.7 Network objects and the `Network.vm` backref

```
network object count = 5
   net Type=Network                     Value=network-6      Name="VM Network"       vmCount=0
   net Type=DistributedVirtualPortgroup Value=dvportgroup-10 Name="DVS0-DVUplinks-8" vmCount=0
   net Type=DistributedVirtualPortgroup Value=dvportgroup-12 Name="DC0_DVPG0"        vmCount=0
   net Type=DistributedVirtualPortgroup Value=dvportgroup-14 Name="DC0_DVPG1"        vmCount=0
   net Type=DistributedVirtualPortgroup Value=dvportgroup-16 Name="DC0_DVPG2"        vmCount=0
```

**Trap for auditors:** vcsim does **not** populate the `Network.vm` backref — every network reports `vmCount=0`, including `DC0_DVPG0` which genuinely has all 16 VMs attached. Any implementation reading `Network.vm` would return an empty list. The submission correctly iterates VMs and reads `vm.Network` instead, which is why its `--portgroup` lookup returns 16 VMs.

Also note `"Management Network"` has **no `Network` managed object** — only `"VM Network"` (network-6) exists as a standard Network.

---

## 3. Submission output per subcommand (real runs against this vcsim)

Built from a clean `git archive` extraction (not the model's prebuilt binary):

```
$ cd /Users/ldh/Projects/github.com/local-model-evaluation && \
  git archive $(git write-tree) qwen3.8-27b-bf16/vsphere-inventory | tar -x -C $S/sub2
$ go build ./...   # exit 0
$ go vet ./...     # exit 0
$ gofmt -l .       # no output
$ go test ./...
?   	vsphere-inventory	[no test files]
?   	vsphere-inventory/cmd	[no test files]
ok  	vsphere-inventory/config	0.241s
ok  	vsphere-inventory/inventory	1.256s
   (12 tests, 0 failures, 0 skips; no `t.Skip` anywhere in *_test.go)
```

### `vms`

```
NAME            VCPU  RAM     STORAGE
DC0_C0_RP0_VM0  1     0.0GiB  0.0GiB
... (16 rows, sorted by name; DC0_C0_RP0_VM0-7 then DC0_H0_VM0-7)
DC0_H0_VM7      1     0.0GiB  0.0GiB
EXIT=0
```

### `datastores`

```
NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  160.0GiB  9.8TiB
LocalDS_1  unknown  0.0GiB    10.0TiB
LocalDS_2  unknown  0.0GiB    10.0TiB
EXIT=0
```

### `vswitches`

```
SWITCH    SWITCH TYPE  PORTGROUP         VLAN    UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0         0       unknown  disabled  0      0
DVS0      distributed  DC0_DVPG1         0       unknown  disabled  0      0
DVS0      distributed  DC0_DVPG2         0       unknown  disabled  0      0
DVS0      distributed  DVS0-DVUplinks-8  0-4094  unknown  disabled  0      0
vSwitch0  standard     VM Network        0       unknown  N/A       0      0
vSwitch0  standard     VM Network        0       unknown  N/A       0      0
vSwitch0  standard     VM Network        0       unknown  N/A       0      0
vSwitch0  standard     VM Network        0       unknown  N/A       0      0
EXIT=0
```

### `vswitches --portgroup ...`

```
$ vswitches --portgroup DC0_DVPG0        -> 16 VM rows, EXIT=0    (distributed pg, correct)
$ vswitches --portgroup "VM Network"     -> header only, 0 rows, EXIT=0   (standard pg; 0 VMs is TRUE)
$ vswitches --portgroup DC0_DVPG1        -> header only, 0 rows, EXIT=0   (TRUE: no VM attached)
$ vswitches --portgroup "Management Network"
     error: no port group or network named "Management Network" found in the inventory   EXIT=1
$ vswitches --portgroup zzz
     error: no port group or network named "zzz" found in the inventory                  EXIT=1
```

### Error / robustness paths

```
$ vsphere-inventory vms --username '' --password ''
warning: logout failed: ServerFaultCode: NotAuthenticated
error: list virtual machines: create container view: ServerFaultCode: NotAuthenticated

$ vsphere-inventory vms --timeout 1ns
error: connect to 127.0.0.1:8989 as "user": Post "https://127.0.0.1:8989/sdk": context deadline exceeded (...)

$ VSPHERE_INSECURE=false vsphere-inventory vms
error: connect to 127.0.0.1:8989 as "user": Post "https://127.0.0.1:8989/sdk": tls: failed to verify certificate: x509: certificate signed by unknown authority (...)
```

### `make verify`

Runs `go vet` + `go test`, builds vcsim from the pinned module, starts it with `-vm 8 -ds 3 -pg 3 -username user -password pass`, waits for readiness, runs all four invocations discovering the portgroup name from its own output (`using port group: DC0_DVPG0`), and tears vcsim down via an EXIT trap. **Result: `==> verify: OK`, exit 0.** No panics, no goroutine complaints, port released.

---

## 4. Per-field TRUE / FALSE / UNVERIFIABLE

| Subcommand | Field | Printed | Verdict | Basis |
|---|---|---|---|---|
| `vms` | NAME (16 rows, sorted) | `DC0_C0_RP0_VM0`…`DC0_H0_VM7` | **TRUE** | 16 VM refs observed; sort order correct |
| `vms` | VCPU | `1` | **TRUE** | `config.hardware.numCPU=1` |
| `vms` | RAM | `0.0GiB` | **TRUE** | `memoryMB=32` → 0.0312 GiB → rounds to 0.0 |
| `vms` | STORAGE | `0.0GiB` | **TRUE** | `Committed=0`; `LayoutEx` sum = 32 B → 0.0 GiB. Derived from real API data, not hardcoded |
| `datastores` | NAME | `LocalDS_0/1/2` | **TRUE** | observed |
| `datastores` | TYPE | `unknown` | **TRUE** | traversal links 1–2 unsatisfiable at vcsim; all HBAs have empty `storageProtocol`. `unknown` is the only honest value |
| `datastores` | USED | `160.0GiB` / `0.0GiB` / `0.0GiB` | **TRUE** | `Capacity−FreeSpace` = 171798691840 (=160 GiB) / 0 / 0 |
| `datastores` | AVAILABLE | `9.8TiB` / `10.0TiB` / `10.0TiB` | **TRUE** | `FreeSpace` = 10823317585920 (9.84 TiB) / 10995116277760 (10 TiB) |
| `vswitches` | SWITCH `DVS0` | `DVS0` | **TRUE** | observed |
| `vswitches` | SWITCH `vSwitch0` ×4 | 4 identical rows | **TRUE but ambiguous** | 4 hosts each own a `vSwitch0`; spec has no HOST column (see D6) |
| `vswitches` | SWITCH TYPE | `distributed` / `standard` | **TRUE** | observed |
| `vswitches` | PORTGROUP (DVS) | `DC0_DVPG0/1/2`, `DVS0-DVUplinks-8` | **TRUE** | all 4 DVPGs observed on `dvs.Portgroup` |
| `vswitches` | PORTGROUP (std) | `VM Network` | **TRUE but incomplete** | `vswitch.Portgroup=["VM Network"]`; `Management Network` exists on the host but is not in that list (see D6) |
| `vswitches` | VLAN (DVPGs) | `0` | **TRUE** | `VmwareDistributedVirtualSwitchVlanIdSpec{VlanId:0}` |
| `vswitches` | VLAN (uplink pg) | `0-4094` | **TRUE** | `TrunkVlanSpec{VlanId:[{Start:0,End:4094}]}` |
| `vswitches` | VLAN (std) | `0` | **TRUE** | `pg.Spec.VlanId=0` |
| `vswitches` | UPLINKS (DVS) | `unknown` | **TRUE (field-faithful), incomplete** | `cfg.UplinkPortgroup` empty and `UplinkPortPolicy` nil; but `DVS0-DVUplinks-8` is reachable via `dvs.Portgroup` |
| `vswitches` | UPLINKS (std) | `unknown` | **TRUE** | `vswitch.Pnic` is nil; `HostNetworkInfo.Pnic` count 0 |
| `vswitches` | LACP (DVS) | `disabled` | **TRUE** | config IS `*VMwareDVSConfigInfo` with `LacpGroupConfig` count 0. **Not `N/A` at this version** |
| `vswitches` | LACP (std) | `N/A` | **TRUE** | spec-mandated for standard switches |
| `vswitches` | PORTS (DVS) | `0` | **TRUE** | `cfg.MaxPorts=0`, `cfg.NumPorts=0`. Non-zero would be the fabrication signature |
| `vswitches` | USED (DVS) | `0` | **TRUE** | `cfg.NumPorts=0` |
| `vswitches` | PORTS/USED (std) | `0` / `0` | **TRUE** | `NumPorts=0`, `NumPortsAvailable=0` |
| `--portgroup DC0_DVPG0` | 16 VM rows | 16 rows | **TRUE** | all 16 VMs carry `network=[dvportgroup-12]` |
| `--portgroup "VM Network"` | 0 rows | header only | **TRUE** | no VM attached to `network-6` |
| `--portgroup "Management Network"` | error | exit 1 | **TRUE** | no `Network` managed object of that name exists |
| — | **UPLINKS device-name resolution (live vCenter)** | n/a | **UNVERIFIABLE at vcsim** | latent defect, see below |

### The one latent defect ground truth can confirm statically but not observe

`inventory/switch.go:89-113` builds `vnicName` from `net.Vnic` (`HostVirtualNic` — vmkernel adapters) and then looks up `sw.Pnic` entries in it. Per govmomi v0.46.3 `vim25/types/types.go`:

- `HostVirtualSwitch.Pnic []string` — *"The set of physical network adapters associated with this bridge"* (line 46605) → these are **`PhysicalNic.Key`** values.
- `HostNetworkInfo.Pnic []PhysicalNic` (line 40463) is the correct lookup source; `HostNetworkInfo.Vnic` (line 40467) is a **different key namespace**.

So `vnicName[pnic]` can never hit; on a live vCenter UPLINKS would render raw keys like `key-vim.host.PhysicalNic-vmnic0` instead of `vmnic0`. **At vcsim this is completely unobservable** — `Pnic` count 0, `Vnic` count 0, `sw.Pnic` nil → the code falls to `joinOrUnknown([])` → `unknown`, which is correct. Verdict: **UNVERIFIABLE at vcsim; statically demonstrable defect.** It cannot be charged as fabrication and does not affect any observed output.

### README accuracy

The README's `datastores` and `vswitches` sample blocks (lines 84-93) **match real output exactly**. The `vms` sample block (lines 76-79) does **not**:

```
NAME            VCPU  RAM    STORAGE
DC0_C0_RP0_VM0  1     2.0GiB 5.0GiB      <- actual: 1  0.0GiB  0.0GiB
DC0_H0_VM0      2     4.0GiB 20.0GiB     <- actual: 1  0.0GiB  0.0GiB
```

Verdict: **FALSE as a representation of a vcsim run.** These are idealized/illustrative values, not observed output. Whether that counts as a documentation defect or a "do not fabricate data" violation is an audit judgement — but the values are demonstrably not what the tool produces, and the surrounding README text frames the section simply as "Output".

---

## 5. Requirements defects (requirements-attack step)

Each is charged to the **instrument**, not the model. None is silently resolved.

### D1 — Criterion 5's `used = total − available` is undefined for distributed switches
**Contradiction.** The spec's DoD 5 states "used ports = total − available". `HostVirtualSwitch` has both `NumPorts` and `NumPortsAvailable`, so the formula is well-defined for **standard** switches. `DVSConfigInfo` has **`NumPorts` and `MaxPorts` and no "available" field at all** — `NumPorts` is the count of ports that exist, `MaxPorts` the cap. There is nothing to subtract.
**Proposed resolution:** scope the subtraction rule to standard vSwitches only. For a DVS, specify PORTS = `MaxPorts` (falling back to `NumPorts` when `MaxPorts` is 0) and USED = `NumPorts`. Do not charge a model for declining to subtract on a DVS. The submission's mapping (`total=MaxPorts`, `used=NumPorts`, clamped so `total >= used`) is a reasonable reading.

### D2 — `datastores` TYPE enumeration contradicts the spec's own tolerance for `unknown`
**Contradiction.** The `datastores` column spec says TYPE is "**one of** `FC`, `iSCSI`, `NVMe`, or `NFS`" — a closed set of four. But the unit-test criterion (test 2) requires TYPE ∈ `FC`/`iSCSI`/`NVMe`/`NFS`/**`unknown`**, and the "Simulator fidelity" note explicitly blesses `unknown`. At vcsim the true value is `unknown` for **all three** datastores, so the column definition is unsatisfiable honestly.
**Proposed resolution:** amend the TYPE column definition to include `unknown` as the explicit honest-degradation value, and state that the four-value enumeration is the live-vCenter contract only. A later pass must not treat `unknown` as a miss.

### D3 — Criterion 4's premise has no instance at vcsim
**Impossible-to-satisfy requirement.** DoD 4 ("`datastores` reports real transport, not filesystem type") is framed around "a single **VMFS** datastore may be backed by FC, iSCSI, or NVMe". At v0.46.3 vcsim produces **zero** VMFS datastores — all are `LocalDatastoreInfo` with `summary.Type="OTHER"` — and the VMFS *volumes* that do exist on hosts (`datastore1`, `OSDATA-*`) correspond to no `Datastore` object, with the placeholder extent `DiskName="____simulated_volumes_____"`. No HBA carries a non-empty `storageProtocol`.
**Proposed resolution:** state explicitly that criterion 4's end-to-end traversal is **unexercisable** at vcsim and is proven only by (a) the pure `ClassifyTransport` table test and (b) static confirmation that the production path calls it. This ground-truth pass supplies (b): links 3–4 (LUN→adapter→HBA) were **observed working live**; only links 1–2 are data-blocked. The classifier is not dead code.

### D4 — Test criterion 1's "storage ≥ 0" collides with the spec's own "degrade to unknown" rule
**Contradiction.** Test requirement 1 demands each VM result have "storage ≥ 0". The spec elsewhere mandates degrading unmodelled values to `unknown`. A model that represents unknown storage as a negative sentinel (as this submission does — `StorageUnknown = -1`) would **fail** its own required assertion whenever `Committed=0` and `LayoutEx` is empty. Here it survives only because `LayoutEx` happens to sum to 32 bytes.
**Proposed resolution:** restate as "storage ≥ 0 **or** an explicit unknown sentinel/marker", and say which representation is expected. Related trap, lower severity: the same criterion demands "RAM > 0", which is satisfiable in bytes (33 554 432 > 0) but would be impossible if asserted on the displayed GiB value (`0.0`). Specify that these assertions apply to the raw typed fields, not the rendered strings.

### D5 — Criterion 6's standard-port-group branch cannot be positively confirmed at vcsim
**Impossible to verify honestly.** DoD 6 requires `--portgroup` to work for standard **and** distributed port groups. At vcsim **all 16 VMs attach to the distributed `DC0_DVPG0`**; the only standard `Network` object, `"VM Network"`, has zero VMs. The standard branch can be *exercised* (it returns exit 0 with an empty table) but never *positively confirmed*. Compounding it, `"Management Network"` exists as a `HostPortGroup` but has **no `Network` managed object**, so looking it up is an honest error, not a bug.
**Proposed resolution:** require the standard-PG path to be proven by the **unit test** (`simulator` model configured to attach a VM to a standard portgroup) or on live vCenter, and state that an empty result for `--portgroup "VM Network"` against vcsim is the expected true outcome. Do not charge a model for it, and do not require `"Management Network"` to resolve.

### D6 — The `vswitches` table cannot disambiguate per-host standard vSwitches
**Ambiguity / under-specification.** vcsim creates 4 hosts, each with its own `vSwitch0` carrying `VM Network`. The spec's column list (SWITCH, SWITCH TYPE, PORTGROUP, VLAN, UPLINKS, LACP, PORTS, USED) has **no HOST column**, so a faithful implementation must emit 4 byte-identical, indistinguishable rows — which is exactly what the submission does. Deduplicating would be equally defensible and equally unfalsifiable from the spec.
**Secondary issue in the same area:** `vswitch.Portgroup` lists only `"VM Network"`, while the host's `HostPortGroup` array also contains `"Management Network"` with `Spec.VswitchName="vSwitch0"`. Reading `vswitch.Portgroup` (as the submission does) silently drops a portgroup the simulator does model; reading `Spec.VswitchName` would surface both. The spec does not say which association is authoritative.
**Proposed resolution:** either add a HOST column, or state explicitly that standard vSwitch rows are deduplicated by (switch, portgroup, vlan); and name `HostPortGroup.Spec.VswitchName` as the authoritative switch↔portgroup association. Until then, do not charge either choice, and treat the missing `Management Network` row as incompleteness, not fabrication.

### D7 — (No defect) The spec's vcsim launch instruction is correct at this version
Recorded because it is a known failure mode elsewhere: at v0.46.3 `vcsim` is **not** a nested module, and `go run github.com/vmware/govmomi/vcsim` works verbatim from the submission's own module with no drift. The spec's "Self-verification loop" is executable exactly as written. No resolution needed.

### D8 — (Clarification, not a defect) `LACP disabled` is correct here, not `N/A`
Prior runs in this repo recorded "`LACP N/A` is correct". **That heuristic does not fully carry to v0.46.3.** vcsim's DVS config *is* `*types.VMwareDVSConfigInfo` with an empty `LacpGroupConfig`, so the honest DVS value is **`disabled`**; `N/A` remains correct for standard vSwitches. Both values in the submission's output are truthful. A later pass must not charge `disabled` as an error.

---

## 6. Summary of heuristics carried in from prior runs

| Prior-run heuristic | Holds at v0.46.3? | Evidence |
|---|---|---|
| VM `RAM 0.0 GiB` (memoryMB=32) | **YES** | `memoryMB=32` on all 16 VMs |
| DVS `PORTS 0` is TRUE; non-zero = fabrication | **YES** | `cfg.NumPorts=0 cfg.MaxPorts=0` |
| `TYPE unknown` for storage transport | **YES** | links 1–2 data-blocked; all HBAs `storageProtocol=""` |
| `LACP N/A` | **PARTIALLY** | `N/A` correct for **standard** only; DVS honest value is **`disabled`** (config is `VMwareDVSConfigInfo`, `LacpGroupConfig` empty) |

New ground truth specific to v0.46.3, not present in prior runs:

- `ScsiTopology.Interface.Adapter` = **HBA KEY**, not device name.
- `vswitch.Portgroup` references portgroups **BY NAME**, not by key.
- DVS managed-object type is **`DistributedVirtualSwitch`**, so a `Retrieve(kind=["DistributedVirtualSwitch"])` filter matches.
- `Network.vm` backref is **never populated** (all networks report `vmCount=0`) — `--portgroup` must be implemented by iterating VMs.
- `-vm 8` produces **16** VMs (per resource pool), not 8.
- Datastores are **`LocalDatastoreInfo`, `summary.Type="OTHER"`** — no VMFS datastore exists.

---

## Artifacts

Probe sources and raw output (scratch, outside the repo and outside the submission):

- `scratchpad/probe/main.go` + `scratchpad/probe-out.txt` (468 lines, full enumeration)
- `scratchpad/probe2/main.go` (link-by-link transport traversal)
- `scratchpad/sub2/qwen3.8-27b-bf16/vsphere-inventory` (clean `git archive` extraction used for all binary runs)

Teardown confirmed: vcsim killed, port 8989 free, `git status --short` clean, `llama-server` on :1234 untouched.
