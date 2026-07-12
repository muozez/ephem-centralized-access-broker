.PHONY: all build test clean run-api run-cli docker-up docker-down fmt lint help

# Variables
BINARY_DIR=bin
CLI_BINARY=$(BINARY_DIR)/ephem
API_BINARY=$(BINARY_DIR)/ephem-api

all: build

## build: Build CLI, API and Agent binaries
build:
	@echo "Building binaries..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(CLI_BINARY) ./cmd/ephem
	go build -o $(API_BINARY) ./cmd/ephem-api
	go build -o $(BINARY_DIR)/ephem-agent ./cmd/ephem-agent
	@echo "Build complete."

## run-api: Run the API server locally
run-api:
	go run ./cmd/ephem-api

## run-cli: Run the CLI tool locally
run-cli:
	go run ./cmd/ephem

## docker-up: Start PostgreSQL using Docker Compose
docker-up:
	docker compose up -d

## docker-down: Stop PostgreSQL and remove volumes
docker-down:
	docker compose down -v

## test: Run unit tests
test:
	go test -v ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BINARY_DIR)
	@echo "Clean complete."

## fmt: Run gofmt on all packages
fmt:
	go fmt ./...

## lint: Run golangci-lint if installed
lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint is not installed. Skip."; \
	fi

## help: Display this help message
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
