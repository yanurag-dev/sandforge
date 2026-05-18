.PHONY: build run test images clean

BIN := bin/sandforge

build:
	go build -o $(BIN) ./cmd/sandforge

run: build
	./$(BIN)

test:
	go test ./...

images:
	./scripts/build-images.sh

clean:
	rm -rf bin/
