# GPU Telemetry Collector

A service that collects, processes, and stores GPU telemetry data from message queues into persistent storage.

## Features

- **High-Throughput Processing**: Efficiently processes telemetry data in batches
- **Persistent Storage**: Stores telemetry data in PostgreSQL for historical analysis
- **Reliable Message Processing**: Implements message acknowledgment and retry mechanisms
- **Scalable**: Supports multiple worker goroutines for parallel processing
- **Observability**: Includes metrics and logging for monitoring and debugging
- **Graceful Shutdown**: Ensures no data loss during service termination

## Architecture

The telemetry-collector consists of the following main components:

1. **Message Consumer**: Subscribes to GPU telemetry messages from the message queue
2. **Batch Processor**: Groups incoming messages into batches for efficient storage
3. **Storage Interface**: Abstracts the underlying storage implementation (PostgreSQL)
4. **Metrics Collector**: Tracks performance metrics and processing statistics
5. **Worker Pool**: Manages concurrent processing of message batches

## Prerequisites

- Go 1.18+
- PostgreSQL 13+
- Access to a message queue service (e.g., the mq-service)

## Configuration

The service can be configured using environment variables or a configuration file. Key configuration options include:

```yaml
message_queue:
  address: "localhost:50051"  # Address of the message queue service

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  dbname: "gpu_telemetry"
  sslmode: "disable"

collector:
  batch_size: 100            # Number of messages to process in each batch
  max_in_flight: 1000        # Maximum number of in-flight messages
  ack_timeout_seconds: 30    # Timeout for message acknowledgment
  worker_count: 3            # Number of worker goroutines
```

## Building and Running

### Building

```bash
# Build the service
make telemetry-collector
```

### Running

```bash
# Run with default configuration
make run-collector

# Run with custom config
make telemetry-collector
./telemetry-collector --config /path/to/config.yaml
```

## Metrics

The service exposes the following metrics:

- `collector_messages_processed_total`: Total number of messages processed
- `collector_processing_time_seconds`: Time taken to process message batches
- `collector_batch_size`: Size of processed message batches
- `collector_errors_total`: Total number of processing errors

## Monitoring and Logging

The service uses structured logging with the following log levels:
- `info`: Service lifecycle events and important operations
- `warn`: Non-critical issues that don't prevent operation
- `error`: Critical errors that may affect functionality
- `debug`: Detailed debugging information

## Deployment

The service is containerized and can be deployed using Docker or Kubernetes. A sample Dockerfile is provided in the repository.

## Future Enhancements

- [ ] Add support for multiple storage backends
- [ ] Implement message deduplication
- [ ] Add rate limiting and backpressure handling
- [ ] Support for custom telemetry processing pipelines
- [ ] Integration with monitoring and alerting systems

## License

[Specify License]

## Contributing

[Contribution guidelines and process]
