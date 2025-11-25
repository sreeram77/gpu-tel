.PHONY: proto generate test build clean deps run help docker-build docker-push docker-all docker-clean docker-push-all

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

# Docker parameters
DOCKER_CMD=docker
DOCKER_COMPOSE_CMD=docker-compose
VERSION?=dev
KIND_CLUSTER_NAME?=gpu-tel-cluster
KUBECONFIG?=${HOME}/.kube/config

# Kind configuration
KIND_CONFIG?=deploy/kind/kind-config.yaml

# Docker image names for local development
MQ_IMAGE_NAME=gpu-tel-mq
STREAMER_IMAGE_NAME=gpu-tel-streamer
COLLECTOR_IMAGE_NAME=gpu-tel-collector
API_IMAGE_NAME=gpu-tel-api

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
	@echo "\nBuild targets:"
	@echo "  all              Build all binaries (default)"
	@echo "  build           Build all binaries"
	@echo "  build-api       Build API server"
	@echo "  run-api         Run API server (port can be set with API_PORT, default: 8080)"
	@echo "  proto           Generate protobuf code"
	@echo "\nTest targets:"
	@echo "  test            Run all tests with coverage"
	@echo "  test-cover      Generate HTML coverage report"
	@echo "  test-cover-func Show function coverage"
	@echo "  test-cover-pkg  Show package coverage"
	@echo "  test-cover-all  Run all coverage checks"
	@echo "  test-cover-show Open coverage report in browser"
	
	@echo "\nDocker targets (local development):"
	@echo "  docker-build     Build all Docker images"
	@echo "  docker-clean     Remove all Docker images"
	@echo "  docker-up        Start all services with docker-compose"
	@echo "  docker-down      Stop all services with docker-compose"
	@echo "  docker-logs      View logs from all services"
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

# Test coverage file
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Run tests with 1 minute timeout
test:
	@echo "Running tests with 1 minute timeout..."
	$(GOTEST) -v -timeout=1m -coverprofile=$(COVERAGE_FILE) -covermode=count ./...

# Run tests with coverage and generate HTML report
test-cover: test
	@echo "Generating coverage report..."
	@$(GOTOOL) cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

# Show function coverage
test-cover-func: test
	@echo "\nFunction coverage:"
	@$(GOTOOL) cover -func=$(COVERAGE_FILE)

# Show package coverage
test-cover-pkg: test
	@echo "\nPackage coverage:"
	@$(GOTOOL) cover -func=$(COVERAGE_FILE) | grep total:

# Run all coverage checks
test-cover-all: test-cover test-cover-func test-cover-pkg

# Show coverage in web browser
test-cover-show: test-cover
	@echo "Opening coverage report in browser..."
	@xdg-open $(COVERAGE_HTML) 2>/dev/null || open $(COVERAGE_HTML) 2>/dev/null || echo "Could not open browser, please open $(COVERAGE_HTML) manually"

# Clean build files
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BIN_DIR) $(COVERAGE_FILE) $(COVERAGE_HTML)
	@find . -name "*.test" -delete
	@echo "Clean complete"

# Build the base image
docker-build-base:
	@echo "Building base Docker image..."
	$(DOCKER_CMD) build -f Dockerfile.base -t gpu-tel-base:$(VERSION) .

# Build all Docker images
docker-build: docker-build-base
	@echo "Building service Docker images..."
	$(DOCKER_CMD) build -f cmd/api-server/Dockerfile -t $(API_IMAGE_NAME):$(VERSION) .
	$(DOCKER_CMD) build -f cmd/telemetry-collector/Dockerfile -t $(COLLECTOR_IMAGE_NAME):$(VERSION) .
	$(DOCKER_CMD) build -f cmd/telemetry-streamer/Dockerfile -t $(STREAMER_IMAGE_NAME):$(VERSION) .
	$(DOCKER_CMD) build -f cmd/mq-service/Dockerfile -t $(MQ_IMAGE_NAME):$(VERSION) .
	@echo "Docker images built successfully for local development"

## docker-clean: Remove all Docker images
docker-clean:
	@echo "Removing Docker images..."
	-$(DOCKER_CMD) rmi $(API_IMAGE_NAME):$(VERSION) \
		$(COLLECTOR_IMAGE_NAME):$(VERSION) \
		$(MQ_IMAGE_NAME):$(VERSION) \
		$(STREAMER_IMAGE_NAME):$(VERSION) \
		gpu-tel-base:$(VERSION) || true
	@echo "Docker images removed"

