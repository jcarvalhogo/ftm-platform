.PHONY: build fmt run-ftp run-backend

build:
	mkdir -p build
	go build -o build/ftm-ftp-server ./cmd/ftm-ftp-server
	go build -o build/ftm-backend ./cmd/ftm-backend

fmt:
	gofmt -w ./cmd ./internal

run-ftp:
	go run ./cmd/ftm-ftp-server -config ./configs/ftp-server.toml

run-backend:
	go run ./cmd/ftm-backend -config ./configs/backend.toml
