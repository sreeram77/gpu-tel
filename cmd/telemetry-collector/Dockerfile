# Build stage
FROM --platform=$BUILDPLATFORM gpu-tel-base:latest as builder

# Final stage
FROM nvidia/cuda:12.2.0-base-ubuntu22.04

WORKDIR /app

# Install NVIDIA Container Toolkit and other dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    nvidia-utils-525 \
    nvidia-settings \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from the builder
COPY --from=gpu-tel-builder /app/bin/collector /app/

# Copy configuration
COPY configs/ /app/configs/

# Command to run the executable
CMD ["/app/collector"]
