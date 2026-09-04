BINARY := bin/vsphere-inventory

.PHONY: build test vet fmt tidy verify clean

## build: compile the binary into bin/
build:
	go build -o $(BINARY) .

## test: run the unit tests (hermetic, uses govmomi's embedded simulator)
test:
	go test -count=1 ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	gofmt -l -w .

## tidy: reconcile go.mod/go.sum after changing dependencies
tidy:
	go mod tidy

## verify: full gate — vet + tests + build + live vcsim run of every subcommand
verify:
	./scripts/verify.sh

## clean: remove build artifacts
clean:
	rm -rf bin
