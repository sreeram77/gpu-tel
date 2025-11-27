# GPU Telemetry API Server

A RESTful API server that provides access to GPU telemetry data stored in PostgreSQL. The server exposes endpoints to query GPU information and their associated telemetry metrics.

## Features

- **RESTful API**: Standardized HTTP endpoints for accessing GPU telemetry data
- **Query by Time Range**: Filter telemetry data by specific time windows
- **Multiple GPU Support**: Manage and query data from multiple GPUs
- **Container Ready**: Designed to run in containerized environments
- **Health Checks**: Built-in health check endpoint for monitoring
- **Request Logging**: Detailed request logging with timing information
- **Graceful Shutdown**: Ensures clean shutdown on termination signals

## API Endpoints

### List GPUs

```http
GET /api/v1/gpus
```

**Response:**
```json
[
  {
    "uuid": "GPU-1234-5678-9012",
    "gpu_id": "0",
    "device": "nvidia0",
    "model_name": "NVIDIA A100",
    "hostname": "gpu-node-1",
    "timestamp": "2023-01-01T12:00:00Z"
  }
]
```

### Get GPU Telemetry

```http
GET /api/v1/gpus/{gpu_id}/telemetry?start_time=2023-01-01T00:00:00Z&end_time=2023-01-02T00:00:00Z
```

**Query Parameters:**
- `start_time`: Start time in RFC3339 format (optional, defaults to 24 hours ago)
- `end_time`: End time in RFC3339 format (optional, defaults to current time)

> **Note**: The old parameter names `start` and `end` are still supported for backward compatibility but are deprecated.

**Response:**
```json
[
  {
    "timestamp": "2023-01-01T12:00:00Z",
    "metric_name": "gpu_utilization",
    "gpu_id": "0",
    "device": "nvidia0",
    "uuid": "GPU-1234-5678-9012",
    "model_name": "NVIDIA A100",
    "hostname": "gpu-node-1",
    "container": "container-123",
    "pod": "gpu-pod-abc123",
    "namespace": "gpu-namespace",
    "value": 42.5,
    "labels_raw": "{\"label1\":\"value1\",\"label2\":\"value2\"}"
  }
]
```

## Prerequisites

- Go 1.18+
- PostgreSQL 13+
- Access to the telemetry database

## Configuration

The service can be configured using environment variables or a configuration file. Key configuration options include:

```yaml
database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  dbname: "gpu_telemetry"
  sslmode: "disable"

server:
  port: 8080
  env: "development"  # or "production"
```

## Building and Running

### Building

```bash
# Build the service
make build-api
```

### Running

```bash
# Run with default configuration
make api-server

# Run with custom configuration
./api-server --config /path/to/config.yaml
```

## Logging

The server logs all HTTP requests with the following information:
- HTTP method and path
- Response status code
- Client IP address
- Request duration
- User agent
- Query parameters (if any)

## Future Enhancements

- [ ] Add authentication and authorization
- [ ] Implement rate limiting
- [ ] Add more query filters (by hostname, container, etc.)
- [ ] Support for pagination of results
- [ ] Add OpenAPI/Swagger documentation
- [ ] Implement caching for frequently accessed data

## License

[Specify License]

## Contributing

[Contribution guidelines and process]
