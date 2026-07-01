.PHONY: all build test verify clean

all: build test

build:
	go build -o govmomi-cli main.go

test:
	go vet ./...
	go test -v ./...

verify: build test
	@echo "Starting vcsim..."
	@go run github.com/vmware/govmomi/vcsim -vm 5 -ds 2 -pg 2 & \
	SIM_PID=$$!; \
	sleep 5; \
	export VSPHERE_URL=https://127.0.0.1:8989/sdk; \
	export VSPHERE_USERNAME=user; \
	export VSPHERE_PASSWORD=pass; \
	export VSPHERE_INSECURE=true; \
	(./govmomi-cli vms && ./govmomi-cli datastores && ./govmomi-cli vswitches && ./govmomi-cli vswitches --portgroup DC0_DVPG0); \
	EXIT_CODE=$$?; \
	kill $$SIM_PID; \
	exit $$EXIT_CODE

clean:
	rm -f govmomi-cli
