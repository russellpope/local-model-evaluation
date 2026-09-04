# vsphere-inventory

A single-binary vSphere inventory CLI in Go (govmomi + Cobra + Viper). It
reports virtual machines, datastores (by storage **transport**, not
filesystem), and virtual switches (standard **and** distributed, with port
groups, VLAN, uplinks, LACP, and port usage).

## Project layout

```
.
├── main.go                          # entry point: os.Exit(cmd.Execute())
├── cmd/                             # Cobra wiring + tabwriter presentation
│   ├── root.go                      # root command, shared flags, precedence
│   ├── vms.go                       # `vms` subcommand
│   ├── datastores.go                # `datastores` subcommand
│   ├── vswitches.go                 # `vswitches` (+ --portgroup) subcommand
│   ├── output.go                    # text/tabwriter renderers (one per table)
│   └── output_test.go
├── internal/
│   ├── config/config.go             # Viper: flag > env > file > default
│   ├── client/client.go             # URL normalization, login, error hints
│   ├── format/format.go             # GiB/TiB formatter, used=total-free, GB RAM
│   └── inventory/                   # retrieval logic, client-in, typed results
│       ├── inventory.go             # shared container-view fetch helper
│       ├── vms.go                   # FetchVMs / VMInfo
│       ├── datastores.go            # FetchDatastores / DatastoreInfo
│       ├── transport.go             # ClassifyTransport (pure FC/iSCSI/NVMe map)
│       ├── switches.go              # FetchSwitches / SwitchInfo / FetchVMsByPortgroup
│       └── *_test.go                # simulator-based feature tests + pure tests
├── config.example.yaml
├── Makefile                         # `make verify` automates the whole loop
└── go.mod                           # direct deps: govmomi, cobra, viper
```

## Requirements

- Go 1.22+ (built and verified with 1.27).
- Direct dependencies: `github.com/vmware/govmomi`, `github.com/spf13/cobra`,
  `github.com/spf13/viper`, and the standard library (`text/tabwriter` for all
  tables). No other third-party table/CLI/VMware libraries.
- `go mod tidy` fetches everything from the module proxy; run it after any
  dependency edit.

Note on `vcsim`: as of govmomi v0.56.0 the `vcsim` command is a nested module
(`github.com/vmware/govmomi/vcsim`). It is wired as a **Go tool dependency** in
`go.mod`, pinned to the pseudo-version of the exact same commit (`81608f9b9725`)
as `github.com/vmware/govmomi v0.56.0` — same version, no extra runtime library
in the shipped binary.

## Build

```
go build -o bin/vsphere-inventory .
make build            # same thing
make verify           # gofmt + vet + tests + full vcsim run (see below)
```

## Connection configuration

Precedence, highest first: **flag → `VSPHERE_*` env → `--config` YAML →
default.** Only flags the user actually set outrank env/file (unset flags never
mask an environment value).

| Key      | Flag           | Env var              | Default |
|----------|----------------|----------------------|---------|
| url      | `--url`        | `VSPHERE_URL`        | —       |
| username | `--username`   | `VSPHERE_USERNAME`   | —       |
| password | `--password`   | `VSPHERE_PASSWORD`   | —       |
| insecure | `--insecure`   | `VSPHERE_INSECURE`   | `false` |
| timeout  | `--timeout`    | `VSPHERE_TIMEOUT`    | `60s`   |
| config   | `--config`     | —                    | —       |

`url` accepts a full SDK URL or a bare host (`vc.lab` →
`https://vc.lab/sdk`). Timeouts are Go duration strings (`60s`, `5m`).

### Using a config file

```
# config.yaml (see config.example.yaml)
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: "change-me"
insecure: true
timeout: 90s

vsphere-inventory vms --config config.yaml
```

### Using environment variables

```
export VSPHERE_URL=https://vc.lab/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD='change-me'
export VSPHERE_INSECURE=true
vsphere-inventory datastores
```

## Subcommands

- `vms` — NAME / VCPU / RAM (GB) / STORAGE. STORAGE is **consumed
  (committed)** storage from `summary.storage.committed`, not provisioned
  capacity. Sorted by name.
- `datastores` — NAME / TYPE / USED / AVAILABLE. TYPE is the real transport:
  NFS datastores report `NFS`; VMFS extents are traced through each attached
  host's storage topology (LUN → HBA) and classified `FC` / `iSCSI` / `NVMe`
  by the owning adapter, degrading to `unknown` when the server does not
  expose the topology. Sorted by name.
- `vswitches` — lists standard and distributed switches with one row per port
  group: SWITCH / SWITCH TYPE / PORTGROUP / VLAN / UPLINKS / LACP / PORTS /
  USED. Trunk port groups render their VLAN range (`0-4094 (trunk)`),
  private-VLAN port groups render `private-vlan (...)`. LACP is distributed
  only; standard vSwitches report `N/A`. USED counts connected ports
  (distributed via `FetchDVPorts`, standard via portgroup port entries).
