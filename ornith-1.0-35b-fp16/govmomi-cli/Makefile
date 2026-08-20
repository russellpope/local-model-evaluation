.PHONY: build test verify clean staticcheck

BINARY := govmomi-cli

build:
	go build -o $(BINARY) ./...

test:
	go test -race -count=1 ./...

staticcheck:
	staticcheck ./...

gosec:
	gosec -quiet ./... || true

govulncheck:
	govulncheck ./... || true

verify: lint vet staticcheck test sec vulncheck
	@echo "all checks passed"

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt failures:"; gofmt -l .; exit 1; }

vet:
	go vet ./...

sec: gosec

vulncheck: gosec govulncheck

clean:
	rm -f $(BINARY)
