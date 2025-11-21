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
API_GATEWAY_BIN=api-gateway

# Directories
BIN_DIR=bin
PROTO_DIR=api/v1/mq

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
	@echo "  all         Build all binaries (default)"
	@echo "  build       Build all binaries"
	@echo "  proto       Generate protobuf code"
	@echo "  test        Run tests"
	@echo "  clean       Remove build artifacts"
	@echo "  deps        Install dependencies"
	@echo "  run-mq      Run message queue service"
	@echo "  run-streamer Run telemetry streamer"
	@echo "  run-collector Run telemetry collector"
	@echo "  run-api     Run API gateway"

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

# Build all binaries
build: build-mq build-streamer build-collector build-api

# Build message queue service
build-mq:
	@echo "Building message queue service..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(MQ_SERVICE_BIN) ./cmd/mqservice

# Build telemetry streamer
build-streamer:
	@echo "Building telemetry streamer..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(STREAMER_BIN) ./cmd/streamer

# Build telemetry collector
build-collector:
	@echo "Building telemetry collector..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(COLLECTOR_BIN) ./cmd/collector

# Build API gateway
build-api:
	@echo "Building API gateway..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(API_GATEWAY_BIN) ./cmd/api-gateway

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
	./$(BIN_DIR)/$(API_GATEWAY_BIN)

# Generate mocks for testing
mock:
	@echo "Generating mocks..."
	$(GOGET) github.com/golang/mock/mockgen@latest
	go generate ./...
