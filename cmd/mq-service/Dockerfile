# Build stage
FROM --platform=$BUILDPLATFORM gpu-tel-base:latest as builder

# Final stage
FROM gpu-tel-base:${VERSION}

WORKDIR /app/bin

# Copy the binary from the builder
COPY --from=builder /app/bin/mqservice /app/bin/

# Copy configuration
COPY configs/ /app/configs/

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Expose the port the app runs on
EXPOSE 5672

# Expose the gRPC port
EXPOSE 50051

# Command to run the executable
ENTRYPOINT ["./mq-service"]

# Default command arguments
CMD ["--config", "/app/configs/config.yaml"]
