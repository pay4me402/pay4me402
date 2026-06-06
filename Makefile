APP := payformeproxy
BIN_DIR := bin

.PHONY: build run tidy fmt test clean

build:
	go build -o $(BIN_DIR)/$(APP) ./cmd/$(APP)

run:
	go run ./cmd/$(APP)

tidy:
	go mod tidy

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)
