# vSphere Inventory CLI

Single-binary Go CLI for VMware vCenter inventory using `govmomi`.

## Commands

```sh
go build -o bin/vsphere-inventory .

bin/vsphere-inventory vms
bin/vsphere-inventory datastores
bin/vsphere-inventory vswitches
bin/vsphere-inventory vswitches --portgroup "DVPG0"
```

## Configuration

Precedence is flag, environment, config file, default.

Environment example:

```sh
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

bin/vsphere-inventory vms
```

Config file example:

```yaml
url: https://vc.lab/sdk
username: administrator@vsphere.local
password: secret
insecure: false
timeout: 60s
```

Run with:

```sh
bin/vsphere-inventory --config config.yaml datastores
```

## Verification

Install dependencies with:

```sh
go mod tidy
```

Run all local checks and the required simulator smoke test:

```sh
make verify
```

`make verify` runs `go vet ./...`, `go test ./...`, builds `bin/vsphere-inventory`, starts the build-ignored `vcsim.go` runner backed by govmomi's simulator package, runs `vms`, `datastores`, `vswitches`, discovers a distributed portgroup from the switch output, and runs `vswitches --portgroup`.

Against `vcsim`, storage transport and LACP/uplink details may render as `unknown` or `N/A` when the simulator does not model those live-vCenter details.
