.PHONY: build agent run test images clean

BIN       := bin/sandforge
AGENT_BIN := bin/sandforge-agent

build:
	go build -o $(BIN) ./cmd/sandforge

agent:
	GOOS=linux GOARCH=amd64 go build -o $(AGENT_BIN) ./cmd/guest-agent

run: build
	./$(BIN)

test:
	go test ./...

images:
	./scripts/build-images.sh

clean:
	rm -rf bin/
