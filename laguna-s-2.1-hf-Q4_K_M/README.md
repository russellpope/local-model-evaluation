# vSphere Inventory CLI

A CLI tool for reporting vSphere virtualization inventory: virtual machines, datastores, and virtual switches.

## Features

- **VMs** — List virtual machines with CPU, memory, and storage details
- **Datastores** — List datastores with capacity, free space, and transport type classification (FC, iSCSI, NVMe, NFS)
- **vSwitches** — List standard and distributed virtual switches with port groups, VLAN, uplinks, LACP, and port usage
- **Port Group VM Lookup** — List VMs connected to a specific port group

## Installation

```bash
go build -o vsphere-inventory ./cmd/vsphere-inventory
```

## Usage

### Environment Variables

| Variable | Description |
|---|---|
| `VSPHERE_URL` | vCenter URL (e.g. `https://vc.lab/sdk`) |
| `VSPHERE_USERNAME` | vCenter username |
| `VSPHERE_PASSWORD` | vCenter password |
| `VSPHERE_INSECURE` | Skip TLS verification (`true`/`false`) |

### Flags

| Flag | Description |
|---|---|
| `--url` | vCenter URL |
| `--username` | vCenter username |
| `--password` | vCenter password |
| `--insecure` | Skip TLS verification |
| `--timeout` | Operation timeout (default `60s`) |
| `--config` | Path to config file |

### Commands

```bash
# List virtual machines
vsphere-inventory vms

# List datastores
vsphere-inventory datastores

# List virtual switches
vsphere-inventory vswitches

# List VMs connected to a port group
vsphere-inventory vswitches --portgroup "VM Network"
```

### Configuration File

Place a `config.yaml` at `~/.vsphere-inventory/` or specify with `--config`:

```yaml
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: your-password-here
insecure: false
timeout: 60s
```

## Development

### Prerequisites

- Go 1.26+
- [vcsim](https://github.com/vmware/govmomi/tree/main/vcsim) (included as a local binary in `cmd/vcsim`)

### Build

```bash
make build
```

### Run Tests

```bash
make test
```

### Verify (vet + test + integration)

```bash
make verify
```

This starts a local vcsim simulator, builds the binary, and runs all subcommands against it.

### Run vcsim Simulator

```bash
make run-vcsim
```

Starts vcsim with 8 VMs, 3 datastores, and 3 port groups on port 8989.

## Architecture

```
cmd/
  vsphere-inventory/   # Main CLI entry point (cobra commands)
  vcsim/               # Local vcsim simulator wrapper with port flag
internal/
  client/              # vSphere API client wrapper
  config/              # Configuration loading and validation
  datastores/          # Datastore inventory with transport classification
  format/              # Byte/GB formatting utilities
  transport/           # Transport type classification (FC, iSCSI, NVMe, NFS)
  vms/                 # Virtual machine inventory
  vswitch/             # Virtual switch and port group inventory
```

## License

MIT
