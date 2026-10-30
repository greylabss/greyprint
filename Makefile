.PHONY: test matrix lint build golden

test:
	go test -race ./...

matrix:
	go test -race -tags matrix ./internal/render

lint:
	golangci-lint run

build:
	go build -o bin/greyprint ./cmd/greyprint

golden:
	go test ./internal/render -update
