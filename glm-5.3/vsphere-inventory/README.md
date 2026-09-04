# vsphere-inventory

A single Go binary that connects to a VMware vCenter Server and reports
virtualization inventory:

- `vms` — all virtual machines (vCPU, RAM in GB, **consumed** storage in GiB/TiB)
- `datastores` — all datastores with the underlying **transport** (FC, iSCSI, NVMe, NFS), used and available capacity
- `vswitches` — all virtual switches, **standard and distributed**, with their port groups, VLANs, uplinks, LACP state and port usage; `--portgroup <name>` lists the VMs connected to a port group instead

Output is plain, greppable, `text/tabwriter`-aligned text.

## Project layout

```
vsphere-inventory/
├── main.go                       # entry point
├── cmd/                          # cobra command wiring
│   ├── root.go                   # root command, flags, connect/logout
│   ├── vms.go
│   ├── datastores.go
│   └── vswitches.go
├── internal/
│   ├── config/                   # viper-based configuration + precedence tests
│   ├── format/                   # human-readable byte/GB formatting + tests
│   ├── inventory/                # inventory retrieval, testable against the
│   │                             # embedded simulator
│   └── render/                   # tabwriter presentation
├── config.example.yaml
├── Makefile                      # make verify = full check incl. vcsim loop
├── go.mod / go.sum
```

Direct dependencies (only these): `github.com/vmware/govmomi`,
`github.com/spf13/cobra`, `github.com/spf13/viper`, plus the Go standard
library.

## Building

Requires Go 1.22+.

```
go mod tidy      # refresh go.sum if needed
go build -o bin/vsphere-inventory .
```

`go build ./...`, `go vet ./...` and `gofmt` must stay clean.

## Configuration

All subcommands share one connection configuration, resolved by viper with
this precedence (highest first):

1. command-line flag
2. environment variable (prefix `VSPHERE_`)
3. YAML config file (`--config`)
4. built-in default

| Key      | Flag         | Env var             | Default |
|----------|--------------|---------------------|---------|
| url      | `--url`      | `VSPHERE_URL`       | — (required) |
| username | `--username` | `VSPHERE_USERNAME`  | — (required) |
| password | `--password` | `VSPHERE_PASSWORD`  | — |
| insecure | `--insecure` | `VSPHERE_INSECURE`  | `false` |
| timeout  | `--timeout`  | `VSPHERE_TIMEOUT`   | `60s` |
| config   | `--config`   | —                   | — |

Example config file (see `config.example.yaml`):

```yaml
url: https://vcenter.lab.example.com/sdk
username: administrator@vsphere.local
password: secret
insecure: false
timeout: 60s
```

Environment-variable example:

```
export VSPHERE_URL=https://vcenter.lab.example.com/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD=secret
export VSPHERE_INSECURE=true
export VSPHERE_TIMEOUT=90s
./bin/vsphere-inventory vms
```

Flag example (overrides env and file):

```
./bin/vsphere-inventory --url https://vc2.lab/sdk --username admin --insecure datastores
./bin/vsphere-inventory --config config.example.yaml vswitches
```

One authenticated client is established per invocation using a context
derived from the configured timeout; logout is deferred and runs on a short
context independent of the request timeout.

## Running against the bundled vCenter simulator (vcsim)

The [vcsim] simulator ships inside the govmomi module pinned in `go.mod`
(v0.46.1 — the last release with vcsim embedded in the module), so it runs
at the same version as the client library with no extra dependency:

```
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3
# defaults: https://127.0.0.1:8989/sdk, user/pass, self-signed TLS

export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

./bin/vsphere-inventory vms
./bin/vsphere-inventory datastores
./bin/vsphere-inventory vswitches
# discover a port group name from the output above, then:
./bin/vsphere-inventory vswitches --portgroup "DC0_DVPG0"
```

Simulator fidelity notes (expected, by design):

- vcsim local datastores have no fabric topology, so `TYPE` degrades to
  `unknown`; NFS-backed datastores report `NFS`. The FC/iSCSI/NVMe
  derivation itself is proven by the pure-function tests
  (`internal/inventory/transport_test.go`).
- vcsim does not populate DVS `config.uplinkPortgroup`, LACP groups, port
  connectees, or per-VM committed storage, so `UPLINKS` shows `unknown`,
  LACP shows `disabled`, distributed `USED` is 0, and VM `STORAGE` is
  `0.0 GiB`. On a live vCenter these fields carry real values.

## Testing

Unit tests are hermetic and run against govmomi's embedded `simulator`
package (no external vCenter needed):

```
go test ./...
```

Coverage:

- `internal/config` — flag > env > config file > default precedence; error cases
- `internal/format` — byte formatting (GiB/TiB, one decimal), used = total − available
- `internal/inventory` —
  - VMs: known model count, sane per-VM values, sort order
  - datastores: capacity math (used + available = capacity), transport vocabulary
  - vSwitches: both switch kinds, VLAN values parse, used ≤ total ports, LACP states
  - port group → VMs: distributed lookup returns the known attached set; a VM is reconfigured onto a standard port group and the lookup returns exactly it; unknown names error
  - pure helpers: transport classifier (FC/iSCSI/NVMe table tests), VLAN formatting/parsing

## Full verification

```
make verify
```

runs `go vet ./...`, a `gofmt` check, `go test ./...`, builds the binary,
starts vcsim in the background, waits for it to be ready, exercises all
three subcommands plus a `--portgroup` invocation (port group name
discovered from the `vswitches` output), and tears the simulator down.
Exits non-zero on any failure.

## Live vCenter

The same binary is deployable against a real vCenter:

```
./bin/vsphere-inventory --url https://vcenter/sdk --username 'administrator@vsphere.local' --password '***' --insecure datastores
```

On live vCenters the full-fidelity behavior applies: datastore `TYPE` is
derived from the actual backing devices/HBAs (FC/iSCSI/NVMe via the host
storage topology — extents → SCSI disk → multipath paths → HBA type, or the
NVMe topology namespaces), and `LACP` reflects the DVS LACP group
configuration.

[vcsim]: https://github.com/vmware/govmomi/blob/v0.46.1/vcsim/README.md
