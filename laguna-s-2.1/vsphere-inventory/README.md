# vSphere Inventory CLI

A CLI tool for reporting VMware vSphere virtualization inventory: virtual machines, datastores, and virtual switches/port groups.

## Usage

```sh
# Build
go build -o vsphere-inventory ./cmd/vsphere-inventory

# List all VMs
./vsphere-inventory vms --url https://vc.lab/sdk --username admin --password pass

# List all datastores
./vsphere-inventory datastores --url https://vc.lab/sdk --username admin --password pass

# List all switches and port groups
./vsphere-inventory vswitches --url https://vc.lab/sdk --username admin --password pass

# List VMs connected to a specific port group
./vsphere-inventory vswitches --portgroup "DC0_DVPG0" --url https://vc.lab/sdk --username admin --password pass
```

### Environment Variables

All flags can be set via environment variables with the `VSPHERE_` prefix:

- `VSPHERE_URL`
- `VSPHERE_USERNAME`
- `VSPHERE_PASSWORD`
- `VSPHERE_INSECURE`
- `VSPHERE_TIMEOUT`

### Configuration File

Use `--config` to specify a YAML config file:

```yaml
url: https://vc.lab/sdk
username: admin
password: pass
insecure: false
timeout: 60s
```

## Development

```sh
# Run all tests with race detection
go test ./... -race -count=1

# Run linting and formatting checks
gofmt -l .

# Run full verification
make verify
```

## Architecture

- `cmd/` - CLI commands (vms, datastores, vswitches)
- `internal/config/` - Configuration loading and vSphere client management
- `internal/datastores/` - Datastore retrieval with transport classification
- `internal/vms/` - Virtual machine retrieval
- `internal/vswitches/` - Virtual switch and port group retrieval
- `internal/transport/` - Storage transport type classification (FC, iSCSI, NVMe, NFS)
- `internal/format/` - Byte formatting utilities
