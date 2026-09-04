BINARY     := vsphere-inventory
VCSIM_ADDR ?= 127.0.0.1:8989
SDK_URL    := https://$(VCSIM_ADDR)/sdk

.PHONY: all build vet fmt-check test verify clean

all: build

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }

test:
	go test ./...

verify: fmt-check vet build test
	@echo ">> starting vcsim on $(VCSIM_ADDR)"
	@go tool vcsim -l $(VCSIM_ADDR) -vm 8 -ds 3 -pg 3 > vcsim.log 2>&1 & echo $$! > vcsim.pid; \
	trap 'kill $$(cat vcsim.pid) 2>/dev/null || true; \
	      sim=$$(sed -n "s/.*GOVC_SIM_PID=\([0-9]*\).*/\1/p" vcsim.log 2>/dev/null | head -1); \
	      [ -n "$$sim" ] && kill $$sim 2>/dev/null || true; \
	      port=$$(echo "$(VCSIM_ADDR)" | sed "s/.*://"); \
	      kill $$(lsof -t -nP -iTCP:$$port -sTCP:LISTEN) 2>/dev/null || true; \
	      rm -f vcsim.pid vcsim.log' EXIT; \
	ready=0; \
	for i in $$(seq 1 50); do \
	  if curl -ks -o /dev/null "$(SDK_URL)"; then ready=1; break; fi; \
	  sleep 0.2; \
	done; \
	if [ "$$ready" -ne 1 ]; then echo "vcsim did not become ready on $(VCSIM_ADDR)"; cat vcsim.log; exit 1; fi; \
	export VSPHERE_URL="$(SDK_URL)" VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true; \
	set -e; \
	echo ">> vms"; ./$(BINARY) vms; \
	echo ">> datastores"; ./$(BINARY) datastores; \
	echo ">> vswitches"; ./$(BINARY) vswitches; \
	pg=$$(./$(BINARY) vswitches | awk '$$2 == "distributed" { n = $$3; for (i = 4; i <= NF - 5; i++) n = n " " $$i; if (n !~ /Uplinks/) { print n; exit } }'); \
	if [ -z "$$pg" ]; then echo "could not discover a distributed port group"; exit 1; fi; \
	echo ">> vswitches --portgroup $$pg"; ./$(BINARY) vswitches --portgroup "$$pg"; \
	spg=$$(./$(BINARY) vswitches | awk '$$2 == "standard" { n = $$3; for (i = 4; i <= NF - 5; i++) n = n " " $$i; print n; exit }'); \
	if [ -n "$$spg" ]; then echo ">> vswitches --portgroup $$spg"; ./$(BINARY) vswitches --portgroup "$$spg"; fi; \
	echo ">> verify OK"

clean:
	rm -f $(BINARY) vcsim.log vcsim.pid
