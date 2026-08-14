# vcsim Ground Truth at govmomi v0.50.0 (vs v0.55.1)

Purpose: establish what vcsim **actually returns** for every property the frozen
submission reads, so no auditor charges a finding against a value that is correct.
This document scores nothing.

Submission under test (read-only): `.../scratchpad/frozen/govmomi-inventory`, pinning
`github.com/vmware/govmomi v0.50.0` (`go.mod` line 8).

Scratch dir: `.../scratchpad/groundtruth/`

## 0. How the environment was stood up (and one discovery)

`go run github.com/vmware/govmomi/vcsim@v0.50.0` **does not work**, at either version:

```
$ go install github.com/vmware/govmomi/vcsim@v0.50.0
go: github.com/vmware/govmomi/vcsim@v0.50.0: module github.com/vmware/govmomi@v0.50.0
    found, but does not contain package github.com/vmware/govmomi/vcsim
$ go list -m -versions github.com/vmware/govmomi/vcsim
github.com/vmware/govmomi/vcsim            # <- no versions: no published tags
```

vcsim is a **nested module with a `replace` back to the parent**, so it is only
buildable from a source checkout:

```
$ cat src-govmomi-050/vcsim/go.mod
module github.com/vmware/govmomi/vcsim
go 1.23.0
replace github.com/vmware/govmomi => ../
```

