BINARY := ai-agent-proto
GOBIN  := $(shell go env GOPATH)/bin
IMAGE  := ai-agent-proto:local

.PHONY: build run test swag fmt lint verify docker-build docker-up docker-down docker-logs

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

# Starts with no configuration at all. Settings come from conf/setup.env when
# that file exists and from built-in defaults otherwise, so a fresh checkout
# runs and serves everything that does not touch a cloud.
run:
	go run ./cmd/$(BINARY)

test:
	go test ./...

swag:
	$(GOBIN)/swag init -g cmd/$(BINARY)/main.go -o api/docs --parseInternal

fmt:
	gofmt -w .

lint:
	$(GOBIN)/golangci-lint run

verify: fmt build test lint

docker-build:
	docker build -t $(IMAGE) .

# Also needs no configuration. Copy .env.example to .env only to point at a real
# CB-Tumblebug or to supply a planning model.
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