## docker-up: Start all services with docker-compose
docker-up:
	$(DOCKER_COMPOSE_CMD) up -d

## docker-down: Stop all services with docker-compose
docker-down:
	$(DOCKER_COMPOSE_CMD) down

## docker-logs: View logs from all services
docker-logs:
	$(DOCKER_COMPOSE_CMD) logs -f

## kind-create: Create a local Kind Kubernetes cluster
kind-create:
	@echo "Creating Kind cluster '$(KIND_CLUSTER_NAME)'..."
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		echo "Cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	else \
		kind create cluster --name $(KIND_CLUSTER_NAME) --config=$(KIND_CONFIG); \
	fi

## kind-delete: Delete the local Kind Kubernetes cluster
kind-delete:
	@echo "Deleting Kind cluster '$(KIND_CLUSTER_NAME)'..."
	@kind delete cluster --name $(KIND_CLUSTER_NAME) || true

## kind-load-images: Load local Docker images into the Kind cluster
kind-load-images: docker-build
	@echo "Loading images into Kind cluster..."
	kind load docker-image $(API_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(COLLECTOR_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(MQ_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(STREAMER_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)

## kind-deploy: Deploy the application to the Kind cluster
kind-deploy: kind-load-images
	@echo "Deploying to Kind cluster..."
	kubectl create namespace gpu-tel --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install gpu-tel ./deploy/charts/gpu-tel \
		--namespace gpu-tel \
		--create-namespace \
		--set postgresql.auth.postgresPassword=postgres \
		--set postgresql.auth.password=mysecretpassword \
		--set postgresql.auth.database=gputel \
		--set images.apiServer.repository=$(API_IMAGE_NAME) \
		--set images.apiServer.tag=$(VERSION) \
		--set images.mqService.repository=$(MQ_IMAGE_NAME) \
		--set images.mqService.tag=$(VERSION) \
		--set images.telemetryCollector.repository=$(COLLECTOR_IMAGE_NAME) \
		--set images.telemetryCollector.tag=$(VERSION) \
		--set images.telemetryStreamer.repository=$(STREAMER_IMAGE_NAME) \
		--set images.telemetryStreamer.tag=$(VERSION)

## kind-undeploy: Remove the application from the Kind cluster
kind-undeploy:
	@echo "Undeploying from Kind cluster..."
	helm uninstall gpu-tel --namespace gpu-tel || true

## kind-status: Show status of the deployed application
kind-status:
	@echo "=== Cluster Status ==="
	kubectl get nodes
	@echo "\n=== Pods ==="
	kubectl get pods -n gpu-tel
	@echo "\n=== Services ==="
	kubectl get svc -n gpu-tel

## kind-logs: View logs from all pods
kind-logs:
	@echo "=== Pod Logs ==="
	@for pod in $$(kubectl get pods -n gpu-tel -o name); do \
		echo "\n=== Logs for $$pod ==="; \
		kubectl logs -n gpu-tel $$pod --tail=50; \
	done

## kind-port-forward: Forward API service port
kind-port-forward:
	@echo "Forwarding API service port 8080 to localhost:8080..."
	@kubectl port-forward -n gpu-tel svc/$(shell kubectl get svc -n gpu-tel -o jsonpath='{.items[?(@.spec.ports[].port==8080)].metadata.name}') 8080:8080

## kind-all: Create cluster and deploy application
kind-all: kind-create kind-deploy kind-status

## kind-clean: Remove cluster and clean up
kind-clean: kind-undeploy kind-delete

## kind-restart: Clean and redeploy
kind-restart: kind-clean kind-all

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOGET) -u google.golang.org/protobuf/cmd/protoc-gen-go
	$(GOGET) -u google.golang.org/grpc/cmd/protoc-gen-go-grpc
	$(GOGET) -u github.com/stretchr/testify/assert@v1.8.4
	$(GOGET) -u github.com/golang/mock/mockgen@v1.6.0
	$(GOGET) -u github.com/rs/zerolog/log
	go install github.com/golang/mock/mockgen@v1.6.0

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
	$(GOGET) github.com/golang/mock/mockgen@v1.6.0
	go generate ./...
