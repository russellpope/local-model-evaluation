.PHONY: build test vet fmt verify

BINARY = vsphere-inventory

build:
	go build -o $(BINARY) ./cmd/vsphere-inventory

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

verify: vet test
	@echo "Running build..."
	go build -o $(BINARY) ./cmd/vsphere-inventory
	@echo "Build successful."
	@echo "Running integration tests against simulator..."
	go test ./cmd/... -v -run "TestVMsCommandAgainstSimulator|TestDatastoresCommandAgainstSimulator|TestVSwitchesCommandAgainstSimulator|TestVSwitchesPortgroupCommandAgainstSimulator"
	@echo "All checks passed."
