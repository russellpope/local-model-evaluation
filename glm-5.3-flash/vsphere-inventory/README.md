# vsphere-inventory

A single-binary Go CLI that connects to a VMware vCenter Server and reports
virtualization inventory: virtual machines, datastores (with real transport
type), and virtual switches (standard + distributed) with their port groups.

Built on [`govmomi`](https://github.com/vmware/govmomi) (vSphere API client),
[`cobra`](https://github.com/spf13/cobra) (command structure) and
[`viper`](https://github.com/spf13/viper) (configuration). All tables are
rendered with the standard library `text/tabwriter`.

## Requirements

- Go 1.22+ (developed and verified with Go 1.27)
- Direct dependencies: `github.com/vmware/govmomi`, `github.com/spf13/cobra`,
  `github.com/spf13/viper`, plus the Go standard library — nothing else.

> **Why govmomi v0.46.2?** From v0.47.0 the `vcsim` command moved into a
> nested module whose `go.mod` contains a `replace` directive, so
> `go run github.com/vmware/govmomi/vcsim` no longer works from a downstream
> module on newer releases. Pinning v0.46.2 keeps the client library and the
> bundled simulator at the same version, exactly as the task describes. After
> editing dependencies, run `go mod tidy` (or `make tidy`) to reconcile
> `go.mod`/`go.sum`.

## Project layout

```
vsphere-inventory/
├── main.go                       # entry point (cobra Execute, exit codes)
├── cmd/                          # cobra wiring + tabwriter presentation
│   ├── root.go                   #   root command, flags, help
│   ├── connect.go                #   config load, timeout context, login, logout
│   ├── vms.go                    #   `vms`
│   ├── datastores.go             #   `datastores`
│   └── vswitches.go              #   `vswitches` (+ --portgroup)
├── internal/
│   ├── config/                   # viper-based resolution: flag > env > file > default
│   │   ├── config.go
│   │   └── config_test.go        # precedence + validation tests
│   ├── format/                   # pure GiB/TiB + used-capacity helpers
│   │   ├── format.go
│   │   └── format_test.go
│   └── inventory/                # retrieval logic (client in, typed results out)
│       ├── inventory.go          #   VMInfo / DatastoreInfo / SwitchInfo types
│       ├── vms.go                #   ListVMs
│       ├── datastores.go         #   ListDatastores + per-host storage walk
│       ├── transport.go          #   HBA -> FC/iSCSI/NVMe classifier (pure)
│       ├── switches.go           #   ListSwitches (standard + distributed)
│       ├── portgroup.go          #   VMsOnPortgroup (standard + distributed)
│       ├── transport_test.go     #   classifier table tests
│       └── inventory_test.go     #   simulator-backed feature tests
├── scripts/verify.sh             # full automated check (vet+test+build+vcsim loop)
├── config.example.yaml           # example YAML config file
├── Makefile                      # build / test / vet / verify / tidy / clean
└── go.mod
```

Retrieval logic lives in `internal/inventory` as functions that take a
`context.Context` and a vSphere client and return typed results
(`[]VMInfo`, `[]DatastoreInfo`, `[]SwitchInfo`, `[]VMInfo` for the port-group
lookup) — deliberately separated from cobra wiring and tabwriter rendering so
each feature is directly unit-testable.

## Build

```sh
go build -o bin/vsphere-inventory .    # or: make build
```

## Configuration

All subcommands share one connection configuration, resolved through Viper
with this precedence (highest first):

1. command-line flag
2. environment variable (`VSPHERE_` prefix)
3. YAML config file (`--config`)
4. built-in default

| Key      | Flag        | Env var            | Default | Notes                            |
|----------|-------------|--------------------|---------|----------------------------------|
| url      | `--url`     | `VSPHERE_URL`      | —       | vCenter URL or host              |
| username | `--username`| `VSPHERE_USERNAME` | —       |                                  |
| password | `--password`| `VSPHERE_PASSWORD` | —       |                                  |
| insecure | `--insecure`| `VSPHERE_INSECURE` | `false` | skip TLS verification            |
| timeout  | `--timeout` | `VSPHERE_TIMEOUT`  | `60s`   | overall operation timeout        |
| config   | `--config`  | —                  | —       | path to a YAML config file       |

Example `config.yaml` (see `config.example.yaml`):

```yaml
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: change-me
insecure: false
timeout: 60s
```

Same settings via environment variables:

```sh
export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD='change-me'
export VSPHERE_INSECURE=false
export VSPHERE_TIMEOUT=60s
```

One authenticated client is created per invocation from a context bounded by
the configured timeout; it is logged out when the command finishes.
Authentication failures (vSphere `InvalidLogin`) and connection failures are
wrapped with actionable hints, e.g.:

```
Error: could not connect to vCenter at https://127.0.0.1:18989/sdk: verify --url and network; use --insecure for self-signed certificates: Post "https://127.0.0.1:18989/sdk": context deadline exceeded
Error: no vCenter URL configured: set --url, $VSPHERE_URL, or "url" in the config file
```

## Usage

```sh
vsphere-inventory vms
vsphere-inventory datastores
vsphere-inventory vswitches
vsphere-inventory vswitches --portgroup "DC0_DVPG0"
```

`vms` reports **consumed** (committed) storage — `summary.storage.committed` —
never provisioned capacity. `datastores` reports the **transport** backing the
datastore (FC / iSCSI / NVMe derived from the extents' HBA topology on
attached hosts; NFS for network volumes), not the filesystem type.
`vswitches` covers standard and distributed switches; LACP is reported
`enabled`/`disabled` for distributed switches and `N/A` for standard ones
(it does not apply there).

## Tests

Hermetic: unit tests use govmomi's embedded
`github.com/vmware/govmomi/simulator` package (`simulator.VPX()` model with a
fixed inventory), so a plain `go test ./...` works with no external vcsim.
Zero failures, zero skips — every test carries real assertions.

```sh
go test ./...          # or: make test
```

Sample result from an actual run:

```
$ go test -count=1 ./...
?       vsphere-inventory       [no test files]
?       vsphere-inventory/cmd   [no test files]
ok      vsphere-inventory/internal/config       0.175s
ok      vsphere-inventory/internal/format       0.286s
ok      vsphere-inventory/internal/inventory    1.314s
```

Covered: VMs (count/name/vCPU/RAM/storage + sort), datastores
(used+available == capacity, transport ∈ FC/iSCSI/NVMe/NFS/unknown),
vSwitches (standard+distributed present, LACP ∈ enabled/disabled/N-A, used ≤
ports, VLAN parseable), port-group→VM lookup (distributed: all model VMs on
`DC0_DVPG0`, empty second PG, standard `VM Network` after attaching a vNIC,
error on unknown PG), config precedence (flag > env > file > default for
url/username/password/insecure/timeout), byte formatting + used-capacity
math, and the transport classifier.

## Full verification loop (`make verify`)

`scripts/verify.sh` (wired to `make verify`) runs `go vet`, `go test -count=1`,
builds the binary, starts a scaled `vcsim` (`-vm 8 -ds 3 -pg 3`) in the
background, waits for readiness, drives every code path — including
`--portgroup` with a name discovered from the `vswitches` output — checks the
error paths (unknown port group, missing URL), and tears the simulator down,
exiting non-zero on any failure.

Sample `vcsim` run from an actual execution:

```
$ make verify
==> go vet ./...
==> go test -count=1 ./...
ok      vsphere-inventory/internal/config       0.175s
ok      vsphere-inventory/internal/format       0.286s
ok      vsphere-inventory/internal/inventory    1.314s
==> go build
==> starting vcsim on 127.0.0.1:18989
==> vms
NAME            VCPU  RAM (GB)  STORAGE
DC0_C0_RP0_VM0  1     0.0       0.0 GiB
...
DC0_H0_VM7      1     0.0       0.0 GiB
==> datastores
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  9.8 TiB
LocalDS_1  unknown  0.0 GiB    10.0 TiB
LocalDS_2  unknown  0.0 GiB    10.0 TiB
==> vswitches
SWITCH    SWITCH TYPE  PORTGROUP           VLAN           UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0              unknown  disabled  4      4
DVS0      distributed  DC0_DVPG1           0              unknown  disabled  4      4
DVS0      distributed  DC0_DVPG2           0              unknown  disabled  4      4
DVS0      distributed  DVS0-DVUplinks-9    trunk(0-4094)  unknown  disabled  4      4
vSwitch0  standard     Management Network  0              vmnic0   N/A       6144   24
vSwitch0  standard     VM Network          0              vmnic0   N/A       6144   24
==> vswitches --portgroup "DC0_DVPG0" (name discovered from output above)
NAME
DC0_C0_RP0_VM0
...
DC0_H0_VM7
==> error path: unknown port group must exit non-zero
==> error path: missing VSPHERE_URL must exit non-zero
verify: OK
```

Manual loop, if you prefer driving it yourself:

```sh
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3   # defaults: https://127.0.0.1:8989/sdk, user/pass
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true
bin/vsphere-inventory vms
bin/vsphere-inventory datastores
bin/vsphere-inventory vswitches
bin/vsphere-inventory vswitches --portgroup "$(bin/vsphere-inventory vswitches | awk 'NR==2{print $3}')"
```

## Simulator fidelity (honest limitations)

`vcsim` is a smoke harness, not a correctness oracle. In the run above the
fields the simulator does not model degrade exactly as specified instead of
being faked:

- datastore `TYPE` shows `unknown` — vcsim attaches VMFS datastores to local
  parallel/IDE HBAs, which are genuinely neither FC, iSCSI nor NVMe. The
  FC/iSCSI/NVMe decision itself is proven by the pure-classifier unit tests
  (`internal/inventory/transport_test.go`).
- RAM shows `0.0` and STORAGE `0.0 GiB` — the simulator's toy VMs are
  configured with 32 MiB RAM and report no committed storage.
- DVS `UPLINKS` shows `unknown` — vcsim does not populate the uplink port
  name policy; standard-switch uplinks (`vmnic0`) come from the vSwitch spec.

Against a **live vCenter** the same code paths resolve real values: VM
committed storage from `summary.storage.committed`, datastore transport from
the extents → SCSI LUN → topology adapter → HBA chain (FC/FCoE → `FC`,
software iSCSI → `iSCSI`, PCIe/NVMe-TCP → `NVMe`, NAS → `NFS`), DVS LACP from
`VMwareDVSConfigInfo.lacpApiVersion`, and uplink names from the DVS uplink
port policy. Nothing is hardcoded to the simulator.

## Live vCenter deployment

The same binary works unchanged against a real vCenter — only the
connection settings differ:

```sh
export VSPHERE_URL=https://vc.corp.example/sdk
export VSPHERE_USERNAME=svc-readonly@vsphere.local
export VSPHERE_PASSWORD='…'
vsphere-inventory vms
vsphere-inventory vswitches --portgroup 'Production-VLAN101'
```

A read-only role is sufficient for every operation performed here.
