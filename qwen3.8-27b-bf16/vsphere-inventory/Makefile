BINARY := vsphere-inventory

.PHONY: all build vet test verify clean

all: build

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

test:
	go test ./...

verify: vet test
	./scripts/verify.sh

clean:
	rm -f $(BINARY)
