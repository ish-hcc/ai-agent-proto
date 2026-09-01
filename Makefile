BINARY := ai-agent-proto
GOBIN  := $(shell go env GOPATH)/bin

.PHONY: build run swag fmt lint verify

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

run:
	go run ./cmd/$(BINARY)

swag:
	$(GOBIN)/swag init -g cmd/$(BINARY)/main.go -o api/docs --parseInternal

fmt:
	gofmt -w .

lint:
	$(GOBIN)/golangci-lint run

verify: fmt build lint
