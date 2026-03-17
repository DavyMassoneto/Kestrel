APP_NAME := kestrel
CMD_DIR  := ./cmd/kestrel

.PHONY: build test dev-api

build:
	go build -o bin/$(APP_NAME) $(CMD_DIR)

test:
	go test ./... -v -race

dev-api:
	go run $(CMD_DIR)
