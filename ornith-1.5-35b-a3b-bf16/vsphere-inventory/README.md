# vsphere-inventory

A single Go binary that connects to a vCenter Server and reports virtualization
inventory — virtual machines, datastores, and virtual switches — through three
subcommands.

## Build

```sh
go build -o bin/vsphere-inventory .
```

## Run

Each subcommand reads connection settings with the precedence
flag > environment variable > config file > default.

```sh
# Via flags
./bin/vsphere-inventory vms \
  --url https://vcenter.example.com/sdk \
  --username Administrator@vsphere-local \
  --password '<password>' \
  --insecure

# Via environment variables
export VSPHERE_URL=https://vcenter.example.com/sdk
export VSPHERE_USERNAME=Administrator@vsphere-local
export VSPHERE_PASSWORD='<password>'
export VSPHERE_INSECURE=true
./bin/vsphere-inventory datastores
```

### Subcommands

| Command     | Description                                              |
|-------------|----------------------------------------------------------|
| `vms`       | List virtual machines (name, host, CPU, RAM, disk).      |
| `datastores`| List datastores (type, capacity, used, available).       |
| `vswitches` | List standard and distributed virtual switches.          |

`vswitches` accepts `--portgroup <name>` to instead list the virtual machines
connected to a single port group (standard or distributed).

```sh
./bin/vsphere-inventory vswitches --portgroup DC0_DVPG0 \
  --url https://vcenter.example.com/sdk --insecure
```

## Configuration

See [`config.yaml.example`](./config.yaml.example). Load it explicitly with
`--config path/to/config.yaml`, or set any key via the matching `VSPHERE_*`
environment variable (`VSPHERE_URL`, `VSPHERE_USERNAME`, `VSPHERE_PASSWORD`,
`VSPHERE_INSECURE`, `VSPHERE_TIMEOUT`).

## Verify

`make verify` runs static analysis, the unit tests, the build, and a live smoke
test of every subcommand against the local `vcsim` simulator (which is started
and torn down automatically):

```sh
make verify
```

Equivalent manual steps:

```sh
go vet ./...
go test ./...
go build -o bin/vsphere-inventory .

# vcsim ships as a separate module and is only needed for this smoke test; add
# it to the build list, then start the simulator in one shell:
go get github.com/vmware/govmomi/vcsim@v0.0.0-20260820135757-2d753e5caa0c
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 -l 127.0.0.1:8989

# Run against it (in another shell):
./bin/vsphere-inventory vms --url https://127.0.0.1:8989/sdk --insecure

# Optionally restore a tidy module graph afterwards:
go mod tidy
```
