# Message Queue Service (mq-service)

A high-performance, in-memory message queue service that provides publish-subscribe and point-to-point messaging patterns over gRPC.

## Features

- **Publish-Subscribe Messaging**: Multiple consumers can subscribe to topics and receive published messages
- **Message Acknowledgment**: Consumers can acknowledge message processing
- **In-Memory Storage**: Fast message storage with optional persistence (to be implemented)
- **gRPC API**: Efficient binary protocol for high-performance communication
- **Concurrent Processing**: Handles multiple publishers and subscribers concurrently
- **Message Filtering**: Basic message filtering capabilities

## API Endpoints

### Publisher Service
- **Publish**: Stream messages to a topic
- **HealthCheck**: Check service health status

### Subscriber Service
- **Subscribe**: Subscribe to messages from a topic
- **Acknowledge**: Acknowledge message processing
- **HealthCheck**: Check service health status

## Getting Started

### Prerequisites

- Go 1.18+
- Protocol Buffers compiler (protoc)
- gRPC tools

### Building

```bash
# Build the service
make build-mq

```

### Running

```bash
# Run with default configuration
make run-mq
```

### Configuration

The service can be configured using a YAML configuration file. See `configs/config.yaml` for available options.

## Architecture

The mq-service consists of the following main components:

1. **gRPC Server**: Handles incoming gRPC requests
2. **Message Store**: In-memory storage for messages with topic-based partitioning
3. **Publisher Service**: Handles message publishing operations
4. **Subscriber Service**: Manages subscriptions and message delivery

## Message Flow

1. **Publishing**:
   - Publisher connects to the gRPC server
   - Messages are published to specific topics
   - Messages are stored in the message store

2. **Subscribing**:
   - Subscriber connects to the gRPC server
   - Subscriber subscribes to a topic
   - Messages are delivered to subscribers
   - Subscribers acknowledge message processing

## Monitoring

The service exposes gRPC health check endpoints that can be used for monitoring and load balancing.

## Limitations

- Current implementation is in-memory only (messages are lost on service restart)
- No built-in authentication/authorization
- No message persistence to disk

## Future Enhancements

- [ ] Add persistent storage backend
- [ ] Add Dead letter queue
- [ ] Implement authentication and authorization
- [ ] Add message TTL and retention policies
- [ ] Add metrics and monitoring
- [ ] Support for message replay

## License

[Specify License]

## Contributing

[Contribution guidelines and process]