So: `git clone --depth 1 --branch v0.50.0`, then `go build ./vcsim`. Verified the
binary's default inventory is exactly `simulator.VPX()` (`vcsim/main.go:57:
model := simulator.VPX()`), which is what the in-process probes use — the two are
the same inventory.

Artifacts: `p050/` and `p055/` (property dump), `p050b/`+`p055b/` (submission code-path
replicas), `p050c/`+`p055c/` (login + property-collector mechanics), `p050d/` (ESX vs VPX
model). Identical source compiled against each pinned version.

## 1. Property table — v0.50.0 vs v0.55.1

Values are for the **default `simulator.VPX()` / bare `vcsim` inventory**.
"Read?" = does the submission actually request and use it.

### VMs (`vms`) — props requested: `name`, `config.hardware.numCPU`, `config.hardware.memoryMB`, `storage.perDatastoreUsage`

| Property | v0.50.0 | v0.55.1 | Agree? |
|---|---|---|---|
| VM count / names | 4: `DC0_C0_RP0_VM0`, `DC0_C0_RP0_VM1`, `DC0_H0_VM0`, `DC0_H0_VM1` | same | ✅ |
| `config.hardware.numCPU` | `1` (all 4) | `1` | ✅ |
| `config.hardware.memoryMB` | **`32`** | **`32`** | ✅ |
| → rendered `RAM (GB)` = MB/1024 | **`0.0`** | **`0.0`** | ✅ |
| `storage.perDatastoreUsage[].committed` | **`0`** | **`234`** | ❌ **DIFFERS** |
| `storage.perDatastoreUsage[].uncommitted` | `10737418240` | `10737418006` | ❌ DIFFERS |
| `storage.perDatastoreUsage[].unshared` | `0` | `202` | ❌ DIFFERS |
| → rendered `STORAGE (GiB)` | **`0.0`** | **`0.0`** | ✅ (both round to 0.0) |
| `runtime.powerState` (not read) | `poweredOn` | same | ✅ |
| `config.guestId` / `guestFullName` (not read) | `otherGuest` | same | ✅ |
| `guest.guestFullName` / `hostName` / `ipAddress` (not read) | `""` (empty) | same | ✅ |
| `guest.toolsStatus` (not read) | `toolsNotInstalled` | same | ✅ |
| NIC concrete type | `*types.VirtualE1000` | same | ✅ |
| NIC backing | `*types.VirtualEthernetCardDistributedVirtualPortBackingInfo` (`PortgroupKey="dvportgroup-12"`) | same | ✅ |

### Datastores (`datastores`) — props: `name`, `summary.capacity`, `summary.freeSpace`, `summary.type`

| Property | v0.50.0 | v0.55.1 | Agree? |
|---|---|---|---|
| count / name | 1 × `LocalDS_0` | same | ✅ |
| `summary.type` | **`"OTHER"`** | **`"OTHER"`** | ✅ |
| `summary.capacity` | `4398046511104` (4096.0 GiB) | same | ✅ |
| `summary.freeSpace` | `4355096838144` (4056.0 GiB) | same | ✅ |
| `summary.uncommitted` | `0` | `0` | ✅ |
| `info` concrete type | `*types.LocalDatastoreInfo` (**not** Vmfs, **not** Nas) | same | ✅ |
| → rendered `TYPE` (`=="NFS"?"NFS":"unknown"`) | **`unknown`** | **`unknown`** | ✅ |
| → rendered `USED (GiB)` | **`40.0`** | **`40.0`** | ✅ |
| → rendered `AVAILABLE (GiB)` | **`4056.0`** | **`4056.0`** | ✅ |

### Host network (`vswitches`) — props: `name`, `config.network`

| Property | v0.50.0 | v0.55.1 | Agree? |
|---|---|---|---|
| Host count | 4 (`DC0_H0`, `DC0_C0_H0..H2`) | same | ✅ |
| vSwitch name | `vSwitch0` (1 per host) | same | ✅ |
| `vswitch.numPorts` | `1536` | `1536` | ✅ |
| `vswitch.numPortsAvailable` | `1530` | `1530` | ✅ |
| → rendered `PORTS` / `USED` | `1536` / **`6`** | same | ✅ |
| `vswitch.pnic` | **`["key-vim.host.PhysicalNic-vmnic0"]`** (key, not `vmnic0`) | same | ✅ |
| `vswitch.spec.policy.nicTeaming.policy` | `loadbalance_srcid`, activeNic `[vmnic0]`, standby `[]` | same | ✅ |
| Portgroups per host | 2: `VM Network`, `Management Network` | same | ✅ |
| `portgroup.spec.vlanId` | **`0`** for both | **`0`** | ✅ |
| Pnics present | `vmnic0`, `vmnic1` (driver `nvmxnet3`) | same | ✅ |
| LACP data on standard vSwitch | **none exists in the API** | none | ✅ |

### DVS

| Property | v0.50.0 | v0.55.1 | Agree? |
|---|---|---|---|
| DVS count / name | 1 × `DVS0` | same | ✅ |
| `config` concrete type | `*types.VMwareDVSConfigInfo` (assertion succeeds) | same | ✅ |
| `VMwareDVSConfigInfo.NumPorts` | **`0`** | **`0`** | ✅ |
| `LacpGroupConfig` | **len 0** → submission renders `disabled` | len 0 | ✅ |
| `LacpApiVersion` / `MaxMtu` | `""` / `0` | same | ✅ |
| `uplinkPortPolicy` | `<nil>` | `<nil>` | ✅ |
| DVPGs | `DC0_DVPG0` (key `dvportgroup-12`), `DVS0-DVUplinks-8` (key `dvportgroup-10`) | same | ✅ |
| `DC0_DVPG0` true VLAN | `VlanIdSpec(VlanId=0)` | same | ✅ |
| `DVS0-DVUplinks-8` true VLAN | **`TrunkVlanSpec([0–4094])`** | same | ✅ |
| DVPG `config.numPorts` | `1` each | same | ✅ |

### Storage transport / classifier inputs (`config.storageDevice`)

| Property | v0.50.0 | v0.55.1 | Agree? |
|---|---|---|---|
| `hostBusAdapter` count/types | 3: `*HostParallelScsiHba` (`vmhba0`, pvscsi), 2 × `*HostBlockHba` (`vmhba1`,`vmhba64`, vmkata) | same | ✅ |
| **FC / iSCSI / NVMe HBAs** | **none** | **none** | ✅ |
| `scsiLun` | 2: `*ScsiLun` cdrom `/vmfs/devices/cdrom/mpx.vmhba1:C0:T0:L0`; `*HostScsiDisk` `/vmfs/devices/disks/mpx.vmhba0:C0:T0:L0` | same | ✅ |
| `scsiTopology` | 3 adapters (targets 1/1/0) | same | ✅ |
| `ClassifyTransportFromDevice(deviceName)` on both LUNs | **`"unknown"`** | **`"unknown"`** | ✅ |

**Bottom line: exactly one genuine version difference across the whole probed surface** —
VM `committed`/`unshared` bytes. Full normalized delta (`version_delta.txt`):

```
<     perDatastoreUsage ds=datastore-59 committed=0   uncommitted=10737418240 unshared=0
>     perDatastoreUsage ds=datastore-59 committed=234 uncommitted=10737418006 unshared=202
<     sum(committed)=0   -> 0.0 GiB
>     sum(committed)=234 -> 0.0 GiB
```

Everything else is byte-identical. Determinism controls (same version, two runs,
after normalizing MAC/UUID/tmp-path/host-placement) both report `IDENTICAL`.

**Nondeterministic — never charge a diff on these:** VM MAC addresses, DVS UUID,
datastore tmp path, and **which cluster host owns which VM** (`host.Vm` length varies
run to run at the *same* version):

```
=== v0.50.0 run1 vs run2 ===
< HOST DC0_C0_H0  host.Vm len=0        > HOST DC0_C0_H0  host.Vm len=2
< HOST DC0_C0_H2  host.Vm len=2        > HOST DC0_C0_H2  host.Vm len=0
```

## 2. The binary produces NO inventory output at either version

Two blocking defects, both **version-independent** (identical at v0.50.0 and v0.55.1).

**(a) `-u/--url` and every other connection flag are unusable.** `config.BindFlags` is
called from `parseConfig` inside `RunE` — after cobra has already parsed argv:

```
$ ./bin/govmomi-inventory vms -u 'https://user:pass@127.0.0.1:8989/sdk' -k
Error: unknown shorthand flag: 'u' in -u
```

Only `VSPHERE_*` env vars can supply config.

**(b) With userinfo in the URL → double login.** `govmomi.NewClient` already logs in;
`connectToVCenter` then calls `c.Login` a second time:

```
$ env VSPHERE_URL='https://user:pass@127.0.0.1:8989/sdk' VSPHERE_INSECURE=true ./bin/govmomi-inventory vms
Error: authenticate to "...": ServerFaultCode: Login failure
```

Mechanism isolated (`p050c`), and identical at v0.55.1:

```
A) NewClient(url WITH userinfo) err=<nil>  sessionActive=true
A) second explicit Login() err=ServerFaultCode: Login failure   <-- submission does exactly this
B) NewClient(url WITHOUT userinfo) err=<nil> sessionActive=false
B) explicit Login() err=<nil>
```

**(c) Without userinfo → login succeeds, then `getDatacenter` fails.** `cmd/root.go`
passes the **ContainerView's own MoRef** to `pc.Retrieve` asking for `name` decoded into
`[]mo.Datacenter`; ContainerView has no `name` property:

```
$ env VSPHERE_URL='https://127.0.0.1:8989/sdk' VSPHERE_INSECURE=true ./bin/govmomi-inventory vms
Error: retrieve datacenters: InvalidProperty
```

Same at v0.55.1 (`./bin/govmomi-inventory-055` → `Error: retrieve datacenters: InvalidProperty`).

**Negative control — vcsim itself is healthy and the credentials are valid:**

```
$ env GOVC_URL='https://user:pass@127.0.0.1:8989/sdk' GOVC_INSECURE=true ./bin/govc050 ls /
/DC0
```

So `vms`, `datastores`, and `vswitches` all abort before any row is printed, on **both**
versions. Every rendered value in section 1 is a **reconstruction** from ground truth
(what the code *would* print if it reached the render), not captured CLI output.
The submission's own `go test ./...` passes (`config`, `format`, `inventory` all `ok`);
`inventory/vms_test.go` does exercise `simulator.ESX()`/`simulator.VPX()` — but through
`inventory.*` directly, bypassing the broken `cmd` layer.

## 3. Values that would look wrong but are CORRECT

1. **`RAM (GB) = 0.0`.** vcsim sets `memoryMB = 32` on every default VM, at both
   versions. `32/1024 = 0.03125 → "%.1f" → 0.0`. **Any non-zero RAM figure would be the
   fabrication.** (This is the exact trap from the prior run in this series.)
2. **`STORAGE (GiB) = 0.0`.** At v0.50.0 `committed` is literally `0`. At v0.55.1 it is
   `234` *bytes*, which still renders `0.0`. Correct at both.
3. **`VCPU = 1`.** vcsim's real `numCPU` is `1`. Not a stub.
4. **Datastore `TYPE = unknown`.** `summary.type` is `"OTHER"` — not `"VMFS"`, not
   `"NFS"`. The `info` object is `*LocalDatastoreInfo`. A classifier that only special-cases
   `NFS` genuinely lands on `unknown` here. **`VMFS` would be wrong.**
5. **`USED 40.0 / AVAILABLE 4056.0 GiB`.** Real: capacity 4 TiB, free 4056 GiB.
   Oddly round, but genuine.
6. **DVS `PORTS = 0`.** `VMwareDVSConfigInfo.NumPorts` really is `0` in vcsim. A non-zero
   DVS port count is the fabrication signature (cf. the qwen3.6 "6144" incident).
7. **DVS `LACP = disabled`.** `LacpGroupConfig` is genuinely empty.
8. **Standard-vSwitch `LACP = N/A`.** Standard vSwitches have no LACP in the vSphere API
   at all. `N/A` is the honest answer. (Bonding info *does* exist as
   `spec.policy.nicTeaming.policy = loadbalance_srcid`, activeNic `[vmnic0]` — reporting
   that would be *more* informative, but `N/A` for LACP specifically is not a fabrication.)
9. **VLAN `0` on `VM Network` and `Management Network`.** Real `spec.vlanId` is `0`.
10. **`UPLINKS = key-vim.host.PhysicalNic-vmnic0`.** Ugly, but `vswitch.pnic` really does
    hold pnic **keys**, not device names. Printing the raw value is faithful.
11. **`USED = 6` ports.** Real: `1536 − 1530`.
12. **Transport classifier → `unknown`.** vcsim's default host has only parallel-SCSI and
    block HBAs; no FC/iSCSI/NVMe exists to find.
13. **`--portgroup <anything>` returns zero rows.** True for all four portgroup names
    (see §4 for why) — but note this is a *code* limitation, not vcsim being empty.

## 4. Values a hardcoded constant would match BY ACCIDENT

These are where vcsim's degeneracy hides a stub. A hardcoder and an honest implementation
produce identical output — **the code must be read; the output cannot discriminate.**

| Hardcoded value | Matches truth? | Discriminator |
|---|---|---|
| `RAM = 0.0` | ✅ always | Only `memoryMB` fetch proves it. Every VM is 32 MB, so `0.0` is indistinguishable from a stub. |
| `VCPU = 1` | ✅ always | All 4 VMs are 1 vCPU. Also masked by the submission's `if vcpu == 0 { vcpu = 1 }` fallback, which makes a *missing* value render as `1`. |
| `STORAGE = 0.0` | ✅ at v0.50.0 (truly 0) | At v0.55.1 truth is 234 bytes, still `0.0`. A hardcoded 0.0 is undetectable at either version. |
| `TYPE = "unknown"` | ✅ always | Only one datastore, type `OTHER`. `classifyDatastoreType` returning a constant `"unknown"` is output-identical. |
| **DVS `VLAN = "0"`** | **⚠️ half** | **Submission hardcodes `vlan := "0"` for every DVPG.** True: `DC0_DVPG0` = 0 (accidental match), `DVS0-DVUplinks-8` = **trunk 0–4094** (wrong). The uplink PG is the discriminator. |
| DVS `PORTS = 0` | ✅ always | `NumPorts` is genuinely 0. |
| DVS `USED = 0` | ✅ (hardcoded) | Submission hardcodes `UsedPorts: 0`. vcsim exposes no DVS used-port count, so this is unfalsifiable from output. **Could not be determined** whether a correct value is even derivable. |
| DVS `LACP = "disabled"` | ✅ always | Group config genuinely empty. |
| Std `LACP = "N/A"` | ✅ always | Hardcoded, and also correct. |
| `SWITCH = "vSwitch0"` | ✅ always | One switch, same name on all 4 hosts. Fabricating `vSwitch0` is invisible. |
| `PORTS = 1536`, `USED = 6` | ✅ always | Identical on every host at both versions. |
| Portgroup names / VLAN `0` | ✅ always | `VM Network` + `Management Network`, vlan 0, on every host. |
| `--portgroup` → empty | ✅ always | A `return nil, nil` stub is output-identical to the real code here (see below). |

### Why `--portgroup` is unfalsifiable from output

Three independent reasons, each alone sufficient — measured, not inferred:

1. `getHosts` requests only `{"name", "config.network"}`, so `host.Vm` is never populated;
   `findVMsInStdPortGroup` iterates an empty slice:
   ```
   ### getHosts() with submission's exact props {name, config.network} -- is host.Vm populated?
   HOST DC0_C0_H0    len(host.Vm)=0  (property "vm" NOT requested)
   ... (all 4 hosts len=0)
   ```
2. Both matchers assert `dev.(*types.VirtualEthernetCard)`, but vcsim's NICs are concrete
   subclasses:
   ```
   device *types.VirtualE1000 : dev.(*types.VirtualEthernetCard)=false  dev.(types.BaseVirtualEthernetCard)=true
   ```
   The assertion can never succeed. (`BaseVirtualEthernetCard` is the correct interface.)
3. All 4 default VMs are DVS-backed, so the standard-portgroup path has no candidates
   regardless.

Measured result, identical at both versions:
```
portgroup "VM Network"         -> standard matches=0, DVS matches=0, TOTAL ROWS=0
portgroup "Management Network" -> standard matches=0, DVS matches=0, TOTAL ROWS=0
portgroup "DC0_DVPG0"          -> standard matches=0, DVS matches=0, TOTAL ROWS=0
portgroup "DVS0-DVUplinks-8"   -> standard matches=0, DVS matches=0, TOTAL ROWS=0
```
Ground truth for a *correct* implementation: **`DC0_DVPG0` should match all 4 VMs**
(their NIC backing carries `PortgroupKey="dvportgroup-12"`, which is `DC0_DVPG0`'s key).

### Bonus: the DVPG vlan fetch does not even populate the field

The submission requests `config.defaultPortConfig.vlan`, then ignores it. That prop path
returns a **nil** `DefaultPortConfig`; the parent must be requested:

```
props=[name config.defaultPortConfig.vlan] err=<nil>
   DVS0-DVUplinks-8     DefaultPortConfig=<nil> -> vlan=<DefaultPortConfig nil>
   DC0_DVPG0            DefaultPortConfig=<nil> -> vlan=<DefaultPortConfig nil>
