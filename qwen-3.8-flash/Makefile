BIN     := vsphere-inventory
VCSIM   ?= go run github.com/vmware/govmomi/vcsim

.PHONY: all build test vet fmt verify clean tidy

all: build

build:
	go build -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

# Full check: vet, unit tests, then every subcommand against the bundled
# vcsim simulator (started, driven, and torn down by the script).
verify:
	BIN=./$(BIN) ./scripts/verify.sh

tidy:
	go mod tidy

clean:
	rm -f $(BIN)
