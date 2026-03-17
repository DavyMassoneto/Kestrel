CMD_DIR := ./cmd/kestrel

.PHONY: build test dev-api

build:
	go build $(CMD_DIR)/...

test:
	go test ./... -coverprofile=coverage.out

dev-api:
	go run $(CMD_DIR)
