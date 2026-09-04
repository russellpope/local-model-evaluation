# vsphere-inventory

A single Go CLI that connects to a VMware vCenter and reports virtualization
inventory: virtual machines, datastores (with real backing transport), and
virtual switches (standard + distributed) with portgroup drill-down.

Built with [`govmomi`](https://github.com/vmware/govmomi),
[`cobra`](https://github.com/spf13/cobra), and
[`viper`](https://github.com/spf13/viper); all tables rendered with the
standard library's `text/tabwriter`.

## Requirements

- Go 1.22+ (developed and verified on Go 1.27; `go.mod` requires ≥ 1.22).
- Network access to the Go module proxy the first time you build (or a
  populated module cache).
- Optional: a vCenter, or the bundled `vcsim` simulator for verification.

## Layout

```
.
├── go.mod / go.sum
├── Makefile                    # build / test / vet / verify targets
├── main.go                     # entry point; signal handling, exit code
├── cmd/                        # Cobra wiring + tabwriter presentation
│   ├── root.go                 # root command, shared flags, connect helper
│   ├── vms.go                  # `vms`
│   ├── datastores.go           # `datastores`
│   └── vswitches.go            # `vswitches` (+ --portgroup)
├── internal/
│   ├── client/
│   │   ├── client.go           # endpoint parsing, authenticated connect, logout,
│   │   │                       #   actionable auth/TLS/connection error wrapping
│   │   └── client_test.go
│   ├── config/
│   │   ├── config.go           # Viper: flag > VSPHERE_* env > YAML file > default
│   │   └── config_test.go      # precedence proven per layer
│   ├── format/
│   │   ├── format.go           # GiB/TiB human bytes, GB memory, used = total − avail
│   │   └── format_test.go
│   └── inventory/              # typed queries: (ctx, client) -> []VMInfo/... ; no Cobra, no output
│       ├── inventory.go        # container-view + property-collector helpers
│       ├── vms.go              # ListVMs (committed storage via summary.storage)
│       ├── datastores.go       # ListDatastores
│       ├── transport.go        # HBA/extent -> FC|iSCSI|NVMe|NFS classifier + resolution
│       ├── vswitches.go        # ListSwitches (standard + distributed), LACP, VLAN, ports
│       ├── portgroup.go        # VMsOnPortgroup (standard and distributed)
│       ├── transport_test.go   # pure classifier + VLAN/port/pnic tables
│       ├── inventory_sim_test.go   # govmomi/simulator feature tests
│       └── portgroup_sim_test.go
├── scripts/verify.sh           # the full check, incl. live vcsim run
├── examples/config.yaml        # example YAML config
└── govmomi-cli-eval-prompt.md  # task spec (input to this repo)
```

## Build

```sh
go mod tidy     # refresh dependencies (requires network the first time)
go build ./...  # or: make build  -> ./vsphere-inventory
go vet ./...    # clean
```

## Configuration

All subcommands share one connection config, resolved through Viper with
precedence **flag > environment > config file > built-in default**:

| Key      | Flag          | Env var             | Default |
|----------|---------------|---------------------|---------|
| url      | `--url`       | `VSPHERE_URL`       | — (required) |
| username | `--username`  | `VSPHERE_USERNAME`  | — |
| password | `--password`  | `VSPHERE_PASSWORD`  | — |
| insecure | `--insecure`  | `VSPHERE_INSECURE`  | `false` |
| timeout  | `--timeout`   | `VSPHERE_TIMEOUT`   | `60s`   |

`--url` accepts a full SDK URL (`https://vc.lab/sdk`) or a bare
host (`vc.lab` → `https://vc.lab/sdk`). The operation timeout bounds a
single derived `context.Context` used for login, every query, and logout.

### Example: YAML config file

```yaml
# examples/config.yaml
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: Secret123!
insecure: true
timeout: 60s
```

```sh
./vsphere-inventory vms --config examples/config.yaml
```

### Example: environment variables

```sh
export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD='Secret123!'
export VSPHERE_INSECURE=true
./vsphere-inventory vms
./vsphere-inventory datastores
./vsphere-inventory vswitches
./vsphere-inventory vswitches --portgroup "VM Network"   # VMs on a port group
```

## What each report shows

- `vms` — NAME, VCPU, RAM (GB), STORAGE. STORAGE is the **committed**
  (actually consumed) bytes from `summary.storage.committed`, not provisioned
  capacity. Missing values render as `unknown`, never dropped.
- `datastores` — NAME, TYPE, USED, AVAILABLE. TYPE is the real storage
  **transport** (`FC`/`iSCSI`/`NVMe`/`NFS`), derived from the datastore's
  VMFS extents → host SCSI LUN / NVMe namespace → host bus adapter type
  (`internal/inventory/transport.go`). NFS is the one transport equal to the
  filesystem type. The decision logic is a pure function proven by a
  table-driven test.
- `vswitches` — one row per portgroup of every **standard** (host vSwitch)
  and **distributed** (vDS) switch: name, type, VLAN (single ID, trunk
  range, or `private-vlan:N`), uplinks, LACP, total ports, used ports.
  Standard switch used ports = `total − available` from vSwitch runtime
  state; LACP is `N/A` on standard switches (it is a distributed-switch
  feature; on vCenter 8 VMware DVS it is reported from `lacpGroupConfig`).
- `vswitches --portgroup <name>` — lists VMs (NAME/VCPU/RAM/STORAGE) whose
  virtual NICs are attached to that portgroup, for standard and distributed
  portgroups alike. Unknown names produce an error, not an empty table.

## Simulator fidelity (read before trusting a green `make verify`)

`vcsim` is a smoke/integration harness, not a correctness oracle:

- It models no HBA → LUN → extent storage topology (its VMFS extents are
  placeholder names), so `datastores` TYPE legitimately reports `unknown`
  there. The classification logic itself is proven by its pure-function test.
- It creates the DVS without uplink port policy / LACP groups, so UPLINKS
  reports `unknown` and LACP reports `disabled` (correct for a switch with
  no LACP group configured).
- Its model VMs carry minimal hardware config (e.g. 32 MiB RAM, near-zero
  committed bytes), which is why the samples below show `0.0 GB` /
  `0.0 GiB`.
- Against a **live vCenter** the same code paths report real transport,
  LACP, and uplink state. Nothing in this code is vcsim-specific.

## Verification — actually run, not just compiled

### Unit tests (`go test ./...`, hermetic in-process `govmomi/simulator`)

```
$ go test ./... -count=1
ok      local-model-evaluation/govmomi-cli/internal/client      1.134s
ok      local-model-evaluation/govmomi-cli/internal/config      1.161s
ok      local-model-evaluation/govmomi-cli/internal/format      0.922s
ok      local-model-evaluation/govmomi-cli/internal/inventory   3.984s
```

Coverage: VMs (exact model count, non-empty names, vCPU/RAM > 0, storage ≥ 0),
datastores (name/capacity consistency, transport domain), vswitches
(standard + distributed rows, VLAN parses, used ≤ total, LACP domain),
portgroup → VMs (exact set on a distributed portgroup; exact single VM after
attaching a NIC to a standard portgroup via the real API), config precedence
(each of the four layers), byte formatting and used-math tables, transport
classifier table, endpoint parsing, login/logout, bad-credential and
unreachable-host error wrapping, and context-timeout honoring. No skips.

### End-to-end against vcsim (`make verify`)

`make verify` runs `go vet`, `go test`, builds the binary, starts `vcsim`
(`go run github.com/vmware/govmomi/vcsim`, pinned in `go.mod` as a Go
`tool` — the binary never imports it), waits for readiness, drives every
subcommand plus a `--portgroup` invocation for both a distributed and a
standard portgroup discovered from its own output, and tears the simulator
down (`scripts/verify.sh` exits non-zero on any failure).

```
$ make verify
==> go vet ./...
==> go test ./...
...
==> starting vcsim on 127.0.0.1:18989
==> vms
==> datastores
==> vswitches
==> vswitches --portgroup DC0_DVPG0
==> vswitches --portgroup 'Management Network' (standard)
verify: OK
```

Sample output (`vcsim -vm 8 -ds 3 -pg 3`, all exits 0):

```
$ ./vsphere-inventory vms
NAME             VCPU   RAM      STORAGE
DC0_C0_RP0_VM0   1      0.0 GB   0.0 GiB
DC0_C0_RP0_VM1   1      0.0 GB   0.0 GiB
...

$ ./vsphere-inventory datastores
NAME        TYPE      USED        AVAILABLE
LocalDS_0   unknown   160.0 GiB   3.8 TiB
LocalDS_1   unknown   0.0 GiB     4.0 TiB
LocalDS_2   unknown   0.0 GiB     4.0 TiB

$ ./vsphere-inventory vswitches
SWITCH     SWITCH TYPE   PORTGROUP            VLAN     UPLINKS   LACP       PORTS   USED
DVS0       distributed   DC0_DVPG0            0        unknown   disabled   1       16
DVS0       distributed   DC0_DVPG1            0        unknown   disabled   1       0
DVS0       distributed   DC0_DVPG2            0        unknown   disabled   1       0
DVS0       distributed   DVS0-DVUplinks-8     0-4094   unknown   disabled   1       0
vSwitch0   standard      Management Network   0        vmnic0    N/A        1536    6
vSwitch0   standard      VM Network           0        vmnic0    N/A        1536    6

$ ./vsphere-inventory vswitches --portgroup DC0_DVPG0 | head -3
NAME             VCPU   RAM      STORAGE
DC0_C0_RP0_VM0   1      0.0 GB   0.0 GiB
DC0_C0_RP0_VM1   1      0.0 GB   0.0 GiB

$ ./vsphere-inventory vswitches --portgroup nope
error: portgroup "nope" not found in the inventory
exit status 1
```

Note: `DC0_DVPG0` shows `USED 16 > PORTS 1` because the simulator attached
more VMs to the portgroup than its configured port count (it performs no
admission control); the numbers truthfully mirror what the API returns. The
hermetic test widens the portgroup through the real `ReconfigureDVPortgroup`
API and then asserts `used ≤ total` as a genuine invariant.

## Dependency notes

Direct dependencies are only `github.com/vmware/govmomi`,
`github.com/spf13/cobra`, `github.com/spf13/viper`, and the Go standard
library. In recent govmomi releases `vcsim` (the CLI harness) lives in its
own nested module; it is pinned in `go.mod` under the `tool` directive so
`go run github.com/vmware/govmomi/vcsim` works at the exact same version as
the library, and it is never imported by the binary itself. `go mod tidy`
keeps the tool pin (Go ≥ 1.24).

## Deploying against a live vCenter

Point the same binary at real infrastructure — nothing changes except the
config:

```sh
export VSPHERE_URL=https://vc.corp.example/sdk VSPHERE_INSECURE=false
export VSPHERE_USERNAME=svc-inventory@vsphere.local VSPHERE_PASSWORD='…'
./vsphere-inventory datastores   # now reports FC / iSCSI / NVMe / NFS
./vsphere-inventory vswitches    # now reports real uplinks and LACP state
```
