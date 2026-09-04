BINARY := vsphere-inventory
GO ?= go

VCSIM_PKG := github.com/vmware/govmomi/vcsim
VCSIM_LOG := $(TMPDIR)/vcsim-verify.log

.DEFAULT_GOAL := build

.PHONY: all build test vet fmt tidy clean verify

all: verify

build:
	$(GO) build -o bin/$(BINARY) .

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

clean:
	rm -rf bin $(VCSIM_LOG)

# verify runs the complete local gate: vet, unit tests (hermetic, using the
# embedded simulator), then a live smoke run of every subcommand against
# vcsim, including a --portgroup lookup discovered from the vswitches output.
# Exits non-zero on any failure and always tears the simulator down.
verify: build
	$(GO) vet ./...
	$(GO) test ./...
	@set -eu; \
	$(GO) run $(VCSIM_PKG) -vm 4 -ds 3 -pg 3 > "$(VCSIM_LOG)" 2>&1 & \
	sim_pid=$$!; \
	trap 'kill -9 $$sim_pid 2>/dev/null || true; pkill -9 -f "exe/vcsim" 2>/dev/null || true' EXIT INT TERM; \
	url=""; \
	for i in $$(seq 1 60); do \
		if grep -q "export GOVC_URL=" "$(VCSIM_LOG)" 2>/dev/null; then \
			endpoint=$$(sed -n 's/^export GOVC_URL=https:\/\/[^@]*@//p' "$(VCSIM_LOG)" | head -1 | cut -d' ' -f1); \
			if [ -n "$$endpoint" ]; then url="$$endpoint"; break; fi; \
		fi; \
		sleep 1; \
	done; \
	if [ -z "$$url" ]; then echo "vcsim did not become ready"; exit 1; fi; \
	export VSPHERE_URL="$$url" VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true VSPHERE_TIMEOUT=30s; \
	echo "== vms ==";              ./bin/$(BINARY) vms; \
	echo "== datastores ==";       ./bin/$(BINARY) datastores; \
	echo "== vswitches ==";        ./bin/$(BINARY) vswitches; \
	pg=$$(./bin/$(BINARY) vswitches | awk 'NR>2 && $$3 != "-" {print $$3}' | head -1); \
	if [ -z "$$pg" ]; then echo "no port group found in vswitches output"; exit 1; fi; \
	echo "== vswitches --portgroup $$pg =="; \
	./bin/$(BINARY) vswitches --portgroup "$$pg"; \
	echo "== verify OK =="
