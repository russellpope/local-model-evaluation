# Round 3 — you compiled and it runs. Five items to a passing grade.

Round 2 was a big jump: it compiles, tests are green, and `vswitches` /
`datastores` / `vms` / distributed `--portgroup` all run against the simulator with
real data. Score 18/30. Five specific gaps remain. Fix them and you pass.

## Rule #0 still applies

After every change: `go build ./...` and `go test ./...`, both exit 0, and run all
three subcommands **plus** `vswitches --portgroup <name>` against the simulator
before declaring done. Do not delete or empty a test to make it pass — fix it.

---

## 1. Restore the two tests you deleted (highest priority — this is criterion 8)

`pkg/inventory/switches_test.go` currently has an **empty** `TestProcessSwitches`
body and the port-group test is gone. That is the one thing keeping integrity down.
Write real tests using the **correct** govmomi types (this is why round 1's version
didn't compile — these are the real names):

- Mock a DVS: `mo.DistributedVirtualSwitch{ Self: types.ManagedObjectReference{Type:"VmwareDistributedVirtualSwitch", Value:"dvs-1"}, Name:"DVS-1", Config: &types.VMwareDVSConfigInfo{ LacpGroupConfig: []types.VMwareDvsLacpGroupConfig{{Mode:"active"}} } }`. Set the ref via the `Self` field — there is no `mo.NewReference`.
- Mock a DVPG: `mo.DistributedVirtualPortgroup{ Name:"DVPG-1", Config: types.DVPortgroupConfigInfo{ DistributedVirtualSwitch: &types.ManagedObjectReference{Value:"dvs-1"}, DefaultPortConfig: &types.VMwareDVSPortSetting{ Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{VlanId:10} } } }`.
- Mock a host: `mo.HostSystem{ Config: &types.HostConfigInfo{ Network: &types.HostNetworkInfo{ Vswitch: []types.HostVirtualSwitch{{Name:"vSwitch0", NumPorts:128, NumPortsAvailable:100}}, Portgroup: []types.HostPortGroup{{ Spec: types.HostPortGroupSpec{Name:"VM Network", VlanId:0, VswitchName:"vSwitch0"} }} } } }`.
- Assert: `processSwitches(...)` returns the expected rows — DVS row with LACP `Enabled`, VLAN `10`, and the port counts you pass in; standard row with switch `vSwitch0`, portgroup `VM Network`.
- Add a port-group→VMs test: build `[]mo.VirtualMachine` with `Network: []types.ManagedObjectReference{...}` and assert the resolver returns exactly the VMs whose `Network` contains the target ref.

## 2. Standard-switch port counts (stop hardcoding 0)

In `processSwitches`, the standard-switch branch sets `Ports: 0, Used: 0`. Use the
real values from the `HostVirtualSwitch` you're already iterating:
`Ports: vsw.NumPorts`, `Used: vsw.NumPorts - vsw.NumPortsAvailable`.

## 3. Standard `--portgroup` (it silently returns empty today)

`GetVMsInPortgroup` only resolves the name against `DistributedVirtualPortgroup`, so
standard port groups return nothing (and there's a comment admitting it). Resolve
against the general **`Network`** container instead — it contains *both* standard
networks and distributed port groups (a DVPG is a Network):

- Retrieve `mo.Network` (`[]string{"name"}`), find the one whose `Name == pgName`, take its `.Reference()`.
- Retrieve VMs with the `network` property and keep any VM whose `v.Network` slice contains that ref.

This one change makes `--portgroup` work for standard *and* distributed, and you can
delete the "stick to the requirement" comment.

## 4. Wire the transport classifier feeder (the hard one — read the caveat)

Your `classifyTransport` logic is correct and table-tested, but `AdapterInfo` is
never populated, so it returns `unknown` for every VMFS datastore. Wire the feeder:

- Retrieve hosts with `config.storageDevice` → `h.Config.StorageDevice` (`*types.HostStorageDeviceInfo`), and datastores with the `host` property (which hosts mount each datastore: `ds.Host[].Key`).
- For a VMFS datastore, take `(*types.VmfsDatastoreInfo).Vmfs.Extent[].DiskName`.
- In the mounting host's `StorageDevice`: find the `ScsiLun` whose `GetScsiLun().CanonicalName` matches that DiskName → get its `.Key`. Then in `MultipathInfo.Lun`, find the entry whose `.Lun == key`, take a `.Path[].Adapter` (the HBA key). Then find that adapter in `HostBusAdapter` via `hba.GetHostHostBusAdapter().Key`.
- Classify by the adapter's **concrete type**: `*types.HostFibreChannelHba` → FC, `*types.HostInternetScsiHba` → iSCSI, an NVMe-over-fabrics HBA → NVMe. (Feed the type name / driver / model string into your existing `classifyAdapter`.)

**Caveat — do not chase this against the simulator.** vcsim does not model HBA
topology, so even with the feeder perfectly wired, `datastores` will still show
`unknown` on the sim. That is the **correct** result — the spec says so. Your proof
that the logic works is the **table test** (FC/iSCSI/NVMe descriptors → specific
protocol), which you already have. Do NOT fabricate a transport to make the sim show
FC/iSCSI, and do NOT conclude the feeder is "broken" because the sim shows `unknown`.
One gotcha for your keyword matcher: the iSCSI HBA type is spelled `InternetScsi`,
which does not contain the substring `iscsi` — match `internetscsi` or the concrete
type, or you'll miss it.

## 5. Cleanup

- `gofmt -w pkg/inventory/switches.go` (it's the one dirty file).
- `make verify`: add a `vswitches --portgroup <name>` invocation and depend on the `test` target so it runs `go vet` + `go test` before the sim run.

---

## Definition of done

- [ ] `go build ./...` and `go vet ./...` exit 0; `gofmt -l .` prints nothing
- [ ] `go test ./...` — 0 failures, 0 skips, and **real** (non-empty, asserting) tests for vSwitches and port-group→VMs
- [ ] `vms`, `datastores`, `vswitches`, and `vswitches --portgroup` (a **standard** port group) all run against the sim with exit 0
- [ ] standard-switch ports are real (not 0); standard `--portgroup` returns its VMs
- [ ] classifier feeder wired (returns real transport on hardware, `unknown` on sim — proven by the table test)
- [ ] no deleted/empty tests, no leftover apology comments
