BINARY := bin/vsphere-inventory

.PHONY: all build vet test fmt tidy verify clean

all: build

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

test:
	go test ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

# Full self-check: static analysis, unit tests, build, then a live smoke test
# of every subcommand against the vcsim simulator (started and torn down here).
verify:
	./scripts/verify.sh

clean:
	rm -rf bin
