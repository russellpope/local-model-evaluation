GO      ?= go
BIN     := bin/govc-inventory
VCSIM   ?= github.com/vmware/govmomi/vcsim

.PHONY: build vet test verify sim clean

build:
	$(GO) build -o $(BIN) .

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

# Full check: vet, unit tests, then every subcommand driven against the
# bundled vcsim simulator (start → run → diagnose → tear down).
verify:
	./scripts/verify.sh

# Run vcsim manually for ad-hoc poking: then use config.example.yaml or
# VSPHERE_URL=https://127.0.0.1:8989/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass
sim:
	$(GO) run $(VCSIM) -vm 8 -ds 3 -pg 3

clean:
	rm -rf bin
