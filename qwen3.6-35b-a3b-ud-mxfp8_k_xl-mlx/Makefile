.PHONY: build test vet verify help clean

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
	@echo "=== Running integration tests against vcsim ==="
	@BINARY=$(BINARY) bash scripts/verify.sh

clean:
	rm -f $(BINARY)
