.PHONY: build test vet fmt verify clean

BINARY := vsphere-inventory
MODULE := github.com/example/vsphere-inventory-cli

build:
	go build -o $(BINARY) .

test:
	go test ./... -v -count=1

vet:
	go vet ./...

fmt:
	gofmt -w .

verify: vet test build
	@echo "All checks passed"

clean:
	rm -f $(BINARY)
	go clean -cache

vcsim-start:
	@echo "Starting vcsim..."
	@-pkill -f vcsim 2>/dev/null || true
	@sleep 1
	@VCSIM_PATH=$$(ls -d $$(go env GOMODCACHE)/github.com/vmware/govmomi/vcsim@*/ 2>/dev/null | head -1) && \
	if [ -n "$$VCSIM_PATH" ] && [ -f "$$VCSIM_PATH/main.go" ]; then \
		go run $$VCSIM_PATH/main.go -vm 8 -ds 3 -pg 3 > /tmp/vcsim.log 2>&1 & \
		echo "vcsim PID: $$!"; \
		sleep 3; \
	else \
		echo "vcsim not found in module cache"; \
		exit 1; \
	fi

vcsim-stop:
	@-pkill -f vcsim 2>/dev/null || true
	@echo "vcsim stopped"

vcsim-test: vcsim-start
	@echo "Testing against vcsim..."
	@VSPHERE_URL=https://127.0.0.1:8989/sdk \
	VSPHERE_USERNAME=user \
	VSPHERE_PASSWORD=pass \
	VSPHERE_INSECURE=true \
	./$(BINARY) vms && \
	./$(BINARY) datastores && \
	./$(BINARY) vswitches && \
	echo "All vcsim tests passed"
	@$(MAKE) vcsim-stop
