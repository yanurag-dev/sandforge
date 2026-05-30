.PHONY: build agent run test images clean lint typecheck fmt check

BIN       := bin/sandforge
AGENT_BIN := bin/sandforge-agent

build:
	mkdir -p $(dir $(BIN))
	go build -o $(BIN) ./cmd/sandforge
	@if [ "$$(uname)" = "Darwin" ]; then \
		echo "Signing binary with virtualization entitlements..."; \
		codesign -f -s - --entitlements entitlements.plist $(BIN); \
	fi

agent:
	mkdir -p $(dir $(AGENT_BIN))
	GOOS=linux GOARCH=amd64 go build -o $(AGENT_BIN) ./cmd/guest-agent

run: build
	./$(BIN)

test:
	go test ./...

images:
	./scripts/build-images.sh

clean:
	rm -rf bin/

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

typecheck:
	go vet ./...

fmt:
	go fmt ./...

check: fmt lint typecheck test build
