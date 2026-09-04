# vsphere-inventory

A single Go CLI that connects to a VMware vCenter Server and reports
virtualization inventory. Built on `govmomi`, `cobra` and `viper`.

## Subcommands

| Command | Reports |
|---|---|
| `vms` | All virtual machines: NAME, VCPU, RAM (GB), STORAGE — **consumed/committed** storage in GiB/TiB, not provisioned |
| `datastores` | All datastores: NAME, TYPE (**real transport**: FC / iSCSI / NVMe / NFS — derived from the backing HBAs and SCSI topology, *not* the filesystem type), USED, AVAILABLE |
| `vswitches` | All **standard** and **distributed** switches with their port groups: SWITCH, SWITCH TYPE, PORTGROUP, VLAN (ranges/pvlan for trunk & pvlan groups), UPLINKS, LACP (`N/A` for standard switches — LACP is vDS-only), PORTS, USED |
| `vswitches --portgroup NAME` | Instead lists the VMs connected to the named port group (works for standard **and** distributed port groups) |

All tables are plain greppable text via `text/tabwriter`, sorted by name,
units in GiB/TiB with one decimal place.

## Project layout

```
vsphere-inventory/
├── main.go                        # entry point
├── Makefile                       # build + `make verify` full gate
├── config.example.yaml            # example YAML config file
├── go.mod                         # deps: govmomi, cobra, viper only
└── internal/
    ├── cmd/                       # cobra wiring + tabwriter presentation
    │   ├── root.go                #   shared flags (--url/--config/...)
    │   ├── commands.go            #   vms / datastores / vswitches
    │   └── table.go               #   table rendering
    ├── config/                    # viper configuration with precedence
    │   └── config.go              #   flag > env > file > default
    └── inventory/                 # retrieval logic (unit-testable core)
        ├── client.go              #   connect/login/logout
        ├── vms.go                 #   ListVMs(ctx, client) -> []VMInfo
        ├── datastores.go          #   ListDatastores -> []DatastoreInfo (+ LUN transport map)
        ├── transport.go           #   pure FC/iSCSI/NVMe classifier
        ├── vswitches.go           #   ListSwitches / ListPortGroupVMs
        └── format.go              #   byte formatting helpers
```

Inventory retrieval lives in functions taking a `context.Context` and a
`*vim25.Client`, returning typed results — separate from both Cobra wiring
and tabwriter rendering, so each feature is directly unit-testable against
govmomi's embedded simulator.

## Build

Requires Go 1.22+.

```sh
go mod tidy        # resolve module dependencies into go.sum
make build         # produces bin/vsphere-inventory
```

Dependencies are limited to:

- `github.com/vmware/govmomi`
- `github.com/spf13/cobra`
- `github.com/spf13/viper`
- the Go standard library (`text/tabwriter` for all tables)

## Configuration

Same keys everywhere: `url`, `username`, `password`, `insecure`, `timeout`.
Precedence (highest first): command-line flag → environment variable →
config file → built-in default.

| Key      | Flag        | Env var            | Default |
|----------|-------------|--------------------|---------|
| url      | `--url`     | `VSPHERE_URL`      | — (required) |
| username | `--username`| `VSPHERE_USERNAME` | — |
| password | `--password`| `VSPHERE_PASSWORD` | — |
| insecure | `--insecure`| `VSPHERE_INSECURE` | `false` |
| timeout  | `--timeout` | `VSPHERE_TIMEOUT`  | `60s` |

Via environment variables:

```sh
export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD='***'
export VSPHERE_INSECURE=false
./bin/vsphere-inventory vms
./bin/vsphere-inventory datastores
./bin/vsphere-inventory vswitches
./bin/vsphere-inventory vswitches --portgroup "Prod-VLAN10"
```

Via a YAML file (see `config.example.yaml`):

```sh
./bin/vsphere-inventory --config config.yaml vms
# flags still override the file:
VSPHERE_URL=https://other-vc.lab/sdk ./bin/vsphere-inventory --config config.yaml vms
```

## Verification

```sh
make verify
```

runs the complete local gate:

1. `go vet ./...`
2. `go test ./...` — hermetic unit tests using govmomi's embedded
   `simulator` package (in-process vCenter; no external binary needed):
   - VMs: count/name/vCPU/RAM/storage invariants
   - datastores: used + available == capacity, available ≤ capacity,
     TYPE ∈ FC/iSCSI/NVMe/NFS/unknown
   - vSwitches: sorting, VLAN parseability, used ≤ total ports,
     LACP ∈ enabled/disabled/N/A, standard + distributed coverage
   - port group → VMs: exact membership for a known port group
   - pure helpers: config precedence (flag > env > file > default),
     byte formatting, transport classifier (FC/iSCSI/NVMe descriptors)
3. starts `vcsim` in the background, waits until ready, runs every
   subcommand plus a `--portgroup` invocation discovered from the live
   `vswitches` output, and tears the simulator down afterwards.

### Simulator fidelity notes (honest degradation, no fabricated data)

`vcsim` is an integration smoke harness, not a correctness oracle:

- Its hosts have only local parallel-SCSI devices, so datastore `TYPE`
  legitimately reports `unknown`; the real FC/iSCSI/NVMe decision logic is
  proven separately by the pure-function classifier test.
- DVS uplink port groups exist but carry no physical-NIC detail, so
  distributed `UPLINKS` may show `-`; standard vSwitch uplinks come from
  nic-teaming order.
- Fields the API does not populate degrade to `-` / `unknown` / `N/A`;
  nothing is invented.

Against a **live vCenter** the same code derives real transports by mapping
each VMFS extent disk to its HBA (FibreChannel/FibreChannelOverEthernet →
FC, InternetScsi → iSCSI, PCIe/TCP/RDMA NVMe adapters → NVMe) and reports
LACP from the vDS LAG configuration (`enabled` when LACP groups exist).

Two other behaviours worth knowing:

- Standard vSwitches repeated identically across hosts (e.g. every host's
  `vSwitch0`) are collapsed to one row since the report has no host column.
- For distributed port groups, PORTS/USED reflect the port group's own
  allocation (`config.numPorts`) and connected ports; for standard port
  groups they reflect the parent host vSwitch totals (the API exposes port
  counts at switch level only).

## Run output sample

From `make verify` against vcsim:

```text
== vms ==
NAME             VCPU  RAM  STORAGE
DC0_C0_RP0_VM0   1     0.0  0.0 GiB
...
== datastores ==
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  9.8 TiB
LocalDS_1  unknown  0.0 GiB    10.0 TiB
LocalDS_2  unknown  0.0 GiB    10.0 TiB
== vswitches ==
SWITCH    SWITCH TYPE  PORTGROUP           VLAN          UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0             -        disabled  1      1
DVS0      distributed  DVS0-DVUplinks-9    trunk 0-4094  -        disabled  1      1
vSwitch0  standard     Management Network  0             vmnic0   N/A       1536   6
vSwitch0  standard     VM Network          0             vmnic0   N/A       1536   6
== vswitches --portgroup DC0_DVPG0 ==
NAME
DC0_C0_RP0_VM0
DC0_C0_RP0_VM1
...
```

(`RAM`/`STORAGE` print as `0.0` because simulated VMs have 32 MB RAM and
sub-GiB committed storage; one-decimal formatting hides that at this scale.)
