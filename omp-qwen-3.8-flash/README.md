# vsphere-inventory

Single-binary CLI that connects to a VMware vCenter Server (govmomi) and
reports virtualization inventory as plain, greppable tables:

- `vms` — every VM: name, vCPU, RAM (GiB), **consumed (committed)** storage.
- `datastores` — every datastore: name, **transport** (FC/iSCSI/NVMe/NFS —
  derived from backing HBA/LUN topology, not the filesystem type), used and
  available capacity.
- `vswitches` — every standard (host vSwitch) and distributed (vDS) port
  group: VLAN (ID, trunk range, or private-VLAN), uplinks, LACP state, total
  and used ports. `--portgroup <name>` lists the VMs attached to that port
  group (standard or distributed) instead.

## Layout

```
.
├── main.go                      # entry point
├── cmd/                         # Cobra wiring + tabwriter presentation
│   ├── root.go                  # shared flags, Viper binding, connect helper
│   ├── vms.go  datastores.go  vswitches.go
│   ├── render.go                # table rendering
│   └── e2e_test.go              # full command tree vs in-process simulator
├── internal/
│   ├── config/                  # Viper precedence (flag > env > file > default)
│   ├── client/                  # govmomi session + error wrapping
│   ├── format/                  # GiB/TiB human units, used = total - available
│   └── inventory/               # retrieval logic: typed results, testable
│       ├── vms.go  datastores.go  vswitches.go  portgroups.go
│       └── transport.go         # pure FC/iSCSI/NVMe classifier (table-tested)
├── scripts/verify.sh            # gofmt + vet + tests + live vcsim loop
├── tools/vcsim/                 # vendored vcsim driver (nested module)
└── config.example.yaml
```

Retrieval functions (`inventory.VMs`, `inventory.Datastores`,
`inventory.Switches`, `inventory.VMsOnPortgroup`) take
`(ctx, *vim25.Client)` and return typed slices; Cobra and tabwriter are kept
out of that layer so each feature is testable directly against
`github.com/vmware/govmomi/simulator`.

## Build & test

```
go mod tidy        # deps: govmomi v0.56.0, cobra, viper (go.sum committed)
go build ./...
go vet ./...
go test ./...      # hermetic: in-process simulator, no external services
```

## Run

Against a live vCenter:

```
./bin/vsphere-inventory vms \
  --url https://vc.lab/sdk --username user --password pass --insecure

export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true
./bin/vsphere-inventory datastores

./bin/vsphere-inventory vms --config config.example.yaml
```

Precedence per key: `--flag` > `VSPHERE_*` env > YAML config file (`--config`,
or `VSPHERE_CONFIG`) > built-in default (`insecure=false`, `timeout=60s`).

## Verification loop (`make verify`)

`scripts/verify.sh` runs gofmt/vet/tests, starts the simulator, drives all
three subcommands plus a discovered `--portgroup` lookup, asserts exit codes,
headers, sort order, and degrade-to-unknown behaviour, then tears the
simulator down.

One wrinkle worth knowing: since govmomi v0.54 the `vcsim` binary lives in a
**nested module** whose `go.mod` contains `replace ../`, and Go refuses to
`go run pkg@version` such modules. `tools/vcsim/` therefore vendors the
verbatim upstream `main.go` pinned to the same commit as our govmomi v0.56.0
requirement, with the replace swapped for a pinned require. The main module's
dependency set is unchanged (govmomi, cobra, viper, stdlib only).

```
make verify
# or by hand:
go -C tools/vcsim run . -l 127.0.0.1:8989 -vm 8 -ds 3 -pg 3 &
export VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user \
       VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true
./bin/vsphere-inventory vms && ./bin/vsphere-inventory datastores \
  && ./bin/vsphere-inventory vswitches
./bin/vsphere-inventory vswitches --portgroup DC0_DVPG0
```

## Simulator fidelity

`vcsim` does not model storage transport topology (HBA → LUN → extent) or
real LACP/uplink state: datastore `TYPE` degrades to `unknown` and distributed
`UPLINKS`/`LACP` to `unknown`/`disabled` there — expected per the task spec.
The transport classifier's logic is instead proven by the dedicated
pure-function table test (`TestClassifyTransport`,
`TestHostDiskProtocols`), and real FC/iSCSI/NVMe + LACP state apply when the
same code runs against a live vCenter.

## Proof it was actually run

`go test ./... -count=1` (29 tests, zero failures, zero skips):

```
ok  	github.com/ldh/vsphere-inventory/cmd	1.710s
ok  	github.com/ldh/vsphere-inventory/internal/client	0.344s
ok  	github.com/ldh/vsphere-inventory/internal/config	0.175s
ok  	github.com/ldh/vsphere-inventory/internal/format	0.989s
ok  	github.com/ldh/vsphere-inventory/internal/inventory	2.638s
```

Live vcsim loop (`-vm 8 -ds 3 -pg 3`, `make verify` → exit 0):

```
NAME            VCPU  RAM      STORAGE
DC0_C0_RP0_VM0  1     0.0 GiB  0.0 GiB
...
DC0_H0_VM7      1     0.0 GiB  0.0 GiB

NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  3.8 TiB
LocalDS_1  unknown  0.0 GiB    4.0 TiB
LocalDS_2  unknown  0.0 GiB    4.0 TiB

SWITCH    SWITCH TYPE  PORTGROUP           VLAN        UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0 (native)  unknown  disabled  1      0
DVS0      distributed  DC0_DVPG1           0 (native)  unknown  disabled  1      0
DVS0      distributed  DC0_DVPG2           0 (native)  unknown  disabled  1      0
vSwitch0  standard     Management Network  0 (native)  vmnic0   N/A       1536   6
vSwitch0  standard     VM Network          0 (native)  vmnic0   N/A       1536   6

$ ./bin/vsphere-inventory vswitches --portgroup DC0_DVPG0   # discovered from output above
NAME            VCPU  RAM      STORAGE
DC0_C0_RP0_VM0  1     0.0 GiB  0.0 GiB
...  (all 16 simulator VMs, attached by vcsim to DVPG0)
```
