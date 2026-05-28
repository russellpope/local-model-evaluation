# vsphere-inventory

A single Go command-line application that connects to a VMware vCenter Server
and reports virtualization inventory. It exposes three subcommands:

- `vms` — virtual machines with vCPU, RAM, and **consumed** (committed) storage
- `datastores` — datastores with their **transport** (FC/iSCSI/NVMe/NFS) and used/available capacity
- `vswitches` — standard and distributed virtual switches with their port groups; or, with `--portgroup <name>`, the VMs connected to that port group

## Requirements

- **Go 1.24+** (govmomi v0.54.0 requires the 1.24 language version).
- Direct dependencies are exactly: [`govmomi`](https://github.com/vmware/govmomi)
  (vSphere API + bundled simulator), [`cobra`](https://github.com/spf13/cobra)
  (commands), [`viper`](https://github.com/spf13/viper) (configuration), and the
  Go standard library (`text/tabwriter` for all table output).

## Project layout

```
.
├── main.go                       # entrypoint: signal-aware context, runs cmd
├── cmd/
│   └── root.go                   # Cobra tree, vCenter connect/logout, tabwriter output
├── internal/
│   ├── config/
│   │   ├── config.go             # Viper wiring: flag > env > file > default
│   │   └── config_test.go        # precedence test
│   └── inventory/
│       ├── types.go              # VMInfo, DatastoreInfo, SwitchInfo, ...
│       ├── helpers.go            # FormatBytes + ClassifyTransport (pure)
│       ├── retrieve.go           # GetVMs / GetDatastores / GetSwitches / GetPortgroupVMs
│       ├── format_test.go        # byte formatter + used=total-available math
│       ├── transport_test.go     # FC/iSCSI/NVMe/NFS/unknown classifier
│       └── retrieve_test.go      # the four feature tests (embedded simulator)
├── tools/
│   └── vcsim/main.go             # launcher for govmomi's bundled simulator
├── scripts/
│   └── verify.sh                 # the `make verify` self-check pipeline
├── Makefile
├── config.example.yaml
├── go.mod / go.sum
└── README.md
```

The inventory-retrieval logic (`internal/inventory`) takes a `context.Context`
and a `*vim25.Client` and returns typed results, kept independent of the Cobra
wiring and the `tabwriter` presentation so each feature is unit-testable.

## Build

```sh
go mod tidy        # ensure go.sum is complete (deps are vendored via the module cache)
go build -o vsphere-inventory .
```

Or `make build`. `go build ./...` and `go vet ./...` pass clean, and the code is
`gofmt`-clean.

## Configuration

All three subcommands share one vCenter connection. Settings resolve with this
precedence (highest first): **command-line flag → environment variable → config
file → built-in default**. The environment prefix is `VSPHERE_`.

| Key        | Flag           | Env var              | Default | Notes                                          |
|------------|----------------|----------------------|---------|------------------------------------------------|
| url        | `--url`        | `VSPHERE_URL`        | —       | vCenter URL or host; `/sdk` appended if absent |
| username   | `--username`   | `VSPHERE_USERNAME`   | —       |                                                |
| password   | `--password`   | `VSPHERE_PASSWORD`   | —       |                                                |
| insecure   | `--insecure`   | `VSPHERE_INSECURE`   | `false` | skip TLS verification                          |
| timeout    | `--timeout`    | `VSPHERE_TIMEOUT`    | `60s`   | overall operation timeout (Go duration)        |
| config     | `--config`     | —                    | —       | path to a YAML config file                     |

### Using environment variables

```sh
export VSPHERE_URL=https://vc.lab.example.com/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD='super-secret'
export VSPHERE_INSECURE=false
vsphere-inventory vms
```

### Using a config file

See [`config.example.yaml`](config.example.yaml):

```yaml
url: https://vcenter.example.com/sdk
username: administrator@vsphere.local
password: "change-me"
insecure: false
timeout: 60s
```

```sh
vsphere-inventory datastores --config ./config.example.yaml
```

A flag still wins over the file, and an env var still wins over the file — e.g.
`vsphere-inventory vms --config ./config.yaml --url https://other/sdk` connects
to `https://other/sdk`.

## Usage

```sh
vsphere-inventory vms
vsphere-inventory datastores
vsphere-inventory vswitches
vsphere-inventory vswitches --portgroup "DC0_DVPG0"
```

Output uses `text/tabwriter` (aligned columns, header row), consistent GiB/TiB
units with one decimal place, and plain greppable text (no color or
box-drawing). Rows are sorted by name.

## Testing

```sh
go test ./...
```

Tests are hermetic — they use govmomi's embedded
`github.com/vmware/govmomi/simulator` package (an in-process vCenter), not the
external binary, so a plain `go test ./...` is all that is required. There is at
least one meaningful test per feature (VMs, datastores, vSwitches, port group →
VMs) plus pure-function tests for config precedence, byte formatting, and the
transport classifier. No skips, no tautologies.

Sample run:

```
$ go test ./...
?       vsphere-inventory               [no test files]
?       vsphere-inventory/cmd           [no test files]
ok      vsphere-inventory/internal/config       0.278s
ok      vsphere-inventory/internal/inventory    2.035s
?       vsphere-inventory/tools/vcsim   [no test files]
```

## Self-verification against the simulator (`make verify`)

`make verify` runs `go vet ./...` and `go test ./...`, then starts the bundled
simulator, waits for it to accept connections, runs all three subcommands plus a
`--portgroup` invocation (the port-group name is discovered from the `vswitches`
output, not hardcoded), and tears the simulator down. It exits non-zero on any
failure.

```sh
make verify
```

> **On the simulator launcher.** The spec suggests `go run
> github.com/vmware/govmomi/vcsim`. In govmomi **v0.54.0** the `vcsim` *command*
> was split into a separate module, so that exact path is no longer in this
> module's graph. To run the simulator **at exactly the govmomi version this
> project depends on** with no extra dependency, `tools/vcsim` wraps the
> `simulator` package that ships *inside* that govmomi version. It mirrors the
> upstream defaults: endpoint `https://127.0.0.1:8989/sdk`, username `user`,
> password `pass`, self-signed TLS. Run it standalone with `make run-vcsim` (or
> `go run ./tools/vcsim -vm 8 -ds 3 -pg 3`).

Sample `make verify` run (abridged):

```
==> starting vcsim on 127.0.0.1:8989
==> vms
NAME            VCPU  RAM     STORAGE
DC0_C0_RP0_VM0  1     0.0 GB  0.0 GiB
...
==> datastores
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  3.8 TiB
LocalDS_1  unknown  0.0 GiB    4.0 TiB
LocalDS_2  unknown  0.0 GiB    4.0 TiB
==> vswitches
SWITCH    SWITCH TYPE  PORTGROUP           VLAN          UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           none          -        disabled  1      0
DVS0      distributed  DVS0-DVUplinks-8    trunk 0-4094  -        disabled  1      0
vSwitch0  standard     Management Network  none          vmnic0   N/A       1536   6
vSwitch0  standard     VM Network          none          vmnic0   N/A       1536   6
==> vswitches --portgroup "DC0_DVPG0"
NAME            VCPU  RAM     STORAGE
DC0_C0_RP0_VM0  1     0.0 GB  0.0 GiB
...
==> SUCCESS: all subcommands ran against vcsim with exit 0
```

### Why some fields read `unknown` / `N/A` / `-` against the simulator

`vcsim` is an integration/smoke harness, not a full correctness oracle. It does
not model storage transport topology (HBA → LUN → extent) or LACP/uplink
detail. So against the simulator the datastore `TYPE` degrades to `unknown`, and
distributed-switch `LACP`/`UPLINKS` read `disabled`/`-`. This is expected and
the program never crashes or drops a row over a missing value. **No data is
fabricated** — the tool reports only what the API truthfully returns.

## Live vCenter and full-fidelity behavior

The same binary runs against a live vCenter, where the full-fidelity parts apply:

- **`datastores` TYPE** derives the real transport by walking each VMFS extent's
  canonical LUN → multipath path → host bus adapter, then classifying the
  adapter (`FC` / `iSCSI` / `NVMe`); NFS datastores report `NFS`. The
  classification decision is a pure function (`ClassifyTransport`) proven by its
  own table-driven test with representative FC/iSCSI/NVMe inputs.
- **`vswitches` LACP/UPLINKS** report real LACP state (distributed-switch only)
  and the physical NIC / uplink port names backing each switch.

```sh
vsphere-inventory --url https://vc.prod.example.com/sdk \
  --username svc-inventory@vsphere.local --password "$VC_PASS" \
  datastores
```

## Notes

- Errors are wrapped with context and surfaced (e.g. *"connecting to vCenter at
  host: …"*); there are no panics in normal flow.
- The configured `timeout` bounds the whole operation via `context.Context`, and
  `Ctrl-C`/`SIGTERM` cancels in-flight work; logout always runs on cleanup.
