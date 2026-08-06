# vSphere Inventory CLI - Makefile

BINARY_NAME := vsphere-inventory
VCSIM_BIN := vcsim-bin
GO := go
VCSIM_PORT := 8989
VCSIM_URL := https://127.0.0.1:$(VCSIM_PORT)/sdk

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
	rm -f $(BINARY_NAME) $(VCSIM_BIN)

.PHONY: run-vcsim
run-vcsim:
	$(GO) build -o $(VCSIM_BIN) ./cmd/vcsim
	./$(VCSIM_BIN) -vm 8 -ds 3 -pg 3 -port $(VCSIM_PORT)

.PHONY: verify
verify: vet test
	@echo "=== Building vcsim ==="
	@$(GO) build -o $(VCSIM_BIN) ./cmd/vcsim
	@echo "=== Starting vcsim simulator ==="
	@./$(VCSIM_BIN) -vm 8 -ds 3 -pg 3 -port $(VCSIM_PORT) & \
	VCSIM_PID=$$!; \
	echo "vcsim PID: $$VCSIM_PID"; \
	echo "=== Waiting for vcsim to be ready ==="; \
	READY=0; \
	for i in $$(seq 1 15); do \
		if curl -sk $(VCSIM_URL) > /dev/null 2>&1; then \
			READY=1; \
			echo "vcsim is ready"; \
			break; \
		fi; \
		if ! kill -0 $$VCSIM_PID 2>/dev/null; then \
			echo "ERROR: vcsim process exited unexpectedly"; \
			rm -f $(VCSIM_BIN); \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	if [ $$READY -eq 0 ]; then \
		echo "ERROR: vcsim did not become ready in time"; \
		kill $$VCSIM_PID 2>/dev/null || true; \
		wait $$VCSIM_PID 2>/dev/null || true; \
		rm -f $(VCSIM_BIN); \
		exit 1; \
	fi; \
	echo "=== Building binary ==="; \
	$(GO) build -o $(BINARY_NAME) ./cmd/vsphere-inventory; \
	echo "=== Running vms subcommand ==="; \
	VSPHERE_URL=$(VCSIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vms; \
	echo "=== Running datastores subcommand ==="; \
	VSPHERE_URL=$(VCSIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) datastores; \
	echo "=== Running vswitches subcommand ==="; \
	VSPHERE_URL=$(VCSIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vswitches; \
	echo "=== Discovering portgroup name ==="; \
	PORTGROUP=$$(VSPHERE_URL=$(VCSIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vswitches | tail -n +2 | head -1 | awk '{print $$3}'); \
	echo "Using portgroup: $$PORTGROUP"; \
	if [ -z "$$PORTGROUP" ]; then \
		echo "ERROR: could not discover portgroup name from vswitches output"; \
		kill $$VCSIM_PID 2>/dev/null || true; \
		wait $$VCSIM_PID 2>/dev/null || true; \
		rm -f $(VCSIM_BIN); \
		exit 1; \
	fi; \
	echo "=== Running vswitches --portgroup $$PORTGROUP ==="; \
	VSPHERE_URL=$(VCSIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true ./$(BINARY_NAME) vswitches --portgroup "$$PORTGROUP"; \
	echo "=== Stopping vcsim ==="; \
	kill $$VCSIM_PID 2>/dev/null || true; \
	wait $$VCSIM_PID 2>/dev/null || true; \
	rm -f $(VCSIM_BIN); \
	echo "=== All checks passed ==="
