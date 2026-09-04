# vsphere-inventory

A single-binary Go CLI that connects to a VMware vCenter Server (or the bundled
`vcsim` simulator) and reports virtualization inventory through three
subcommands: `vms`, `datastores`, and `vswitches`.

Built with [govmomi](https://github.com/vmware/govmomi), Cobra, and Viper.
All tables are rendered with `text/tabwriter` as plain, greppable text.

## Subcommands

| Command                     | Output                                                                                              |
|-----------------------------|-----------------------------------------------------------------------------------------------------|
| `vsphere-inventory vms`     | NAME, VCPU, RAM (GB), STORAGE (consumed/committed, GiB/TiB) — sorted by VM name                      |
| `vsphere-inventory datastores` | NAME, TYPE (transport: FC/iSCSI/NVMe/NFS), USED, AVAILABLE — sorted by datastore name            |
| `vsphere-inventory vswitches` | one row per port group across standard vSwitches and distributed switches (vDS): SWITCH, SWITCH TYPE, PORTGROUP, VLAN, UPLINKS, LACP, PORTS, USED |
| `vsphere-inventory vswitches --portgroup <name>` | the virtual machines attached to the named port group (standard or distributed) |

Notes on fields:

- `vms` STORAGE is the *consumed* (committed) storage from
  `summary.storage.committed`, not provisioned capacity.
- `datastores` TYPE is the backing *transport*, not the filesystem type:
  NFS datastores report `NFS`; VMFS datastores are classified by matching the
  volume's extent devices against the host's NVMe topology and SCSI topology
  (target transport). When the topology cannot be determined (e.g. local
  datastores, or the simulator) the type degrades to `unknown`.
- `vswitches` VLAN shows a single ID, a trunk range (`0-4094`), or `pvlan <id>`.
  LACP applies to distributed switches only and is reported `enabled` or
  `disabled` from the vDS LACP group configuration; standard vSwitches report
  `N/A`. For standard switches, USED = total − available ports
  (`NumPorts − NumPortsAvailable`), falling back to the in-use port count of
  the host port groups when the switch does not report availability; for
  distributed port groups, USED is the number of ports currently allocated
  (`portKeys`).

## Requirements

- Go 1.22+ (developed on Go 1.27)
- Direct dependencies only: `govmomi`, `cobra`, `viper`, and the Go standard
  library. `vcsim` is used as a Go *tool* (see `go.mod`) for local verification.

## Project layout

```
.
├── main.go                     # entry point
├── cmd/                        # Cobra command wiring + tabwriter presentation
│   ├── root.go                 # root command, flags, config load, connect/logout
│   ├── vms.go
│   ├── datastores.go
│   └── vswitches.go
└── internal/
    ├── config/                 # Viper-backed configuration with flag>env>file>default
    ├── vclient/                # govmomi client construction + actionable errors
    ├── format/                 # human-readable GiB/TiB formatting, used=total-free
    ├── transport/              # pure FC/iSCSI/NVMe/NFS transport classifier
    └── inventory/              # inventory retrieval (typed results, no I/O formatting)
```

Each feature's retrieval logic lives in `internal/inventory` as functions that
take a `context.Context` and a `*vim25.Client` and return typed results
(`[]VMInfo`, `[]DatastoreInfo`, `[]PortgroupInfo`), kept separate from the
Cobra wiring and the `tabwriter` presentation so every feature is directly
testable against govmomi's embedded simulator.

## Build

```
go build -o vsphere-inventory .
```

`go.mod` and `go.sum` are committed; if you change dependencies, run
`go mod tidy` to reconcile them.

## Configuration

All three subcommands share one connection configuration, resolved through
Viper with this precedence (highest first):

1. command-line flag
2. environment variable (`VSPHERE_` prefix)
3. config file (`--config <file.yaml>`)
4. built-in default

| Key        | Flag           | Env var              | Default |
|------------|----------------|----------------------|---------|
| url        | `--url`        | `VSPHERE_URL`        | —       |
| username   | `--username`   | `VSPHERE_USERNAME`   | —       |
| password   | `--password`   | `VSPHERE_PASSWORD`   | —       |
| insecure   | `--insecure`   | `VSPHERE_INSECURE`   | `false` |
| timeout    | `--timeout`    | `VSPHERE_TIMEOUT`    | `60s`   |
| config     | `--config`     | —                    | —       |

The URL may be a full SDK URL (`https://vc.lab/sdk`) or a bare host, in which
case `https://` and the `/sdk` path are added automatically. The whole
operation runs under a single `context.Context` derived from `timeout`, and the
client logs out cleanly when the command finishes.

### Config file example

```
./vsphere-inventory vms --config config.example.yaml
```

```yaml
# config.example.yaml
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: change-me
insecure: false
timeout: 60s
```

### Environment variable example

```
export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD=change-me
export VSPHERE_INSECURE=false
export VSPHERE_TIMEOUT=60s
./vsphere-inventory datastores
```

Errors are wrapped with context and give actionable hints, e.g. an untrusted
self-signed certificate suggests `--insecure`, a bad login names the user and
the env vars to check, and a timed-out connection suggests raising the timeout.

## Tests

```
go test ./...
```

Tests use govmomi's embedded `simulator` package (no external `vcsim` binary),
so they are hermetic. Coverage:

- **VMs** — a known model VM count is asserted, and each result must have a
  non-empty name, vCPU > 0, RAM > 0, storage ≥ 0, sorted output.
- **Datastores** — each has a non-empty name, transport ∈
  FC/iSCSI/NVMe/NFS/unknown, available ≤ capacity, and
  `used + available == capacity` exactly.
- **vSwitches** — at least one standard and one distributed row; VLAN values
  parse; used ports ≤ total ports; LACP ∈ enabled/disabled/N/A.
- **Port group → VMs** — for both a distributed port group (VMs attached by the
  simulator model) and the standard `VM Network`, the lookup is asserted to
  return *exactly* the set computed independently from raw `network` refs; an
  unknown port group returns an error.
- **Config precedence** — flag > env > file > default, plus invalid timeouts.
- **Byte formatting** — table-driven GiB/TiB cases and `used = total − free`.
- **Transport classifier** — table-driven FC, iSCSI, NVMe, NFS, and unknown
  cases over synthetic SCSI/NVMe topology descriptors.

Sample run:

```
$ go test ./...
?       vsphere-inventory       [no test files]
?       vsphere-inventory/cmd   [no test files]
ok      vsphere-inventory/internal/config       0.459s
ok      vsphere-inventory/internal/format       0.229s
ok      vsphere-inventory/internal/inventory    3.323s
ok      vsphere-inventory/internal/transport    0.843s
?       vsphere-inventory/internal/vclient      [no test files]
```

## Verification against `vcsim`

`make verify` runs the full check: `gofmt` cleanliness, `go vet ./...`,
`go build`, `go test ./...`, then starts `vcsim` in the background (`go tool
vcsim -vm 8 -ds 3 -pg 3`), waits for it to be ready, runs all three
subcommands plus a `--portgroup` invocation for a distributed and a standard
port group discovered from the `vswitches` output, and exits non-zero on any
failure. The simulator is torn down afterwards. The default listen address is
`127.0.0.1:8989`; override with `make verify VCSIM_ADDR=127.0.0.1:8990`.

Sample `vcsim` run (the simulator's inventory is minimal — 32 MiB VMs, no
committed-storage accounting, local datastores with no fabric topology):

```
$ make verify VCSIM_ADDR=127.0.0.1:8990
...
>> vms
NAME            VCPU  RAM     STORAGE
DC0_C0_RP0_VM0  1     0.0 GB  0.0 GiB
...
DC0_H0_VM7      1     0.0 GB  0.0 GiB
>> datastores
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  3.8 TiB
LocalDS_1  unknown  0.0 GiB    4.0 TiB
LocalDS_2  unknown  0.0 GiB    4.0 TiB
>> vswitches
SWITCH    SWITCH TYPE  PORTGROUP           VLAN    UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0       N/A      disabled  1      1
DVS0      distributed  DC0_DVPG1           0       N/A      disabled  1      1
DVS0      distributed  DC0_DVPG2           0       N/A      disabled  1      1
DVS0      distributed  DVS0-DVUplinks-8    0-4094  N/A      disabled  1      1
vSwitch0  standard     Management Network  0       vmnic0   N/A       1536   6
vSwitch0  standard     VM Network          0       vmnic0   N/A       1536   6
>> vswitches --portgroup DC0_DVPG0
NAME
DC0_C0_RP0_VM0
...
DC0_H0_VM7
>> vswitches --portgroup Management Network
NAME
>> verify OK
```

### Simulator fidelity

`vcsim` is an integration/smoke harness, not a correctness oracle: it does not
model storage transport topology (HBA → LUN → extent) or LACP/uplink detail.
Against it, datastore `TYPE` and `vswitches` `LACP`/`UPLINKS` legitimately
degrade to `unknown`/`N/A` — no data is fabricated or special-cased to make the
output look complete. The transport classifier's own logic is proven by its
dedicated pure-function test with FC, iSCSI, and NVMe topology fixtures.

## Deployment against a live vCenter

The same binary runs unmodified against a real vCenter: point `--url` at the
SDK endpoint, supply credentials, and (only if you must) `--insecure`. Against
live inventory, datastore transport is derived from the actual backing
topology and distributed-switch LACP/uplink state is reported as configured.
