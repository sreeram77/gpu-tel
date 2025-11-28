.PHONY: proto generate test integration-test build clean deps run help docker-build docker-push docker-all docker-clean docker-push-all

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
SINK_BIN=sink

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
VERSION?=latest
KIND_CLUSTER_NAME?=gpu-tel-cluster
KUBECONFIG?=${HOME}/.kube/config

# Kind configuration
KIND_CONFIG?=deploy/kind/kind-config.yaml
KIND_REGISTRY=localhost:5000

# Docker image names for local development
MQ_IMAGE_NAME=gpu-tel-mq

# Kubernetes/Helm parameters
HELM_NAMESPACE?=gpu-tel
HELM_RELEASE_NAME?=gpu-tel
HELM_CHART=./deploy/charts/gpu-tel
STREAMER_IMAGE_NAME=gpu-tel-streamer
SINK_IMAGE_NAME=gpu-tel-sink

# Image tags for local development
IMAGE_TAG?=latest

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
	@echo "  proto           Generate protobuf code"
	@echo "\nTest targets:"
	@echo "  test            Run all tests with coverage"
	@echo "  integration-test Run integration tests"
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
	@echo "  docker-tag       Tag images for local registry"
	@echo "  docker-push      Push images to local registry"
	
	@echo "\nKubernetes/Helm targets:"
	@echo "  helm-install     Install/upgrade the Helm release"
	@echo "  helm-uninstall   Uninstall the Helm release"
	@echo "  helm-status      Show status of the Helm release"
	@echo "  helm-template    Template the Helm charts"
	@echo "  kind-create      Create a local Kind cluster"
	@echo "  kind-delete      Delete the local Kind cluster"
	@echo "  kind-load-images Load local images into Kind"
	
	@echo "\nDevelopment:"
	@echo "  test             Run tests"
	@echo "  clean            Remove build artifacts"
	@echo "  deps             Install dependencies"
	@echo "  lint             Run linters"
	@echo "  format           Format source code"
	@echo "  run-mq           Run message queue service"
	@echo "  run-streamer     Run telemetry streamer"
	@echo "  dev-setup        Setup development environment (Kind cluster + local registry)"
	@echo "  dev-deploy       Build, load images, and deploy to local cluster"

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
build: $(BIN_DIR) $(MQ_SERVICE_BIN) $(STREAMER_BIN) $(SINK_BIN)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(MQ_SERVICE_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

$(STREAMER_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

$(SINK_BIN):
	@echo "Building $@..."
	@$(GOBUILD_CMD)

## integration-test: Run integration tests
integration-test:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./integration/... -count=1

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

# Build individual Docker images
docker-build-mq:
	$(DOCKER_CMD) build -t $(MQ_IMAGE_NAME):$(VERSION) -f cmd/mq-service/Dockerfile .

docker-build-streamer:
	$(DOCKER_CMD) build -t $(STREAMER_IMAGE_NAME):$(VERSION) -f cmd/telemetry-streamer/Dockerfile .

docker-build-sink:
	$(DOCKER_CMD) build -t $(SINK_IMAGE_NAME):$(VERSION) -f cmd/sink/Dockerfile .

# Build all Docker images
docker-build: docker-build-mq docker-build-streamer docker-build-sink

# Tag images for local registry
docker-tag:
	$(DOCKER_CMD) tag $(MQ_IMAGE_NAME):$(IMAGE_TAG) $(KIND_REGISTRY)/$(MQ_IMAGE_NAME):$(IMAGE_TAG)
	$(DOCKER_CMD) tag $(STREAMER_IMAGE_NAME):$(IMAGE_TAG) $(KIND_REGISTRY)/$(STREAMER_IMAGE_NAME):$(IMAGE_TAG)
	$(DOCKER_CMD) tag $(SINK_IMAGE_NAME):$(IMAGE_TAG) $(KIND_REGISTRY)/$(SINK_IMAGE_NAME):$(IMAGE_TAG)

# Push images to local registry
docker-push:
	$(DOCKER_CMD) push $(KIND_REGISTRY)/$(MQ_IMAGE_NAME):$(IMAGE_TAG)
	$(DOCKER_CMD) push $(KIND_REGISTRY)/$(STREAMER_IMAGE_NAME):$(IMAGE_TAG)
	$(DOCKER_CMD) push $(KIND_REGISTRY)/$(SINK_IMAGE_NAME):$(IMAGE_TAG)
# Helm commands
helm-deps:
	helm dependency update $(HELM_CHART)

helm-install: helm-deps
	helm upgrade --install $(HELM_RELEASE_NAME) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set mq-service.image.repository=$(MQ_IMAGE_NAME) \
		--set sink.image.repository=$(SINK_IMAGE_NAME) \
		--set telemetry-streamer.image.repository=$(STREAMER_IMAGE_NAME) \
		--set global.image.tag=$(VERSION) \
		--set global.image.pullPolicy=Never

helm-uninstall:
	helm uninstall $(HELM_RELEASE_NAME) --namespace $(HELM_NAMESPACE)

helm-status:
	helm status $(HELM_RELEASE_NAME) --namespace $(HELM_NAMESPACE)

helm-template:
	helm template $(HELM_RELEASE_NAME) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--set mq-service.image.repository=$(MQ_IMAGE_NAME) \
		--set sink.image.repository=$(SINK_IMAGE_NAME) \
		--set telemetry-streamer.image.repository=$(STREAMER_IMAGE_NAME) \
		--set global.image.tag=$(VERSION) \
		--set global.image.pullPolicy=Never

# Development setup
dev-setup: kind-create

# Full deployment workflow
dev-deploy: docker-build kind-load-images helm-install

# Test coverage file
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Run tests with 1 minute timeout
test:
	@echo "Running tests with 1 minute timeout..."
	$(GOTEST) `go list ./... | grep -v -e './api/v1/mq' -e 'mocks'` -v -timeout=1m  -coverprofile=$(COVERAGE_FILE) -covermode=count ./...


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

## docker-clean: Remove all Docker images
docker-clean:
	@echo "Removing Docker images..."
	-$(DOCKER_CMD) rmi \
		$(SINK_IMAGE_NAME):$(VERSION) \
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
	kind load docker-image $(SINK_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(MQ_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(STREAMER_IMAGE_NAME):$(VERSION) --name $(KIND_CLUSTER_NAME)

## kind-clean: Clean up all resources in the Kind cluster
kind-clean:
	@echo "Cleaning up Kind cluster resources..."
	-helm uninstall $(HELM_RELEASE_NAME) -n $(HELM_NAMESPACE) 2>/dev/null || true
	-kubectl delete namespace $(HELM_NAMESPACE) 2>/dev/null || true

## kind-status: Show status of the deployed application
kind-status:
	@echo "=== Cluster Status ==="
	kubectl get nodes
	@echo "\n=== Pods ==="
	kubectl get pods -n $(HELM_NAMESPACE)
	@echo "\n=== Services ==="
	kubectl get svc -n $(HELM_NAMESPACE)

## kind-logs: View logs from all pods
kind-logs:
	@echo "=== Pod Logs ==="
	@for pod in $$(kubectl get pods -n $(HELM_NAMESPACE) -o name); do \
		echo "\n=== Logs for $$pod ==="; \
		kubectl logs -n $(HELM_NAMESPACE) $$pod --tail=50; \
	done

## kind-port-forward: Forward API service port
kind-port-forward:
	@echo "Forwarding API service port 8080 to localhost:8080..."
	@kubectl port-forward -n $(HELM_NAMESPACE) svc/$(shell kubectl get svc -n $(HELM_NAMESPACE) -o jsonpath='{.items[?(@.spec.ports[].port==8080)].metadata.name}') 8080:8080

## kind-all: Setup and deploy everything to Kind
kind-all: kind-create kind-load-images helm-install wait kind-port-forward

## kind-restart: Clean and redeploy
kind-restart: kind-clean kind-all

wait:
	@echo "Waiting for pods to be ready..."
	@kubectl wait --for=condition=ready pod --all -n $(HELM_NAMESPACE) --timeout=10s

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
run-mq: $(MQ_SERVICE_BIN)
	@echo "Starting message queue service..."
	./$(BIN_DIR)/$(MQ_SERVICE_BIN)

# Run telemetry streamer
run-streamer: $(STREAMER_BIN)
	@echo "Starting telemetry streamer..."
	./$(BIN_DIR)/$(STREAMER_BIN)

# Run sink
run-sink: $(SINK_BIN)
	@echo "Starting sink..."
	./$(BIN_DIR)/$(SINK_BIN)

# Generate mocks for testing
mock:
	@echo "Generating mocks..."
	$(GOGET) github.com/golang/mock/mockgen@v1.6.0
	go generate ./...
