BINARY := vsphere-cli
VCSIM_PID_FILE := /tmp/vcsim.pid
VCSIM_PORT := 8989

.PHONY: build vet test verify clean vcsim-start vcsim-stop vcsim-run

build:
	go build -o $(BINARY) ./cmd/vsphere-cli

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f $(BINARY)
	$(MAKE) vcsim-stop

vcsim-start:
	go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 &
	echo $$! > $(VCSIM_PID_FILE)
	@echo "Waiting for vcsim to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -sk https://127.0.0.1:$(VCSIM_PORT)/sdk > /dev/null 2>&1; then \
			echo "vcsim is ready"; \
			exit 0; \
		fi; \
		sleep 1; \
	done
	@echo "vcsim failed to start"
	@exit 1

vcsim-stop:
	@if [ -f $(VCSIM_PID_FILE) ]; then \
		kill $$(cat $(VCSIM_PID_FILE)) 2>/dev/null || true; \
		rm -f $(VCSIM_PID_FILE); \
	fi

vcsim-run: build vcsim-start
	@echo "Running vms..."
	VSPHERE_URL=https://127.0.0.1:$(VCSIM_PORT)/sdk \
	VSPHERE_USERNAME=user \
	VSPHERE_PASSWORD=pass \
	VSPHERE_INSECURE=true \
	./$(BINARY) vms
	@echo ""
	@echo "Running datastores..."
	VSPHERE_URL=https://127.0.0.1:$(VCSIM_PORT)/sdk \
	VSPHERE_USERNAME=user \
	VSPHERE_PASSWORD=pass \
	VSPHERE_INSECURE=true \
	./$(BINARY) datastores
	@echo ""
	@echo "Running vswitches..."
	VSPHERE_URL=https://127.0.0.1:$(VCSIM_PORT)/sdk \
	VSPHERE_USERNAME=user \
	VSPHERE_PASSWORD=pass \
	VSPHERE_INSECURE=true \
	./$(BINARY) vswitches
	@echo ""
	@PG=$$(VSPHERE_URL=https://127.0.0.1:$(VCSIM_PORT)/sdk \
	VSPHERE_USERNAME=user \
	VSPHERE_PASSWORD=pass \
	VSPHERE_INSECURE=true \
	./$(BINARY) vswitches 2>/dev/null | awk 'NR>1 && $$3!="-" {print $$3; exit}'); \
	if [ -n "$$PG" ]; then \
		echo "Running vswitches --portgroup $$PG..."; \
		VSPHERE_URL=https://127.0.0.1:$(VCSIM_PORT)/sdk \
		VSPHERE_USERNAME=user \
		VSPHERE_PASSWORD=pass \
		VSPHERE_INSECURE=true \
		./$(BINARY) vswitches --portgroup "$$PG"; \
	fi
	$(MAKE) vcsim-stop

verify: vet test vcsim-run
	@echo ""
	@echo "=== All checks passed ==="
