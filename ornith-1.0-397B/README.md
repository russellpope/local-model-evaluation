# vSphere CLI

A command-line tool for querying vSphere inventory: virtual machines, datastores, and virtual switches.

## Prerequisites

- Go 1.26+
- Access to a vCenter Server (or a `vcsim` simulator for testing)

## Build

```bash
go build -o vsphere-cli ./cmd/vsphere-cli
```

Or via Make:

```bash
make build
```

## Configuration

Configuration is resolved in this precedence order (highest first): CLI flags > environment variables > config file > defaults.

### Config file

Place at `~/.vsphere-cli/config.yaml` or specify with `--config`:

```yaml
url: https://vcenter.example.com/sdk
username: administrator@vsphere.local
password: your-password-here
insecure: false
timeout: 60s
```

See `config.yaml.example` for a template.

### Environment variables

```bash
export VSPHERE_URL=https://vcenter.example.com/sdk
export VSPHERE_USERNAME=administrator@vsphere.local
export VSPHERE_PASSWORD=your-password-here
export VSPHERE_INSECURE=true
export VSPHERE_TIMEOUT=60s
```

### CLI flags

All config values can be passed as flags:

```bash
./vsphere-cli --url https://vcenter.example.com/sdk \
  --username admin --password pass --insecure vms
```

Use `--insecure=false` to explicitly override `VSPHERE_INSECURE=true`.

## Usage

### List virtual machines

```bash
./vsphere-cli vms
```

Output:

```
NAME              VCPU  RAM       STORAGE
DC0_H0_VM0        1     32.0 MiB  0.0 GiB
DC0_H0_VM1        1     32.0 MiB  0.0 GiB
```

### List datastores

```bash
./vsphere-cli datastores
```

Output:

```
NAME              TYPE     USED       AVAILABLE
Local-Issue-DS    unknown  0.0 GiB    10.0 GiB
```

### List virtual switches

```bash
./vsphere-cli vswitches
```

Output:

```
SWITCH      SWITCH TYPE  PORTGROUP       VLAN  UPLINKS  LACP     PORTS  USED
DVSwitch0   distributed  DVPG0           0     dvport   disabled 16     0
vSwitch0    standard     Management Net  0     vmnic1   N/A      56     0
vSwitch0    standard     VM Network      0     vmnic1   N/A      56     0
```

### List VMs in a port group

```bash
./vsphere-cli vswitches --portgroup "VM Network"
```

Output:

```
NAME              VCPU  RAM       STORAGE
DC0_H0_VM0        1     32.0 MiB  0.0 GiB
DC0_H0_VM1        1     32.0 MiB  0.0 GiB
```

## Make targets

| Target       | Description                                                  |
|--------------|--------------------------------------------------------------|
| `make build` | Compile the binary                                           |
| `make vet`   | Run `go vet`                                                 |
| `make test`  | Run tests                                                    |
| `make clean` | Remove binary and stop vcsim                                 |
| `make verify`| Full pipeline: vet, test, then run all subcommands against vcsim |

### `make verify`

Starts a `vcsim` simulator, runs all three subcommands (`vms`, `datastores`, `vswitches`), extracts a real port group name from the vswitch output, runs `vswitches --portgroup <name>`, then stops the simulator. All checks must pass for the target to succeed.

## Sample run with vcsim

```bash
make vcsim-start

VSPHERE_URL=https://127.0.0.1:8989/sdk \
VSPHERE_USERNAME=user \
VSPHERE_PASSWORD=pass \
VSPHERE_INSECURE=true \
./vsphere-cli vms

make vcsim-stop
```

## Security note

- `gosec` reports 5x G104 on `v.BindEnv` calls in `config.go`. These are false positives: `BindEnv` only errors on an empty key list, which is a programmer error. The errors are explicitly ignored with `_ =`.
- `govulncheck` reports GO-2026-5024 in `golang.org/x/sys@v0.29.0`. This is a Windows-only vulnerability (registry operations) and is unreachable on darwin/linux. No action required.
