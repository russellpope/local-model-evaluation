# Makefile for vsphere-inventory.
# Kept compatible with GNU make 3.81 (the version shipped on macOS): no
# .ONESHELL; multi-step shell logic lives in scripts/verify.sh.

SHELL := /bin/bash

BINARY := vsphere-inventory

.PHONY: all build test vet fmt tidy run-vcsim verify clean

all: build

build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }
	@echo "gofmt clean"

tidy:
	go mod tidy

# Run the bundled simulator in the foreground (Ctrl-C to stop).
run-vcsim:
	go run ./tools/vcsim -vm 8 -ds 3 -pg 3

# Full self-check: vet + tests, then build and exercise every subcommand
# against a freshly started vcsim and tear it down. Non-zero on any failure.
verify: vet test
	@bash scripts/verify.sh

clean:
	rm -rf bin
	rm -f $(BINARY) .vcsim-verify.log
