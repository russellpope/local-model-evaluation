# vsphere-inventory

A small command-line tool that connects to a vCenter Server (or a vCenter
simulator, `vcsim`) and reports virtualization inventory as plain-text tables:
virtual machines, datastores, and virtual switches.

Built on [govmomi](https://github.com/vmware/govmomi),
[cobra](https://github.com/spf13/cobra), and
[viper](https://github.com/spf13/viper).

## Commands

- `vms` — list virtual machines with vCPU, RAM, and committed storage.
- `datastores` — list datastores with backing transport, used, and available capacity.
- `vswitches` — list virtual switches and port groups (standard and distributed).
  Pass `--portgroup <name>` to list the virtual machines connected to that
  port group instead.

Every table is written with the standard library `text/tabwriter`, so columns
line up regardless of value width.

## Building

```sh
make build      # produces ./vsphere-inventory
make vet        # go vet ./...
make test       # go test ./...
make verify     # vet + test, then an end-to-end run against vcsim
make clean
```

## Configuration

Connection settings are resolved in this order of precedence (highest wins):

1. Command-line flags: `--url`, `--username`, `--password`, `--insecure`, `--timeout`
2. Environment variables (prefix `VSPHERE_`): `VSPHERE_URL`, `VSPHERE_USERNAME`,
   `VSPHERE_PASSWORD`, `VSPHERE_INSECURE`, `VSPHERE_TIMEOUT`
3. A YAML config file, if `--config <path>` is given
4. Built-in defaults (`insecure=false`, `timeout=60s`)

A flag that you do not set does *not* override a lower layer — only flags you
actually provide take effect.

### Config file

`--config` points at a YAML file using the keys `url`, `username`, `password`,
`insecure`, and `timeout`. See [config.yaml.example](config.yaml.example).

```sh
./vsphere-inventory --config config.yaml vms
```

### Examples

```sh
# Everything from environment variables
export VSPHERE_URL=https://vc.example.com/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD=secret
export VSPHERE_INSECURE=true
./vsphere-inventory vms

# A bare host works; https is assumed and /sdk appended
./vsphere-inventory --url vc.example.com --username user --password pass --insecure vswitches

# VMs behind a specific port group
./vsphere-inventory vswitches --portgroup "VM Network"
```

## Output

`vms`:

```
NAME            VCPU  RAM    STORAGE
DC0_C0_RP0_VM0  1     2.0GiB 5.0GiB
DC0_H0_VM0      2     4.0GiB 20.0GiB
```

`datastores`:

```
NAME       TYPE    USED     AVAILABLE
LocalDS_0  unknown 160.0GiB 9.8TiB
```

`vswitches`:

```
SWITCH    SWITCH TYPE  PORTGROUP  VLAN    UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0  0       unknown  disabled  0      0
vSwitch0  standard     VM Network 0       unknown  N/A       0      0
```

Fields that cannot be determined from the inventory (for example the backing
transport of a datastore that has no scsi/NFS host, or uplinks on a
simulator) are rendered as `unknown` or `N/A` rather than guessed.

## End-to-end verification

`make verify` (or `scripts/verify.sh`) builds the tool, starts a `vcsim`
simulator in the background with a known inventory (8 VMs, 3 datastores, 3
port groups), waits for it to become ready, and runs every subcommand —
including a `--portgroup` lookup whose value is discovered from the
`vswitches` output. It tears the simulator down on exit and fails non-zero if
anything is wrong.

## Project layout

```
main.go            entrypoint
cmd/               cobra commands (root, vms, datastores, vswitches)
config/            viper-backed configuration + precedence
inventory/         govmomi access, classification, formatting, tests
scripts/verify.sh  end-to-end check against vcsim
```
