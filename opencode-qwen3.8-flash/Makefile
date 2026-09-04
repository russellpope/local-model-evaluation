BINARY      := bin/vsphere-inventory
VCSIM       := bin/vcsim
VCSIM_PORT  := 18989
GOVOMIMI    := github.com/vmware/govmomi

.PHONY: all build vet test fmtcheck verify clean

all: build

build:
	go build -o $(BINARY) .

$(VCSIM): go.mod go.sum
	go build -o $(VCSIM) $(GOVOMIMI)/vcsim

vet:
	go vet ./...

test:
	go test ./...

fmtcheck:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

# verify automates the full self-check loop:
# vet + unit tests, then a live vcsim run of every subcommand including
# --portgroup discovery. Non-zero exit on any failure; simulator is torn down.
verify: fmtcheck vet test build $(VCSIM)
	@set -eu; \
	$(VCSIM) -l 127.0.0.1:$(VCSIM_PORT) -vm 8 -ds 3 -pg 3 > bin/vcsim.log 2>&1 & \
	pid=$$!; \
	trap 'kill $$pid 2>/dev/null || true; wait $$pid 2>/dev/null || true' EXIT INT TERM; \
	echo "waiting for vcsim (pid $$pid)..."; \
	for i in $$(seq 120); do \
		curl -sk https://127.0.0.1:$(VCSIM_PORT)/about >/dev/null 2>&1 && break; \
		sleep 0.25; \
	done; \
	curl -sk https://127.0.0.1:$(VCSIM_PORT)/about >/dev/null || { echo "vcsim did not come up"; cat bin/vcsim.log; exit 1; }; \
	export VSPHERE_URL=https://127.0.0.1:$(VCSIM_PORT)/sdk \
		VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true; \
	echo "--- vms ---";        $(BINARY) vms; \
	echo "--- datastores ---"; $(BINARY) datastores; \
	echo "--- vswitches ---";  $(BINARY) vswitches; \
	pg=$$($(BINARY) vswitches | awk '$$2=="distributed" {print $$3; exit}'); \
	if [ -z "$$pg" ]; then echo "no distributed portgroup discovered"; exit 1; fi; \
	echo "--- vswitches --portgroup $$pg ---"; \
	$(BINARY) vswitches --portgroup "$$pg"; \
	echo "verify: OK"

clean:
	rm -rf bin
