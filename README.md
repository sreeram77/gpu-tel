# GPU Telemetry Pipeline (gpu-tel)

A high-performance, scalable telemetry pipeline for collecting, processing, and analyzing GPU metrics in AI/ML clusters.

## Features

- **Real-time GPU Metrics Collection**: Capture detailed GPU metrics including utilization, memory usage, and temperature
- **Scalable Architecture**: Built with microservices for horizontal scaling
- **Message Queue Integration**: Uses gRPC-based message queue for reliable message delivery
- **Efficient Processing**: Optimized for high-throughput telemetry data processing
- **Containerized**: Easy deployment with Docker and Kubernetes
- **Monitoring**: Built-in OpenTelemetry integration for observability

## Services

This project consists of several microservices, each with its own documentation:

1. [Message Queue Service (mq-service)](./cmd/mq-service/readme.md) - gRPC-based message queue for reliable communication between services
2. [Sink](./cmd/sink/README.md) - Processes and stores GPU telemetry data 
3. [Telemetry Streamer](./cmd/telemetry-streamer/readme.md) - Streams telemetry data from CSV to the message queue

## Architecture

The system consists of several microservices:

1. **Sink**: Processes and stores GPU metrics, and exposes HTTP APIsd 
2. **Message Queue Service**: Handles message routing and delivery
3. **Telemetry Streamer**: Streams telemetry data from CSV to the message queue

```
├── api/                  # API definitions (gRPC/OpenAPI)
├── cmd/                  # Main applications
│   ├── mq-service/       # Message queue service
│   ├── sink/             # Sink service for processing and storing metrics
│   └── telemetry-streamer/   # Real-time streaming service
├── configs/              # Configuration files
├── internal/             # Private application code
│   ├── api/              # API handlers
│   ├── config/           # Configuration loading
│   ├── mq/               # Message queue implementation
│   └── storage/          # Storage abstractions
└── test-data/            # Test data and fixtures
```

## Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Make (for development)

## Getting Started

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/sreeram77/gpu-tel.git
   cd gpu-tel
   ```

2. Build and run services on kind:
   ```bash
   make kind-all
   ```
   The API server will be available at http://localhost:8080/swagger

3. To run test cases:
   ```bash
   make test
   ```
   OR only integration tests
   ```bash
   make integration-test
   ```
4. To view test coverage:
   ```bash
   make test-cover-all
   ```

## API Documentation

API documentation is available in `api/openapi.yaml` and can be viewed using Swagger UI.

## Development

### Building

```bash
make build
```

### Testing

Run all tests:
```bash
make test
```

Run integration-test:
```bash
make integration-test
```

View total test coverage:
```bash
make test-cover-all
```

Generate and view test coverage report on the browser:
```bash
make test-cover-show
```

## Local Development

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Make
- Kind (Kubernetes in Docker)
- kubectl
- Helm 3+

### Quick Start with Kind (Single Command)

The easiest way to set up a complete development environment is using the `kind-all` make target, which will:
1. Create a local Kind cluster
2. Build all Docker images
3. Load the images into the Kind cluster
4. Install all dependencies using Helm

```bash
make kind-all
```

After the setup completes, you can access the sink service in http://localhost:8080 by port-forwarding:
```bash
make kind-port-forward
```

Then access the sink service at `http://localhost:8080`

### Local Development without Kubernetes

You can also run the services directly on your local machine without Kubernetes:

1. **Run Individual Services**:

   - **Sink**:
     ```bash
     make run-sink
     ```

   - **Message Queue Service**:
     ```bash
     make run-mq
     ```

   - **Telemetry Streamer**:
     ```bash
     make run-streamer
     ```

2. **Environment Variables**:
   You can customize the services using environment variables. Common variables include:
   ```bash
   # API server configuration
   API_PORT=8080
   
   # Message queue configuration
   MQ_ADDRESS=localhost:50051
   ```

3. **Verify Services**:
   - API: `http://localhost:8080`
   - Health Check: `http://localhost:8080/health`

### Manual Setup with Kind

If you prefer more control over the setup process, you can run the steps manually:

1. Set up a local Kind cluster with a container registry:
   ```bash
   make kind-create
   ```

2. Build and load all Docker images into the Kind cluster:
   ```bash
   make docker-build VERSION=latest
   make kind-load-images
   ```

3. Deploy the application to Kind:
   ```bash
   make helm-install
   ```

4. Port-forward the API service:
   ```bash
   make kind-port-forward
   ```

### Common Tasks

- View cluster status: `make kind-status`
- View logs: `make kind-logs`
- Clean up: `make kind-clean`
- Delete and recreate the cluster: `make kind-restart`

### Production Deployment

For production, make sure to:

1. Set appropriate resource requests and limits in the Helm values
2. Configure proper secrets management
3. Enable ingress and TLS
4. Set up monitoring and alerting
5. Configure persistent storage for PostgreSQL

Example production values:
```yaml
# production-values.yaml
replicaCount: 3

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 100m
    memory: 128Mi

postgresql:
  enabled: true
  auth:
    postgresPassword: ${DB_PASSWORD}
    password: ${DB_APP_PASSWORD}
    database: gputel_prod
  persistence:
    size: 20Gi
    storageClass: standard

service:
  type: LoadBalancer

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: gpu-tel.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: gpu-tel-tls
      hosts:
        - gpu-tel.yourdomain.com
```

Deploy with:
```bash
helm upgrade --install gpu-tel ./deploy/charts/gpu-tel \
  -f production-values.yaml \
  --namespace gpu-tel
```

## Monitoring

The system exposes Prometheus metrics at `/metrics` on each service.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

[Specify License]

## Support

For support, please open an issue in the GitHub repository.
