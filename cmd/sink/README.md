# GPU Telemetry Sink Service

The Sink Service is a critical component of the GPU Telemetry system that receives, processes, and stores GPU metrics data.

## Overview

The Sink Service provides the following key features:

- **gRPC Server**: Listens for incoming telemetry data from streamers
- **Message Queue Integration**: Processes messages from the configured message queue
- **Data Storage**: Persists telemetry data to the configured storage backend
- **REST API**: Provides HTTP endpoints for querying telemetry data
- **Swagger Documentation**: Interactive API documentation

## Configuration

The service can be configured using environment variables or a configuration file. The following configuration options are available:

### Environment Variables

```bash
# Server configuration
PORT=8080
ENV=development
LOG_LEVEL=info

# Message Queue Configuration
MQ_ADDRESS=localhost:50051
MQ_TOPIC=gpu_metrics

### Configuration File

Alternatively, you can use a YAML configuration file (default: `configs/config.yaml`):

```yaml
server:
  port: 8080
  env: development
  log_level: info

message_queue:
  address: "localhost:50051"
  topic: "gpu_metrics"
```

## API Endpoints

The service exposes the following HTTP endpoints:

- `GET /health` - Health check endpoint
- `GET /api/v1/gpus` - List all available GPUs
- `GET /api/v1/gpus/:id/telemetry` - Get telemetry data for a specific GPU

### Interactive Documentation

When running locally, you can access the Swagger UI at:
```
http://localhost:8080/swagger
```

## Building and Running

### Prerequisites

- Go 1.19+
- Protocol Buffer Compiler (protoc)
- Go plugins for protoc

### Building

```bash
# Build the binary
make build-sink

# Or build with Docker
make docker-build-sink
```

### Running

```bash
# Run directly
./bin/sink

# Or with Docker
make docker-run-sink

# Or with custom config
CONFIG_PATH=/path/to/config.yaml ./bin/sink
```

## Development

### Running Tests

```bash
make test
```

### Code Generation

If you modify any protocol buffer definitions, you'll need to regenerate the Go code:

```bash
make generate
```

## Deployment

The service can be deployed as a container using the provided Dockerfile. See the main project README for Kubernetes deployment instructions.

## Monitoring

The service exposes Prometheus metrics at `/metrics` when running in production mode.

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
