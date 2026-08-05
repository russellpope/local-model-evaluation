# vSphere Inventory CLI - Makefile

BINARY_NAME := vsphere-inventory
GO := go

.PHONY: build
build:
	$(GO) build -o $(BINARY_NAME) ./cmd/vsphere-inventory

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test ./...

.PHONY: fmt
fmt:
	gofmt -l .

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

.PHONY: run-vcsim
run-vcsim:
	go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 &

.PHONY: verify
verify: vet test
	@echo "=== Starting vcsim simulator ==="
	@go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 &
	@VCSIM_PID=$$!
	@sleep 3
	@echo "=== Waiting for vcsim to be ready ==="
	@for i in $$(seq 1 10); do \
		if curl -sk https://127.0.0.1:8989/sdk > /dev/null 2>&1; then \
			break; \
		fi; \
		sleep 1; \
	done
	@echo "=== Building binary ==="
	@$(GO) build -o $(BINARY_NAME) ./cmd/vsphere-inventory
	@echo "=== Running vms subcommand ==="
	@VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vms
	@echo "=== Running datastores subcommand ==="
	@VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) datastores
	@echo "=== Running vswitches subcommand ==="
	@VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vswitches
	@echo "=== Discovering portgroup name ==="
	@PORTGROUP=$$(VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vswitches | tail -n +2 | head -1 | awk -F'\t' '{print $$3}'); \
	echo "Using portgroup: $$PORTGROUP"; \
	echo "=== Running vswitches --portgroup $$PORTGROUP ==="; \
	VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vswitches --portgroup "$$PORTGROUP"
	@echo "=== Stopping vcsim ==="
	@kill $$VCSIM_PID 2>/dev/null || true
	@wait $$VCSIM_PID 2>/dev/null || true
	@echo "=== All checks passed ==="
