BIN     := bin/vsphere-inventory
VCSIM   := go -C tools/vcsim run .

.PHONY: all build vet test fmt verify vcsim clean

all: build

build:
	go build -o $(BIN) .

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal main.go

test:
	go test ./...

# Full gate: gofmt check, vet, tests, then every subcommand driven against
# a live vcsim simulator (started and torn down by the script).
verify:
	./scripts/verify.sh

# Run the vendored vcsim in the foreground (8 VMs, 3 datastores, 3 DVPGs).
vcsim:
	$(VCSIM) -l 127.0.0.1:8989 -vm 8 -ds 3 -pg 3

clean:
	rm -rf bin
