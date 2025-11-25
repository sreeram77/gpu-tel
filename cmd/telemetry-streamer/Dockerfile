# Build stage
FROM --platform=$BUILDPLATFORM gpu-tel-base:latest as builder

# Final stage
FROM gpu-tel-base:${VERSION}

WORKDIR /app/bin

# Copy the binary from the builder
COPY --from=builder /app/bin/streamer /app/bin/telemetry-streamer

# Copy configuration
COPY configs/ /app/configs/

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Command to run the executable
ENTRYPOINT ["./telemetry-streamer"]

# Default command arguments
CMD ["--config", "/app/configs/config.yaml"]
