.PHONY: all build vet test verify clean vcsim-stop

BINARY := govmomi-cli
VCSIM_PID := /tmp/vcsim.pid

all: build vet test

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

test:
	go test -v ./...

verify: build vet test
	@echo "Starting vcsim..."
	@go run github.com/vmware/govmomi/vcsim -vm 4 -ds 2 -pg 2 > /tmp/vcsim.log 2>&1 & \
		echo $$! > $(VCSIM_PID); \
		sleep 3; \
		if ! ps -p $$($(cat $(VCSIM_PID))) > /dev/null; then \
			echo "vcsim failed to start"; \
			cat /tmp/vcsim.log; \
			exit 1; \
		fi; \
		echo "vcsim started with PID $$(cat $(VCSIM_PID))"; \
		export VSPHERE_URL=https://127.0.0.1:8989/sdk; \
		export VSPHERE_USERNAME=user; \
		export VSPHERE_PASSWORD=pass; \
		export VSPHERE_INSECURE=true; \
		echo "Running vms..."; \
		./$(BINARY) vms || { echo "vms command failed"; vcsim-stop; exit 1; }; \
		echo "Running datastores..."; \
		./$(BINARY) datastores || { echo "datastores command failed"; vcsim-stop; exit 1; }; \
		echo "Running vswitches..."; \
		./$(BINARY) vswitches || { echo "vswitches command failed"; vcsim-stop; exit 1; }; \
		PG_NAME=$$(./$(BINARY) vswitches | grep -v "SWITCH" | grep -v "------" | head -1 | awk '{print $3}'); \
		echo "Running vswitches --portgroup $$PG_NAME..."; \
		./$(BINARY) vswitches --portgroup "$$PG_NAME" || { echo "vswitches --portgroup command failed"; vcsim-stop; exit 1; }; \
		echo "All verifications passed!"; \
		vcsim-stop

vcsim-stop:
	@if [ -f $(VCSIM_PID) ]; then \
		kill $$(cat $(VCSIM_PID)) 2>/dev/null || true; \
		rm -f $(VCSIM_PID); \
	fi

clean:
	rm -f $(BINARY)
	rm -f /tmp/vcsim.log
	rm -f $(VCSIM_PID)
