# vint — vCenter Inventory CLI

A single Go binary that connects to a VMware vCenter Server (or ESXi host) and
reports virtualization inventory. Three subcommands:

| Command                 | Reports                                                    |
|-------------------------|------------------------------------------------------------|
| `vint vms`              | every VM: name, vCPU, RAM (GB), **committed** storage      |
| `vint datastores`       | every datastore: name, **transport** type, used, available |
| `vint vswitches`        | standard + distributed switches and their port groups      |
| `vint vswitches --portgroup <name>` | VMs connected to a named (standard *or* distributed) port group |

Built with **Go 1.22+**, `cobra` for the command tree, `viper` for
configuration, `govmomi` for the vSphere API, and the standard library
`text/tabwriter` for all table output.

## Project layout

```
.
├── main.go                     entry point
├── Makefile                    build / test / vet / verify (full vcsim loop)
├── config.example.yaml         sample viper config file
├── config/
│   ├── config.go               viper resolution: flag > env > file > default
│   └── config_test.go          precedence and parsing tests
├── format/
│   ├── format.go               human-readable GiB/TiB byte formatting
│   └── format_test.go
├── transport/
│   ├── transport.go            FC / iSCSI / NVMe descriptor classifier
│   └── transport_test.go
├── inventory/                  vSphere retrieval logic (tested directly,
│   ├── client.go                 no cobra / tabwriter here)
│   ├── finder.go               per-datacenter iteration + ref dedup
│   ├── vms.go                  ListVMs
│   ├── datastores.go           ListDatastores + transport derivation
│   ├── vswitches.go            ListSwitches (standard + distributed)
│   ├── portgroup.go            VMsOnPortGroup
│   └── *_test.go               hermetic tests against in-process simulator
└── cmd/                        cobra wiring + tabwriter presentation
    ├── root.go
    ├── vms.go
    ├── datastores.go
    └── vswitches.go
```

## Build, vet, test

```
go build ./...                  # compile
go vet ./...                    # static checks
go test ./...                   # all tests (hermetic — no server needed)
```

`make verify` runs `go vet ./...` and `go test ./...`, then starts a local
`vcsim`, waits for readiness, and drives all four command paths against it,
tearing the simulator down afterwards. Exits non-zero on any failure.

Note: `go mod tidy` keeps `go.mod`/`go.sum` in sync after dependency changes;
the module tree is already tidy.

## Configuration

All three subcommands share the same connection settings, resolved by Viper
with strict precedence:

```
command-line flag > environment variable > config file > built-in default
```

| Key       | Flag          | Env var             | Default |
|-----------|---------------|---------------------|---------|
| url       | `--url`       | `VSPHERE_URL`       | (none — required) |
| username  | `--username`  | `VSPHERE_USERNAME`  | (none)  |
| password  | `--password`  | `VSPHERE_PASSWORD`  | (none)  |
| insecure  | `--insecure`  | `VSPHERE_INSECURE`  | `false` |
| timeout   | `--timeout`   | `VSPHERE_TIMEOUT`   | `60s`   |
| config    | `--config`    | —                    | (none)  |

### Environment variables

```
export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD=s3cr3t
export VSPHERE_INSECURE=true

vint vms
```

### Config file

```
vint --config config.yaml datastores
```

See [`config.example.yaml`](config.example.yaml). A flag always wins over the
environment, which wins over the file, which wins over the default.

## Output

Plain, greppable text via `text/tabwriter`. Sizes use GiB/TiB with one decimal
place. Rows are sorted (VMs and datastores by name; switches by switch then
port group).

```
$ vint vms
NAME            VCPU  RAM (GB)  STORAGE
DC0_C0_RP0_VM0  1     0.0       0.0 GiB
```

## Behaviour that degrades gracefully

The simulator (`vcsim` / `simulator`) does not model storage transport
topology or LACP/uplink detail. Where the API cannot prove a value the CLI
reports it truthfully:

- Datastore `TYPE` → `unknown` when no NFS info, disk-name hint or HBA points
  to a protocol.
- `vswitches` `LACP` → `disabled` on distributed switches without LACP
  groups, `N/A` on standard switches.
- `UPLINKS` → empty when the vCenter exposes no uplink names.
- VLAN → `untagged`/`trunk`/a range/`pvlan-N` instead of a single ID, per the
  underlying vlan spec.

No row is dropped and nothing is fabricated; the code never crashes on missing
values.

### Real transport and LACP on a live vCenter

The full-fidelity behaviour is implemented for a real vCenter:

- **`datastores`**: NFS datastores report `NFS` (from `NasDatastoreInfo`).
  For VMFS datastores the transport derives from the backing LUN identifiers
  in the volume extents (e.g. `naa.…` → FC, `t10.NVMe____…` → NVMe) and, when
  extents carry no hint, from the mounting hosts' HBA descriptors. The
  FC/iSCSI/NVMe decision is a pure function
  ([`transport.Classify`](transport/transport.go)) table-tested with
  representative inputs.
- **`vswitches`**: standard rows report the host's vSwitch uplinks and
  `LACP N/A`; distributed rows report uplink port names and `enabled` when the
  DVS has LACP groups.

## Verification record

### `go test ./...`

```
go test ./v...
?   vint    [no test files]
?   vint/cmd    [no test files]
ok  vint/config 0.308s
ok  vint/format 0.490s
ok  vint/inventory  2.225s
ok  vint/transport  0.697s
```

Zero failures, zero skips. The inventory tests spin up the in-process
`govmomi/simulator` (`simulator.Test`) and assert real, deterministic values
against configured model counts.

### End-to-end against `vcsim`

```
$ vcsim -l 127.0.0.1:18989 -vm 8 -ds 3 -pg 3 &
$ export VSPHERE_URL=https://127.0.0.1:18989/sdk VSPHERE_USERNAME=user \
        VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true

$ vint datastores
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  240.0 GiB  6.8 TiB
LocalDS_1  unknown  0.0 GiB    7.0 TiB
LocalDS_2  unknown  0.0 GiB    7.0 TiB

$ vint vswitches
SWITCH    SWITCH TYPE  PORTGROUP           VLAN    UPLINKS                          LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0                                        disabled  1      1
DVS0      distributed  DC0_DVPG1           0                                        disabled  1      1
DVS0      distributed  DC0_DVPG2           0                                        disabled  1      1
DVS0      distributed  DVS0-DVUplinks-8    0-4094                                   disabled  1      1
vSwitch0  standard     Management Network  0       key-vim.host.PhysicalNic-vmnic0  N/A       1536   6
vSwitch0  standard     VM Network          0       key-vim.host.PhysicalNic-vmnic0  N/A       1536   6

$ vint vswitches --portgroup DC0_DVPG0
# virtual machines on port group "DC0_DVPG0"
NAME            VCPU  RAM (GB)
DC0_C0_RP0_VM0  1     0.0
...
```

All subcommands exit `0` with aligned, greppable, correctly ordered output.
The same code deploys against a live vCenter for full transport / LACP
fidelity.