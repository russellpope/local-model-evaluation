SHELL := /bin/bash

ROOT := $(CURDIR)
GOCACHE := $(ROOT)/.cache/go-build
GOMODCACHE := $(ROOT)/.cache/go-mod
BINARY := $(ROOT)/bin/vsphere-inventory
VSPHERE_URL := https://127.0.0.1:8989/sdk

.PHONY: build test vet verify

build:
	mkdir -p $(ROOT)/bin $(GOCACHE) $(GOMODCACHE)
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o $(BINARY) .

test:
	mkdir -p $(GOCACHE) $(GOMODCACHE)
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test ./...

vet:
	mkdir -p $(GOCACHE) $(GOMODCACHE)
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go vet ./...

verify: vet test build
	mkdir -p $(ROOT)/.cache
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go run ./vcsim.go -vm 8 -ds 3 -pg 3 > $(ROOT)/.cache/vcsim.log 2>&1 & echo $$! > $(ROOT)/.cache/vcsim.pid
	trap 'kill $$(cat $(ROOT)/.cache/vcsim.pid) 2>/dev/null || true' EXIT; \
	export VSPHERE_URL=$(VSPHERE_URL); \
	export VSPHERE_USERNAME=user; \
	export VSPHERE_PASSWORD=pass; \
	export VSPHERE_INSECURE=true; \
	for i in {1..30}; do \
		if $(BINARY) vms > $(ROOT)/.cache/verify-vms.txt 2> $(ROOT)/.cache/verify-vms.err; then break; fi; \
		sleep 1; \
		if [[ $$i -eq 30 ]]; then cat $(ROOT)/.cache/vcsim.log; cat $(ROOT)/.cache/verify-vms.err; exit 1; fi; \
	done; \
	$(BINARY) datastores > $(ROOT)/.cache/verify-datastores.txt; \
	$(BINARY) vswitches > $(ROOT)/.cache/verify-vswitches.txt; \
	pg=$$(awk 'NR > 1 && $$2 == "distributed" { print $$3; exit }' $(ROOT)/.cache/verify-vswitches.txt); \
	if [[ -z "$$pg" ]]; then echo "no distributed portgroup discovered"; cat $(ROOT)/.cache/verify-vswitches.txt; exit 1; fi; \
	$(BINARY) vswitches --portgroup "$$pg" > $(ROOT)/.cache/verify-portgroup.txt; \
	test $$(wc -l < $(ROOT)/.cache/verify-portgroup.txt) -gt 1
