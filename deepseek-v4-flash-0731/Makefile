# vint — vCenter inventory CLI

SHELL := /bin/bash
BUILD_DIR := build
BIN := $(BUILD_DIR)/vint

# vcsim ships as a standalone binary (installed where Go tools are dropped).
SIM ?= $(shell command -v vcsim 2>/dev/null || echo "$$(go env GOPATH)/bin/vcsim")
SIM_ADDR ?= 127.0.0.1:18989
SIM_URL ?= https://127.0.0.1:18989/sdk

.PHONY: all build vet test verify clean

all: build

build:
	go build -o $(BIN) .

vet:
	go vet ./...

test:
	go test ./...

# Full acceptance loop: vet, hermetic tests, then an end-to-end run against a
# freshly started vcsim exercising every subcommand (including --portgroup).
verify: vet test build
	@set -euo pipefail; \
	mkdir -p $(BUILD_DIR); \
	"$(SIM)" -l $(SIM_ADDR) -vm 8 -ds 3 -pg 3 >"$(BUILD_DIR)/vcsim.log" 2>&1 & \
	sim_pid=$$!; \
	trap 'kill "$$sim_pid" 2>/dev/null || true' EXIT; \
	export VSPHERE_URL=$(SIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true; \
	ready=0; \
	for i in $$(seq 1 30); do \
		if curl -sk --max-time 1 $(SIM_URL) -o /dev/null; then ready=1; break; fi; \
		sleep 1; \
	done; \
	[ "$$ready" = "1" ] || { echo "verify: vcsim did not become ready" >&2; exit 1; }; \
	sleep 1; \
	./$(BIN) vms; \
	./$(BIN) datastores; \
	./$(BIN) vswitches; \
	pg="$$(./$(BIN) vswitches | awk 'NR>1 && $$2=="distributed" && $$1=="DVS0" {print $$3; exit}')"; \
	[ -n "$$pg" ] || { echo "verify: no port group discoverable from vswitches output" >&2; exit 1; }; \
	./$(BIN) vswitches --portgroup "$$pg"; \
	echo; \
	echo "verify: OK"

clean:
	rm -rf $(BUILD_DIR)