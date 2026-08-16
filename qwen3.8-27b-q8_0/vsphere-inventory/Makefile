# vsphere-inventory
#
# Primary entry points:
#   make verify   - full gate: vet + test + build, then run the CLI against a
#                   local vcsim simulator and exercise every subcommand.
#   make build    - build the CLI binary into ./bin/vsphere-inventory
#   make test     - run all unit tests (they use the embedded govmomi simulator)

BINARY      := vsphere-inventory
BIN_DIR     := bin
CLI         := $(BIN_DIR)/$(BINARY)
VCSIMSERVER := $(BIN_DIR)/vcsimserver

GO          ?= go
GOFLAGS     ?=

.PHONY: all
all: build

.PHONY: build
## Build the CLI binary.
build:
	$(GO) build $(GOFLAGS) -o $(CLI) .

.PHONY: test
## Run all unit tests.
test:
	$(GO) test ./...

.PHONY: vet
## Run go vet.
vet:
	$(GO) vet ./...

.PHONY: fmt
## Format all Go source.
fmt:
	gofmt -w .

.PHONY: fmt-check
## Fail if any Go source is not gofmt-clean.
fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

.PHONY: verify
## vet + test + build, then run the CLI against a local vcsim and exercise
## every subcommand.
verify:
	@sh scripts/verify.sh

.PHONY: clean
## Remove build artifacts.
clean:
	rm -rf $(BIN_DIR)
