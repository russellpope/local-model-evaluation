# Round 2 — your last attempt regressed. Here is exactly what to fix.

Your round-1 remediation of the vSphere inventory CLI scored **worse** than the
original: 16/30 → 11/30. The reason is simple and fixable: **the code does not
compile.** You wrote plausible-looking Go against the govmomi API but invented
type and field names that do not exist, and you never ran `go build`.

## Rule #0 — this is why you failed

After **every** change, run:

```
go build ./...
go test ./...
```

Do not call anything "done", "fixed", or "complete" until **both are green** and
you have run all three subcommands plus `--portgroup` against a live simulator
with exit code 0. In round 1 you never ran `go build` a single time. Changing
this one behavior matters more than any individual fix below.

## The exact compile errors you shipped

```
datastores.go:60,62,64: undefined: types            (you use types.* but never import it)
datastores.go:66: ds.Summary != nil                 (Summary is a struct value, not a pointer)
datastores.go:66: ds.Summary.Capabilities undefined
switches.go:84:  dvs.Config.TeamingPolicy undefined
switches.go:92:  dvpg.Config.DefaultPortgroupVlan undefined
switches.go:99:  dvpg.Summary.NumPorts undefined
switches.go:119: host.Config.Network.vSwitch undefined
switches.go:182 / vms.go:39,44: struct-value != nil  (invalid nil comparisons)
```

## Wrong identifier → real govmomi v0.55.0 API

**datastores.go**
- Add `import "github.com/vmware/govmomi/vim25/types"`.
- `mo.Datastore.Summary` is a **value** (`types.DatastoreSummary`). Don't nil-check
  it. Use `ds.Summary.Type`, `ds.Summary.Capacity`, `ds.Summary.FreeSpace`.
- `ds.Summary.Capabilities` does not exist — delete that dead `if` block.

**switches.go (distributed)**
- `dvs.Config` is the interface `types.BaseDVSConfigInfo`. There is no
  `TeamingPolicy`. For LACP, type-assert: `vcfg, ok := dvs.Config.(*types.VMwareDVSConfigInfo)`,
  then LACP is enabled if any entry in `vcfg.LacpGroupConfig` has a non-empty
  `Mode` or `UplinkNum > 0`.
- DVPG VLAN is **not** `dvpg.Config.DefaultPortgroupVlan`. It is
  `dvpg.Config.DefaultPortConfig.(*types.VMwareDVSPortSetting).Vlan`, which is one
  of `*types.VmwareDistributedVirtualSwitchVlanIdSpec` /
  `...TrunkVlanSpec` / `...PvlanSpec` — switch on the concrete type.
- Port counts are **not** on the DVPG summary. Get them from the switch's ports:
  `object.NewDistributedVirtualSwitch(c, dvs.Reference()).FetchDVPorts(ctx, &types.DistributedVirtualSwitchPortCriteria{})`,
  then per port bump a total keyed by `port.PortgroupKey`; used = ports whose
  `Connectee != nil`.

**switches.go (standard)**
- The field is `Vswitch` (capital V), not `vSwitch`:
  `host.Config.Network.Vswitch` is `[]types.HostVirtualSwitch` with `.Name`,
  `.NumPorts`, `.NumPortsAvailable`, `.Pnic []string`.
- Standard port groups are **not** nested in the vSwitch struct
  (`HostVirtualSwitch.Portgroup` is a `[]string` of keys). Iterate
  `host.Config.Network.Portgroup` (`[]types.HostPortGroup`) and match
  `pg.Spec.VswitchName == sw.Name`. Each has `pg.Spec.Name` and `pg.Spec.VlanId`
  (an `int32`, not a pointer).
- The same standard switch (e.g. `vSwitch0`) exists on every host — merge by name
  so you don't emit one duplicate row per host.

**Port group → VMs** (works for standard *and* distributed): resolve the name in
the `Network` list to a MoRef, then list VMs with the `network` property and keep
any VM whose `v.Network` slice contains that ref. Simpler and more robust than
matching device backings.

## Pointer-vs-value — your nil-guards were the bug

