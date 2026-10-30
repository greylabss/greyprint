.PHONY: run build test tidy

run:
	go run ./cmd/api

build:
	go build ./cmd/api

test:
	go test ./...

tidy:
	go mod tidy

migrate:
	go run ./cmd/migrate
