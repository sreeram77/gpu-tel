# GPU Telemetry Streamer

A service that streams GPU telemetry data from a CSV file to a message queue for processing by downstream services.

## Features

- **CSV Data Ingestion**: Loads GPU metrics from a CSV file
- **Batch Processing**: Streams metrics in configurable batch sizes
- **Configurable Intervals**: Adjustable streaming interval between batches
- **Graceful Shutdown**: Ensures clean shutdown on termination
- **gRPC Integration**: Publishes metrics to a message queue via gRPC
- **Container Ready**: Designed to run in containerized environments

## Architecture

The telemetry-streamer consists of the following main components:

1. **CSV Loader**: Loads and parses GPU metrics from a CSV file
2. **Batch Processor**: Groups metrics into batches for efficient processing
3. **Message Publisher**: Streams batches to a message queue via gRPC
4. **Configuration Manager**: Handles service configuration and defaults
5. **Lifecycle Controller**: Manages service startup and shutdown

## Prerequisites

- Go 1.18+
- Access to a message queue service (e.g., the mq-service)
- GPU metrics CSV file (default: `test-data/metrics.csv`)

## Configuration

The service can be configured using environment variables or a configuration file. Key configuration options include:

```yaml
telemetry:
  stream_interval: "1s"    # Interval between batch processing
  batch_size: 10           # Number of metrics to process in each batch
  metrics_path: "./test-data/metrics.csv"  # Path to metrics CSV file

message_queue:
  address: "localhost:50051"  # Address of the message queue service
```

### CSV File Format

The expected CSV format should include the following columns:
- `timestamp`: ISO 8601 timestamp
- `metric_name`: Name of the GPU metric
- `gpu_id`: GPU identifier
- `device`: Device identifier
- `uuid`: GPU UUID
- `modelName`: GPU model name
- `Hostname`: Hostname of the machine
- `container`: Container identifier (if applicable)
- `pod`: Kubernetes pod name (if applicable)
- `namespace`: Kubernetes namespace (if applicable)
- `value`: Numeric metric value
- `labels_raw`: Additional labels in raw format

## Building and Running

### Building

```bash
# Build the service
make telemetry-streamer
```

### Running

```bash
# Run with default configuration
make run-streamer

# Run with custom configuration
make telemetry-streamer
./telemetry-streamer --config /path/to/config.yaml
```

## Metrics and Logging

The service logs important events including:
- Service startup and shutdown
- Batch processing statistics
- Error conditions
- Connection status with the message queue

Logs are output in a structured JSON format with timestamps and log levels.

## Deployment

The service is containerized and can be deployed using Docker or Kubernetes. A sample Dockerfile is provided in the repository.

## Future Enhancements

- [ ] Add support for multiple input formats (JSON, Prometheus, etc.)
- [ ] Implement metrics endpoint for monitoring
- [ ] Add support for dynamic metric sources
- [ ] Implement backpressure handling
- [ ] Add support for authentication with message queue
- [ ] Add support for message compression

## License

[Specify License]

## Contributing

[Contribution guidelines and process]
