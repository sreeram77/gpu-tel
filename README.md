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

## Deployment

### Docker

```bash
docker-compose up -d
```

### Kubernetes

```bash
kubectl apply -f deploy/kubernetes/
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
