# vsphere-inventory

A single Go CLI that connects to a vCenter (or standalone ESXi) and reports
inventory as aligned, greppable tables:

- **`vms`** — every virtual machine with vCPU, RAM and committed storage.
- **`datastores`** — every datastore with its backing transport, used and
  available capacity.
- **`vswitches`** — every standard and distributed virtual switch with its
  port groups (VLAN, uplinks, LACP, ports, used); or, with
  **`--portgroup <name>`**, the VMs connected to that port group.

Built on [govmomi](https://github.com/vmware/govmomi), Cobra and Viper.

## Requirements

- Go 1.22 or newer.
- A reachable vCenter/ESXi endpoint (or the bundled simulator for local
  verification — see [Verifying against vcsim](#verifying-against-vcsim)).

## Build

```sh
make build            # -> bin/vsphere-inventory
# or
go build -o bin/vsphere-inventory .
```

## Configuration

Connection settings are resolved with the precedence
**flag > environment variable > config file > default**:

| Setting    | Flag         | Environment variable | Config file key | Default |
|------------|--------------|----------------------|-----------------|---------|
| URL        | `--url`      | `VSPHERE_URL`        | `url`           | —       |
| Username   | `--username` | `VSPHERE_USERNAME`   | `username`      | —       |
| Password   | `--password` | `VSPHERE_PASSWORD`   | `password`      | —       |
| Insecure   | `--insecure` | `VSPHERE_INSECURE`   | `insecure`      | `false` |
| Timeout    | `--timeout`  | `VSPHERE_TIMEOUT`    | `timeout`       | `60s`   |

A YAML config file is optional: `cp config.example.yaml config.yaml`, then
`--config config.yaml`. Credentials may also be embedded in the URL as
`user:pass@host`.

## Usage

```sh
export VSPHERE_URL=https://vcenter.example.com/sdk
export VSPHERE_USERNAME='administrator@vsphere.local'
export VSPHERE_PASSWORD=secret
export VSPHERE_INSECURE=true        # self-signed cert

./bin/vsphere-inventory vms
./bin/vsphere-inventory datastores
./bin/vsphere-inventory vswitches
./bin/vsphere-inventory vswitches --portgroup "VM Network"
```

### `vms`

```
NAME            VCPU  RAM        STORAGE
DC0_C0_RP0_VM0  4     8.0 GiB    12.3 GiB
...
```

`STORAGE` is the **committed** (actually consumed) storage reported by the
server, not provisioned/allocated capacity. Rows are sorted by name.

### `datastores`

```
NAME       TYPE     USED        AVAILABLE
LocalDS_0  FC       40.0 GiB    4.0 TiB
nfs-1      NFS      1.2 TiB     2.8 TiB
...
```

`TYPE` is the backing **transport** (one of `FC`, `iSCSI`, `NVMe`, `NFS`)
derived from the datastore's storage devices and host bus adapters — not the
filesystem type. A datastore whose transport cannot be determined from the API
reports `unknown`. `USED` is `capacity - available`. Rows are sorted by name.

### `vswitches`

```
SWITCH  SWITCH TYPE  PORTGROUP   VLAN    UPLINKS  LACP      PORTS  USED
DVS0    distributed  VM Network  5       vmnic0   disabled  128    7
vsw0    standard     Management  0       vmnic0   N/A       128    3
```

LACP applies to distributed switches only; standard vSwitches report `N/A`.
VLANs may be a single id, a trunk range (`1-5,100-200`) or a type
(`trunk`, `pvlan 12`). Rows are sorted by switch name, then port group.

### `vswitches --portgroup <name>`

Prints only the VMs connected to the named port group (works for both
standard and distributed port groups):

```
NAME
DC0_C0_RP0_VM0
DC0_H0_VM1
```

## Testing

Unit tests use govmomi's embedded `simulator` package (no external process).
They cover config precedence, byte formatting, the transport classifier, and
each inventory feature against a synthetic model.

```sh
make test             # go test ./...
make vet              # go vet ./...
make fmt-check        # fail if not gofmt-clean
```

## Verifying against vcsim

`make verify` (or `sh scripts/verify.sh`) is the end-to-end gate. It:

1. Runs `go vet`, a gofmt check, `go test ./...`, and builds the CLI.
2. Builds and starts a local vcsim endpoint.
3. Runs `vms`, `datastores`, `vswitches`, then
   `vswitches --portgroup <name>` using a port group discovered from the
   `vswitches` output.
4. Fails if any step errors.

```sh
make verify
```

### About the local simulator

The simulator is govmomi's bundled `simulator` package, run at the **same
govmomi version** the CLI depends on. It is started through
`tools/vcsimserver`, a small separate Go module, because the standalone
`github.com/vmware/govmomi/vcsim` module cannot be `go run` as an external
dependency (its `go.mod` uses a `replace` directive). `tools/vcsimserver`
serves plain HTTP and is used **only** for local verification — it is not part
of the CLI binary.

The simulator is an integration/smoke harness, not a correctness oracle. It
does not richly model storage transport topology (HBA → LUN → extent) or
LACP/uplink detail, so against the simulator the datastore `TYPE` and the
`LACP`/`UPLINKS` columns legitimately report `unknown`/`N/A`, and small
synthetic values (e.g. a 32 MiB VM) round to `0.0 GiB`. These are faithful,
graceful degradations — the tool never fabricates a value or drops a row.
Real `FC`/`iSCSI`/`NVMe` and real LACP state are the spec for a live vCenter.

## Project layout

```
main.go                        entrypoint
cmd/                           Cobra commands + tabwriter presentation
internal/config/               flag > env > file > default resolution
internal/format/               GiB/TiB byte formatting + used-capacity math
internal/inventory/            inventory retrieval (vms, datastores, vswitches)
tools/vcsimserver/             separate module: local vcsim launcher (verify only)
scripts/verify.sh              the `make verify` gate
```
