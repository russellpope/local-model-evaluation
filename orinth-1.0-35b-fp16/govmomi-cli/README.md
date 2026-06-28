# govmomi-cli

A small CLI tool that connects to a VMware vCenter Server (or the
[vcsim](https://github.com/vmware/govmomi/tree/main/simulator) simulator) and
reports virtualisation inventory across three views:

```
govmomi-cli vms              List all VMs with vCPU, RAM (GiB), consumed storage.
govmomi-cli datastores       List all datastores with transport type, used/available.
govmomi-cli vswitches        List standard + distributed switches and port groups.
govmomi-cli vswitches --portgroup <name>   VMs connected to that port group.
```

## Build

```sh
go build -o govmomi-cli ./...
```

Run the full verification suite (gofmt, vet, staticcheck, `go test -race`):

```sh
make verify
```

## Configuration

The CLI accepts connection parameters via **CLI flags**, **environment variables**
prefixed with `VSPHERE_`, or an optional YAML config file. Precedence is:

> flag > env var > YAML config file > default

### Flags (all persistent on the root command)

```
--url           vCenter URL or host, e.g. https://vc.lab/sdk
--username      vCenter username
--password      vCenter password
--insecure      Skip TLS certificate verification
--timeout       Overall operation timeout (default 60s), e.g. 30s / 5m
--config        Path to YAML config file (optional)
```

### Environment variables

| Variable               | Example                         |
|------------------------|---------------------------------|
| `VSPHERE_URL`          | `https://vc.lab/sdk`            |
| `VSPHERE_USERNAME`     | `administrator@vsphere.local`   |
| `VSPHERE_PASSWORD`     | `s3cret`                        |
| `VSPHERE_INSECURE`     | `true`                          |
| `VSPHERE_TIMEOUT`      | `30s`                           |

### YAML config file

Place a `config.yaml` next to the binary or pass it explicitly:

```yaml
# ~/.config/govmomi-cli/config.yaml  (searched automatically)
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: s3cret
insecure: true
timeout: 60s
```

Override via the flag or env var on a per-invocation basis:

```sh
govmomi-cli --config ~/.config/govmomi-cli/config.yaml vms

# Or, using only environment variables:
VSPHERE_URL=https://vc.lab/sdk \
VSPHERE_USERNAME=administrator@vsphere.local \
VSPHERE_PASSWORD=s3cret \
govmomi-cli datastores

# Flags always win over env vars and file values:
govmomi-cli --url https://other/sdk --insecure vswitches
```

## Example run against vcsim

Start the simulator (from the govmomi source tree):

```sh
go run github.com/vmware/govmomi/simulator@latest -l :8989
```

Then point `govmomi-cli` at it:

```sh
VSPHERE_URL=http://localhost:8989/sdk \
VSPHERE_USERNAME=admin \
VSPHERE_PASSWORD=admin \
./govmomi-cli --insecure vms
```