You wrote `!= nil` on struct **values**. Guard the POINTER fields, use value
fields directly:

- `vm.Config` — pointer → `if vm.Config != nil` is valid.
- `vm.Config.Hardware` — **value** (`types.VirtualHardware`) → never `!= nil`;
  guard `vm.Config`, then use `vm.Config.Hardware.NumCPU`.
- `vm.Summary` — **value** → never `!= nil`. But `vm.Summary.Storage` **is** a
  pointer → `if vm.Summary.Storage != nil { ...Committed }` is the correct guard.
- `mo.Datastore.Summary` — **value**. `host.Config` and `host.Config.Network` —
  pointers (guard these before iterating).

Rule of thumb: if unsure whether a field is a pointer, grep the type in the
govmomi source in your module cache before using it. **Do not guess API names.**

## Conceptual fix 1 — TYPE is the transport, not the filesystem

You changed the classifier to return `VMFS`/`NFS`. That is the **wrong axis** and
the exact mistake the task warns about. `VMFS` is a filesystem, never a valid
answer. TYPE must be `FC`, `iSCSI`, `NVMe`, or `NFS`:

- NFS: `ds.Summary.Type` is `NFS`/`NFS41` (or Info is `*types.NasDatastoreInfo`).
- FC/iSCSI/NVMe: derive from the backing HBA. Walk the VMFS extent
  (`(*types.VmfsDatastoreInfo).Vmfs.Extent[].DiskName`) → the host's
  `config.storageDevice` (`HostStorageDeviceInfo`) → match `DiskName` to a
  `ScsiLun.CanonicalName` → follow `MultipathInfo.Lun[].Path[].Adapter` to a
  `HostBusAdapter` → classify by the adapter's concrete type/driver/model
  (contains `fibrechannel`/`fcoe` → FC, `iscsi` → iSCSI, `nvme` → NVMe).
- Factor the last step into a **pure** function `classifyTransport(descriptor)
  string` and table-test it with FC, iSCSI, and NVMe descriptors. vcsim does not
  model HBA topology, so against the simulator the correct answer is `unknown` —
  that is acceptable and expected. **Do not** write a test that asserts
  `expected:"VMFS"`.

## Conceptual fix 2 — the precedence test is still fake and now fails

Your test uses `v.Set("url","http://flag")` to simulate the flag. `v.Set` is
viper's **highest**-precedence override — it is not a flag, and it beats
`AutomaticEnv`, which is why your `EnvVarOverridesConfigFile` case **fails** (env
cannot override a `Set` value). Fix it two ways:

- Model each layer with its real mechanism: config file via a temp YAML +
  `ReadInConfig`, env via `os.Setenv` + `AutomaticEnv`, and the flag via a real
  `pflag.FlagSet` bound with `BindPFlag` then `flag.Set(...)` — **not** `v.Set` on
  the key.
- Better still, test the actual production wiring in `cmd/root.go` (`initConfig`),
  since that is what the criterion is about.

## make verify / actually running the loop

`go run github.com/vmware/govmomi/vcsim` does **not** resolve in govmomi v0.55.0 —
vcsim is a separate module now. Either (a) write a small launcher under
`tools/vcsim/` that imports `github.com/vmware/govmomi/simulator` (already your
dependency) and starts a server, or (b) pin the standalone vcsim module. Then make
`verify` depend on `test`, start the sim, run `vms`, `datastores`, `vswitches`,
**and** `vswitches --portgroup <name>`, and exit non-zero on any failure.

## Definition of done (every item must hold — verify, don't assume)

- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` clean, `gofmt -l .` prints nothing
- [ ] `go test ./...` — 0 failures, 0 skips
- [ ] all three subcommands + `--portgroup` run against vcsim with exit 0
- [ ] classifier returns FC/iSCSI/NVMe/NFS (unknown on sim), proven by a table
      test fed FC/iSCSI/NVMe descriptors
- [ ] no invented API names — every govmomi identifier verified against the SDK

Start by running `go build ./...`, reading the errors, and fixing them one file at
a time. Paste the final `go build`, `go test`, and simulator-run output as proof.
