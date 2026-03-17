CMD_DIR := ./cmd/kestrel

.PHONY: build build-web copy-web test dev-api dev-web clean

build-web:
	cd web && npm run build

copy-web: build-web
	mkdir -p cmd/kestrel/web/dist
	cp -r web/dist/* cmd/kestrel/web/dist/

build: copy-web
	go build -o kestrel $(CMD_DIR)/...

test:
	go test ./... -coverprofile=coverage.out

dev-api:
	go run $(CMD_DIR)

dev-web:
	cd web && npm run dev

clean:
	rm -rf kestrel kestrel.exe cmd/kestrel/web/ web/dist/
