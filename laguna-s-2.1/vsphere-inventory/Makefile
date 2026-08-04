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

verify: vet fmt
	@echo "Running gofmt check..."
	@if [ -n "$$(gofmt -l .)" ]; then echo "gofmt found issues:"; gofmt -l .; exit 1; fi
	@echo "gofmt check passed."
	@echo "Running build..."
	go build -o $(BINARY) ./cmd/vsphere-inventory
	@echo "Build successful."
	@echo "Running full end-to-end loop against simulator..."
	go test ./... -race -count=1 -v -run "TestEndToEnd"
	@echo "End-to-end loop passed."
	@echo "Running full test suite..."
	go test ./... -race -count=1
	@echo "All checks passed."
