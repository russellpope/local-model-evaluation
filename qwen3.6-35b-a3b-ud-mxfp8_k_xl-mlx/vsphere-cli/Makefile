.PHONY: all build vet test verify vcsim

all: build

build:
	go build -o vsphere-cli ./main.go

vet:
	go vet ./...

test:
	go test ./... -v

verify: build vet test vcsim
	@echo "All checks passed!"

vcsim: build
	go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 &
	VCPID=$$!; \
	sleep 5; \
	if ! curl -sk https://127.0.0.1:8989/sdk > /dev/null 2>&1; then \
		echo "Failed to start vcsim"; \
		kill $$VCPID 2>/dev/null; \
		exit 1; \
	fi; \
	export VSPHERE_URL=https://127.0.0.1:8989/sdk; \
	export VSPHERE_USERNAME=user; \
	export VSPHERE_PASSWORD=pass; \
	export VSPHERE_INSECURE=true; \
	echo "Running vms command..."; \
	./vsphere-cli vms || { kill $$VCPID 2>/dev/null; exit 1; }; \
	echo "Running datastores command..."; \
	./vsphere-cli datastores || { kill $$VCPID 2>/dev/null; exit 1; }; \
	echo "Running vswitches command..."; \
	SWITCHES_OUTPUT=$$(./vsphere-cli vswitches) || { kill $$VCPID 2>/dev/null; exit 1; }; \
	echo "$$SWITCHES_OUTPUT"; \
	PORTGROUP=$$(echo "$$SWITCHES_OUTPUT" | awk 'NR>1 {print $$3; exit}') || true; \
	if [ -n "$$PORTGROUP" ]; then \
		echo "Running vswitches --portgroup $$PORTGROUP..."; \
		./vsphere-cli vswitches --portgroup "$$PORTGROUP" || { kill $$VCPID 2>/dev/null; exit 1; }; \
	fi; \
	kill $$VCPID 2>/dev/null; \
	echo "vcsim verification completed successfully!"
