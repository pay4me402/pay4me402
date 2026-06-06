APP := payformeproxy
BIN_DIR := bin
SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest

.PHONY: build run tidy fmt test generate clean

build: generate
	go build -o $(BIN_DIR)/$(APP) ./cmd/$(APP)

run: generate
	go run ./cmd/$(APP)

tidy:
	go mod tidy

fmt:
	gofmt -w ./cmd ./internal

test: generate
	go test ./...

generate:
	$(SQLC) generate

clean:
	rm -rf $(BIN_DIR)
