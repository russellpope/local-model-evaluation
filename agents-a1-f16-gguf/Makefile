.PHONY: all build test vet verify clean

all: verify

build:
	go build -o vsphere-inventory .

test:
	go test ./...

vet:
	go vet ./...

verify: test vet
	@echo "Running vcsim verification..."
	@go run github.com/vmware/govmomi/vcsim -vm 4 -ds 2 -pg 2 & \
		VSPHERE_URL=https://127.0.0.1:8989/sdk \
		VSPHERE_USERNAME=user \
		VSPHERE_PASSWORD=pass \
		VSPHERE_INSECURE=true \
		./vsphere-inventory vms && \
		./vsphere-inventory datastores && \
		./vsphere-inventory vswitches && \
		./vsphere-inventory vswitches --portgroup "Network 1" && \
		kill $$!
	@echo "Verification complete."

clean:
	rm -f vsphere-inventory
