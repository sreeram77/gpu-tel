# Build stage
FROM --platform=$BUILDPLATFORM gpu-tel-base:latest as builder

# Final stage
FROM alpine:3.18

WORKDIR /app

# Copy the binary from the builder
COPY --from=gpu-tel-builder /app/bin/streamer /app/

# Copy configuration
COPY configs/ /app/configs/

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Command to run the executable
CMD ["/app/streamer"]
