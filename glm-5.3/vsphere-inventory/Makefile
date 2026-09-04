BIN      := bin/vsphere-inventory
VCSIM    := github.com/vmware/govmomi/vcsim
VCSIM_URL    ?= https://127.0.0.1:8989/sdk
VCSIM_FLAGS  ?= -vm 8 -ds 3 -pg 3 -l 127.0.0.1:8989

.PHONY: all build test vet fmt-check verify clean

all: build

build:
	go build -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed for:"; echo "$$out"; exit 1; fi

# verify runs the full acceptance check: static checks and unit tests, then
# an in-process vCenter simulator (vcsim) and every CLI code path against it.
verify: vet fmt-check test build
	@set -eu; \
	VCSIM_BIN=$$(mktemp -t vcsim); \
	LOG=$$(mktemp -t vcsim-log); \
	trap 'kill $$VCSIM_PID 2>/dev/null || true; rm -f "$$VCSIM_BIN" "$$LOG"' EXIT; \
	go build -o "$$VCSIM_BIN" $(VCSIM); \
	echo "== starting vcsim ($(VCSIM_FLAGS)) =="; \
	"$$VCSIM_BIN" $(VCSIM_FLAGS) >"$$LOG" 2>&1 & \
	VCSIM_PID=$$!; \
	ready=0; \
	for i in $$(seq 1 30); do \
	  if curl -ks -o /dev/null "$(VCSIM_URL)"; then ready=1; break; fi; \
	  sleep 1; \
	done; \
	if [ "$$ready" != 1 ]; then echo "vcsim did not become ready; log:"; cat "$$LOG"; exit 1; fi; \
	export VSPHERE_URL=$(VCSIM_URL) VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true; \
	echo; echo "== $(BIN) vms =="; $(BIN) vms; \
	echo; echo "== $(BIN) datastores =="; $(BIN) datastores; \
	echo; echo "== $(BIN) vswitches =="; $(BIN) vswitches; \
	PG=$$($(BIN) vswitches | awk 'NR>1 && $$3 != "-" {print $$3; exit}'); \
	echo; echo "== $(BIN) vswitches --portgroup \"$$PG\" =="; \
	test -n "$$PG" || { echo "no port group found in vswitches output"; exit 1; }; \
	$(BIN) vswitches --portgroup "$$PG"; \
	echo; echo "ALL CHECKS PASSED"

clean:
	rm -rf bin
