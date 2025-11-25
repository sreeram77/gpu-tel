# Build stage
FROM --platform=$BUILDPLATFORM gpu-tel-base:latest as builder

# Final stage
FROM gpu-tel-base:${VERSION}

WORKDIR /app/bin

# Install NVIDIA Container Toolkit and other dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    nvidia-utils-525 \
    nvidia-settings \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from the builder
COPY --from=builder /app/bin/collector /app/bin/telemetry-collector

# Copy configuration
COPY configs/ /app/configs/

# Command to run the executable
ENTRYPOINT ["./telemetry-collector"]

# Default command arguments
CMD ["--config", "/app/configs/config.yaml"]
