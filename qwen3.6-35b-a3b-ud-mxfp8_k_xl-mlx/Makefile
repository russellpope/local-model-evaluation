.PHONY: build test vet verify help

BINARY = vsphere-inventory

help:
	@echo "Available targets:"
	@echo "  build    - Build the binary"
	@echo "  test     - Run unit tests"
	@echo "  vet      - Run go vet"
	@echo "  verify   - Full verification: vet, test, and run against vcsim"
	@echo "  clean    - Remove build artifacts"

build:
	go build -o $(BINARY) ./cmd/$(BINARY)/

test:
	go test ./... -v

vet:
	go vet ./...

verify: vet test build
	@echo ""
	@echo "=== Unit tests passed ==="
	@echo ""
	@echo "To run against vcsim simulator, execute:"
	@echo ""
	@echo "  # Start vcsim in one terminal"
	@echo "  go run github.com/vmware/govmomi/simulator/cmd/vcsim@"
	@echo ""
	@echo "  # In another terminal, run the CLI"
	@echo "  export VSPHERE_URL=https://127.0.0.1:8989/sdk"
	@echo "  export VSPHERE_USERNAME=user"
	@echo "  export VSPHERE_PASSWORD=pass"
	@echo "  export VSPHERE_INSECURE=true"
	@echo ""
	@echo "  ./$(BINARY) vms"
	@echo "  ./$(BINARY) datastores"
	@echo "  ./$(BINARY) vswitches"
	@echo ""

clean:
	rm -f $(BINARY)