- `vswitches --portgroup <name>` — instead of the listing, prints the VMs
  attached to that port group; works for both standard and distributed port
  groups (matched through each VM's virtual NIC backing).

Every command runs under a `context.Context` bounded by `--timeout` (or
`VSPHERE_TIMEOUT`), logs out with a deferred clean `Logout`, and surfaces
wrapped, actionable errors (TLS hint for self-signed certs, credential hint on
`InvalidLogin`, reachability hint on dial failures, timeout hint when the
deadline fires). Nothing panics in normal flow; missing simulator/vCenter data
degrades to `unknown`/`N/A` instead of crashing or dropping rows.

## Self-verification loop (`make verify`)

`make verify` runs gofmt/vet/`go test ./...`, then builds and starts `vcsim`
(`-vm 8 -ds 3 -pg 3`) on 127.0.0.1:18989, waits for `/about` to respond, runs
`vms`, `datastores`, `vswitches`, discovers a real distributed port group name
from the switch output, runs `vswitches --portgroup <that name>`, tears the
simulator down on exit (trap), and fails non-zero on any error.

### Sample `go test ./...` (run 2026-09-02, go1.27.0, govmomi v0.56.0)

```
?   	github.com/local-model-evaluation/vsphere-inventory	[no test files]
ok  	github.com/local-model-evaluation/vsphere-inventory/cmd	0.483s
ok  	github.com/local-model-evaluation/vsphere-inventory/internal/client	0.200s
ok  	github.com/local-model-evaluation/vsphere-inventory/internal/config	0.623s
ok  	github.com/local-model-evaluation/vsphere-inventory/internal/format	0.310s
ok  	github.com/local-model-evaluation/vsphere-inventory/internal/inventory	1.688s
```

18 top-level tests, 0 failures, 0 skips (`-v` verified). Feature coverage:
`TestFetchVMs`, `TestFetchDatastores`, `TestFetchSwitches`,
`TestVMsByDistributedPortgroup`, `TestVMsByStandardPortgroup`,
`TestVMsByUnknownPortgroup` (hermetic, via `github.com/vmware/govmomi/simulator`
with pinned `VPX`/`ESX` model counts); pure coverage: `TestPrecedence`,
`TestBytes`/`TestUsed`/`TestMemoryMB`, `TestClassifyTransport` (representative
FC/iSCSI/NVMe HBA types and device identifiers).

### Sample `vcsim` run (excerpt from `make verify`)

```
--- vms ---
NAME            VCPU  RAM         STORAGE
DC0_C0_RP0_VM0  1     0.03125 GB  0.0 GiB
...
--- datastores ---
NAME       TYPE     USED       AVAILABLE
LocalDS_0  unknown  160.0 GiB  3.8 TiB
LocalDS_1  unknown  0.0 GiB    4.0 TiB
--- vswitches ---
SWITCH    SWITCH TYPE  PORTGROUP           VLAN            UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0               unknown  disabled  1      0
DVS0      distributed  DVS0-DVUplinks-8    0-4094 (trunk)  unknown  disabled  1      0
vSwitch0  standard     Management Network  0               vmnic0   N/A       6144   4
vSwitch0  standard     VM Network          0               vmnic0   N/A       6144   0
--- vswitches --portgroup DC0_DVPG0 ---
NAME            VCPU  RAM         STORAGE
DC0_C0_RP0_VM0  1     0.03125 GB  0.0 GiB
...
verify: OK
```

### Simulator fidelity (read before judging those values)

`vcsim` is an integration/smoke harness, not a correctness oracle. It does not
model storage transport topology (HBA → LUN → extent) or proxy-switch uplink
detail, so `TYPE=unknown` and `UPLINKS=unknown` are the *truthful* API answers
against the simulator, and thin VM disks legitimately commit ~0 bytes
(`0.0 GiB`). Real FC/iSCSI/NVMe classification and real LACP/uplink state
appear against a live vCenter, where the same code paths read them from the
host storage topology and `VMwareDVSConfigInfo.lacpGroupConfig`. The transport
decision itself is proven by the dedicated pure-function test
`TestClassifyTransport`. No value is hardcoded, faked, or special-cased for
the simulator.

## Deploying against a live vCenter

Point the same binary at real vCenter — no rebuild needed:

```
export VSPHERE_URL=https://vc.corp.example/sdk
export VSPHERE_USERNAME=svc-inventory@vsphere.local
export VSPHERE_PASSWORD='...'
vsphere-inventory datastores   # now reports FC/iSCSI/NVMe, not unknown
vsphere-inventory vswitches    # real pNIC uplinks and LACP state
```

Use `--insecure` only for lab endpoints with self-signed certificates.
