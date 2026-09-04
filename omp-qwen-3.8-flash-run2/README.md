# govc-inventory

Single-binary vSphere inventory CLI built with
[govmomi](https://github.com/vmware/govmomi),
[cobra](https://github.com/spf13/cobra), and
[viper](https://github.com/spf13/viper). Reports virtual machines, datastores
(with real storage transport), and virtual switches (standard + distributed)
from a vCenter Server. Table output uses `text/tabwriter` — plain, greppable
text, no color.

## Subcommands

| Command      | Output                                                                      |
|--------------|-----------------------------------------------------------------------------|
| `vms`        | NAME, VCPU, RAM (GiB), STORAGE — *consumed/committed* bytes, not provisioned |
| `datastores` | NAME, TYPE (FC/iSCSI/NVMe/NFS), USED, AVAILABLE                              |
| `vswitches`  | SWITCH, SWITCH TYPE, PORTGROUP, VLAN, UPLINKS, LACP, PORTS, USED             |
| `vswitches --portgroup <name>` | VMs connected to that standard *or* distributed port group |

All tables are sorted by the first column (switch table by switch, then
port group).

## Configuration

Shared connection settings resolve with precedence
**flag > env (`VSPHERE_*`) > YAML config file > built-in default**:

| Key      | Flag         | Env var            | Default |
|----------|--------------|--------------------|---------|
| url      | `--url`      | `VSPHERE_URL`      | —       |
| username | `--username` | `VSPHERE_USERNAME` | —       |
| password | `--password` | `VSPHERE_PASSWORD` | —       |
| insecure | `--insecure` | `VSPHERE_INSECURE` | `false` |
| timeout  | `--timeout`  | `VSPHERE_TIMEOUT`  | `60s`   |
| config   | `--config`   | —                  | —       |

`--url` accepts a bare host (`vc.lab` → `https://vc.lab/sdk`) or a full SDK
URL. The overall timeout bounds the entire operation via `context.Context`.

## Build

Requires Go 1.25+ (the current govmomi module floor; satisfies the spec's
Go 1.22+ target). Dependencies are fetched automatically; `go mod tidy` is
safe to run — `tools.go` pins the `vcsim` command so tidy keeps it.

```
go build -o bin/govc-inventory .
```

Note on versions: `github.com/vmware/govmomi/vcsim` split into its own nested
module at the same commit as the govmomi pseudo-version pinned here
(`v0.0.0-20260902050809-f0d835f384b5`). Both must move together or the vcsim
binary fails to compile against govmomi internals.

## Run against a live vCenter

```
export VSPHERE_URL=https://vcenter.lab.example/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD='secret'
export VSPHERE_INSECURE=false   # vCenter with a public CA chain
./bin/govc-inventory vms
```

or with a file:

```
./bin/govc-inventory datastores --config config.example.yaml
```

Against a real vCenter the datastore `TYPE` column derives the actual
transport (FC / iSCSI / NVMe) from host HBA → LUN → VMFS extent topology, and
distributed switch `LACP` reflects the switch's LACP capability plus
configured LACP groups. See "Simulator fidelity" below.

## Run against the bundled simulator (`vcsim`)

```
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3
# endpoint: https://127.0.0.1:8989/sdk  user/pass: user/pass (self-signed TLS)

export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

./bin/govc-inventory vms
./bin/govc-inventory datastores
./bin/govc-inventory vswitches
./bin/govc-inventory vswitches --portgroup DC0_DVPG0   # discover from the table above
```

## Verify

`make verify` runs the whole loop unattended: `go vet`, `go test ./...`
(hermetic simulator tests via `github.com/vmware/govmomi/simulator`), build,
start `vcsim`, wait for readiness, run all three subcommands plus a
discovered `--portgroup` lookup, assert config precedence and error
behaviour, then tear the simulator down. Non-zero exit on any failure.

```
make verify
```

## Simulator fidelity

`vcsim` is an integration/smoke harness, not a correctness oracle:

- It does not model storage transport topology (HBA → LUN → extent), so
  `datastores` `TYPE` legitimately degrades to `unknown` (VMFS volumes report
  `mpx.vmhba0` local-disk extents) — never a crash or a fabricated value.
  The FC/iSCSI/NVMe decision itself lives in pure, table-tested functions
  (`ClassifyHBA`, `ClassifyCanonicalName`, `ClassifyScsiLun`,
  `ClassifyNVMeController`, `hostStorage.deviceTransport`).
- Its vCenter DVS objects carry no LACP capability/groups and its DVPGs carry
  no pNIC bindings, so `LACP` reports `N/A` and `UPLINKS` `-`; standard
  vSwitches always report `N/A` for LACP (a vDS-only feature).
- The simulator's model VMs are 1 vCPU / 32 MiB / 234 B committed — small
  values rendered as `0.0 GiB` are the real API numbers, not a bug.

## Layout

```
.
├── main.go                      # entry: signal ctx → cli.NewRootCmd()
├── tools.go                     # pins vcsim so `go mod tidy` keeps the sim
├── config.example.yaml          # example file-config
├── Makefile                     # build | vet | test | verify | sim
├── scripts/verify.sh            # automated build→run→diagnose loop
└── internal/
    ├── cli/                     # cobra wiring + tabwriter presentation
    ├── config/                  # viper precedence (flag > env > file > default)
    ├── format/                  # GiB/TiB humanising, used = total − available
    └── vsphere/                 # collectors: ListVMs, ListDatastores,
                                 # ListSwitches, VMsOnPortGroup + transport
                                 # classifier (pure, table-tested)
```

## Tests

`go test ./...` covers each feature with the embedded `simulator` package and
each pure helper with table-driven tests — config precedence, byte
formatting, transport classification (representative FC/iSCSI/NVMe inputs),
VLAN rendering, URL normalisation, and table shape. Zero skips, zero failures
is the bar; `make verify` enforces it.

## Run evidence (this code was actually executed)

`go test ./... -count=1` (go 1.27.0, darwin/arm64):

```
?   	github.com/example/govc-inventory	[no test files]
ok  	github.com/example/govc-inventory/internal/cli	0.783s
ok  	github.com/example/govc-inventory/internal/config	0.326s
ok  	github.com/example/govc-inventory/internal/format	0.540s
ok  	github.com/example/govc-inventory/internal/vsphere	2.169s
```

`make verify` against `vcsim -vm 8 -ds 3 -pg 3` (trimmed):

```
=== go vet ===
=== go test ===          (all packages ok, zero skips)
=== build ===
=== start vcsim on 127.0.0.1:18989 ===
vcsim ready (pid 75509)
=== vms ===        -- vms OK
=== datastores === -- datastores OK
=== vswitches ===  -- vswitches OK
=== vswitches --portgroup DC0_DVPG0 ===
NAME            VCPU  RAM      STORAGE
DC0_C0_RP0_VM0  1     0.0 GiB  0.0 GiB
...
-- portgroup OK (16 VMs)
=== unknown portgroup is a clean error ===  -- OK
=== config file + env + flag precedence smoke ===  -- config OK
=== error handling: unreachable endpoint yields wrapped error ===  -- OK
ALL CHECKS PASSED
```
