# GPU Telemetry Pipeline (gpu-tel)

A high-performance, scalable telemetry pipeline for collecting, processing, and analyzing GPU metrics in AI/ML clusters.

## Features

- **Real-time GPU Metrics Collection**: Capture detailed GPU metrics including utilization, memory usage, and temperature
- **Scalable Architecture**: Built with microservices for horizontal scaling
- **Message Queue Integration**: Uses gRPC-based message queue for reliable message delivery
- **Persistent Storage**: Stores metrics in PostgreSQL for historical analysis
- **Containerized**: Easy deployment with Docker and Kubernetes
- **Monitoring**: Built-in OpenTelemetry integration for observability

## Architecture

The system consists of several microservices:

1. **Telemetry Collector**: Collects and processes GPU metrics
2. **Message Queue Service**: Handles message routing and delivery
3. **API Server**: Provides REST/gRPC endpoints for querying metrics
4. **Telemetry Streamer**: Streams real-time metrics to connected clients

```
├── api/                  # API definitions (gRPC/OpenAPI)
├── cmd/                  # Main applications
│   ├── api-server/       # API server implementation
│   ├── mq-service/       # Message queue service
│   ├── telemetry-collector/  # Metrics collection service
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
- PostgreSQL 13+
- Make (for development)

## Getting Started

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/sreeram77/gpu-tel.git
   cd gpu-tel
   ```

2. Start dependencies:
   ```bash
   docker-compose up -d postgres
   ```

3. Build and run services:
   ```bash
   make build
   make run
   ```

### Configuration

Edit `configs/config.yaml` to configure the services:

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: postgres
  dbname: gpu_telemetry
  sslmode: disable

message_queue:
  address: "localhost:50051"

collector:
  batch_size: 100
  max_in_flight: 1000
  ack_timeout_seconds: 30
  worker_count: 3
```

## API Documentation

API documentation is available in `api/openapi.yaml` and can be viewed using Swagger UI.

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Linting

```bash
make lint
```

## Local Development

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- PostgreSQL 13+
- Make
- Kind (Kubernetes in Docker)
- kubectl
- Helm 3+

### Quick Start with Kind

1. Set up a local Kind cluster with a container registry:
   ```bash
   make dev-setup
   ```

2. Build and load all Docker images into the Kind cluster:
   ```bash
   make docker-build VERSION=latest
   make kind-load-images
   ```

3. Deploy the application to Kind:
   ```bash
   make dev-deploy
   ```

4. Port-forward the API service:
   ```bash
   kubectl port-forward -n gpu-tel svc/gpu-tel-api-service 8080:8080
   ```

## Deployment

### Kubernetes with Helm

For deploying to a Kubernetes cluster:

1. Add the Helm repository:
   ```bash
   helm repo add bitnami https://charts.bitnami.com/bitnami
   helm repo update
   ```

2. Create the namespace:
   ```bash
   kubectl create namespace gpu-tel
   ```

3. Deploy using the included Helm chart:
   ```bash
   helm upgrade --install gpu-tel ./deploy/charts/gpu-tel \
     --namespace gpu-tel \
     --set postgresql.auth.postgresPassword=postgres \
     --set postgresql.auth.password=mysecretpassword \
     --set postgresql.auth.database=gputel
   ```

4. Check the deployment status:
   ```bash
   kubectl get pods -n gpu-tel
   ```

5. Access the API:
   ```bash
   kubectl port-forward -n gpu-tel svc/gpu-tel-api-service 8080:8080
   ```
   Then access the API at `http://localhost:8080`

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
