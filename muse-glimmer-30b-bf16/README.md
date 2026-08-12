# vSphere Inventory CLI

Go CLI using govmomi, cobra, viper.

Build:
go build -o vsphere-cli ./cmd/vsphere-cli

Run against vcsim:
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 &
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true
./vsphere-cli vms
./vsphere-cli datastores
./vsphere-cli vswitches

Config precedence: flag > env > file > default.

make verify runs go vet and go test.