props=[name config.defaultPortConfig] err=<nil>
   DVS0-DVUplinks-8     DefaultPortConfig=*types.VMwareDVSPortSetting -> vlan=TrunkVlanSpec([{{} 0 4094}])
   DC0_DVPG0            DefaultPortConfig=*types.VMwareDVSPortSetting -> vlan=VlanIdSpec(0)
```
Identical at v0.55.1. So even had the hardcoded `"0"` been removed, the requested property
set could not have supplied the real VLAN.

## 5. Dead code (states facts only)

- `inventory.ClassifyTransportFromDevice` — defined, never called (`grep` over all `*.go`
  finds only its own definition and doc comment). `ListDatastores` uses
  `classifyDatastoreType` instead.
- `format.HumanReadable` — referenced only by `format/format_test.go`.
- `DatastoreInfo.CapacityGiB` — assigned in `ListDatastores`, never printed.

## 6. `simulator.ESX()` model (used by `inventory/vms_test.go`)

Different defaults; do not cross-apply VPX numbers to ESX-model assertions.

```
===== simulator.ESX() =====
Datacenter=0 Cluster=0 ClusterHost=0 Host=0 Machine=2 Datastore=1 Portgroup=0
VM ha-host_VM0        numCPU=1 memoryMB=32 -> RAM 0.0 GB | committed=0 -> 0.0 GiB
VM ha-host_VM1        numCPU=1 memoryMB=32 -> RAM 0.0 GB | committed=0 -> 0.0 GiB
DS LocalDS_0      type="OTHER" capacity=1099511627776 free=1078036791296 -> used 20.0 GiB avail 1004.0 GiB
DVS count = 0
HOST localhost.localdomain vswitch="vSwitch0" numPorts=1536 avail=1530 used=6 pnic=[key-vim.host.PhysicalNic-vmnic0]
HOST localhost.localdomain pg="VM Network" vsw="vSwitch0" vlan=0
HOST localhost.localdomain pg="Management Network" vsw="vSwitch0" vlan=0
```

Key ESX-vs-VPX differences: 2 VMs (not 4), host `localhost.localdomain`, datastore
capacity 1 TiB → used 20.0 / avail 1004.0 GiB, and **no DVS at all**. `memoryMB=32`,
`numCPU=1`, vSwitch `1536/1530`, and vlan `0` are the same.

## 7. Explicitly not determined

- Whether a DVS "used ports" figure is derivable at all from vcsim (`DVPG.config.numPorts`
  is `1`, but that is configured size, not usage). The hardcoded `0` cannot be judged
  right or wrong from vcsim output.
- Behaviour against a real vCenter. Everything here is vcsim only.
- v0.52.0 (mentioned as used by one prior run) was not probed — only v0.50.0 and v0.55.1.
