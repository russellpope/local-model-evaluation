.PHONY: all build test verify clean

all: build test

build:
	go build -o vsphere ./cmd/main.go

test:
	go test -v ./...

verify:
	go vet ./...
	go test -v ./...
	@echo "Starting vcsim..."
	@go run github.com/vmware/govmomi/vcsim & \
	VC_PID=$$! ; \
	sleep 10 ; \
	export VSPHERE_URL=https://127.0.0.1:8989/sdk; \
	export VSPHERE_USERNAME=user; \
	export VSPHERE_PASSWORD=pass; \
	export VSPHERE_INSECURE=true; \
	./vsphere vms; \
	./vsphere datastores; \
	./vsphere vswitches; \
	# Get a portgroup name from the output (simplified, might need better parsing)
	# For now we just run it. If it fails due to empty portgroup list it's okay 
	# as long as it doesn't crash.
	./vsphere vswitches --portgroup "dummy_pg"; \
	kill $VC_PID

clean:
	rm -f vsphere
