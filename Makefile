.PHONY: proto generate test build clean deps run help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOTOOL=$(GOCMD) tool

# Binary names
MQ_SERVICE_BIN=mq-service
STREAMER_BIN=telemetry-streamer
COLLECTOR_BIN=telemetry-collector
API_SERVER_BIN=api-server

# Directories
BIN_DIR=bin
PROTO_DIR=api/v1/mq
API_DOCS_DIR=api

# Build flags
LDFLAGS=-ldflags="-s -w"
GOBUILD_CMD=$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$@ ./cmd/$@

# Default port for API server
API_PORT?=8080

# Protobuf
PROTOC_GEN_GO := $(GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GOPATH)/bin/protoc-gen-go-grpc
PROTOC := $(shell which protoc)

# Check if protoc and plugins are installed
PROTOC_OK := $(shell which protoc)
PROTOC_GEN_GO_OK := $(shell which protoc-gen-go)
PROTOC_GEN_GO_GRPC_OK := $(shell which protoc-gen-go-grpc)

# Default target
all: build

## help: Display this help message
help:
	@echo "\nAvailable targets:"
	@echo "  all              Build all binaries (default)"
	@echo "  build            Build all binaries"
	@echo "  build-api        Build API server"
	@echo "  run-api          Run API server (port can be set with API_PORT, default: 8080)"
	@echo "  proto            Generate protobuf code"
	@echo "  test             Run tests"
	@echo "  test-api         Run API tests"
	@echo "  clean            Remove build artifacts"
	@echo "  deps             Install dependencies"
	@echo "  deps-api         Install API server dependencies"
	@echo "  lint             Run linters"
	@echo "  format           Format source code"
	@echo "  run-mq           Run message queue service"
	@echo "  run-streamer     Run telemetry streamer"
	@echo "  run-collector    Run telemetry collector"

# Install protoc and plugins if not present
check-protoc:
	@if [ -z "$(PROTOC_OK)" ]; then \
		echo "Error: protoc is required. Please install it first."; \
		echo "On Ubuntu/Debian: sudo apt-get install protobuf-compiler"; \
		echo "On macOS: brew install protobuf"; \
		exit 1; \
	fi

check-protoc-go:
	@if [ -z "$(PROTOC_GEN_GO_OK)" ]; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	fi

check-protoc-go-grpc:
	@if [ -z "$(PROTOC_GEN_GO_GRPC_OK)" ]; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi

# Generate protobuf code
proto: check-protoc check-protoc-go check-protoc-go-grpc
	@echo "Generating protobuf files..."
	@mkdir -p $(PROTO_DIR)
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       $(PROTO_DIR)/mq.proto

# Build targets
build: $(BIN_DIR) $(MQ_SERVICE_BIN) $(STREAMER_BIN) $(COLLECTOR_BIN) $(API_SERVER_BIN)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(MQ_SERVICE_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

$(STREAMER_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

$(COLLECTOR_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

$(API_SERVER_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

## API Server
deps-api:
	@echo "Installing API server dependencies..."
	@$(GOGET) -u github.com/gin-gonic/gin
	@$(GOGET) -u github.com/rs/zerolog

build-api: $(BIN_DIR) $(API_SERVER_BIN)

run-api: build-api
	@echo "Starting API server on port $(API_PORT)..."
	@API_PORT=$(API_PORT) $(BIN_DIR)/$(API_SERVER_BIN)

test-api:
	@echo "Running API tests..."
	@cd internal/api && $(GOTEST) -v -coverprofile=coverage.out ./...
	@$(GOTOOL) cover -html=internal/api/coverage.out -o internal/api/coverage.html

## Code quality
lint:
	@echo "Running linters..."
	@$(GOGET) -u golang.org/x/lint/golint
	@golint ./...
	@$(GOGET) -u honnef.co/go/tools/cmd/staticcheck
	@staticcheck ./...

format:
	@echo "Formatting code..."
	@$(GOCMD) fmt ./...
	@$(GOCMD) vet ./...

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Clean build files
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOGET) -u google.golang.org/protobuf/cmd/protoc-gen-go
	$(GOGET) -u google.golang.org/grpc/cmd/protoc-gen-go-grpc
	$(GOGET) -u github.com/stretchr/testify/assert

# Run message queue service
run-mq: build-mq
	@echo "Starting message queue service..."
	./$(BIN_DIR)/$(MQ_SERVICE_BIN)

# Run telemetry streamer
run-streamer: build-streamer
	@echo "Starting telemetry streamer..."
	./$(BIN_DIR)/$(STREAMER_BIN)

# Run telemetry collector
run-collector: build-collector
	@echo "Starting telemetry collector..."
	./$(BIN_DIR)/$(COLLECTOR_BIN)

# Run API gateway
run-api: build-api
	@echo "Starting API gateway..."
	./$(BIN_DIR)/$(API_SERVER_BIN)

# Generate mocks for testing
mock:
	@echo "Generating mocks..."
	$(GOGET) github.com/golang/mock/mockgen@latest
	go generate ./...
