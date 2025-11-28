# Build stage
FROM golang:1.25.3-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev git

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/sink ./cmd/sink

# Final stage
FROM alpine:3.18

WORKDIR /app/bin

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create necessary directories
RUN mkdir -p /app/configs

# Copy binary and configs from builder
COPY --from=builder /app/bin/sink .
COPY --from=builder /app/configs /app/configs
COPY --from=builder /app/swagger/ /app/swagger/
COPY --from=builder /app/api/openapi.yaml /app/api/

# Set environment variables
ENV CONFIG_FILE=/app/configs/config.yaml

# Make sure the swagger.json is accessible
RUN chmod -R 755 /app/swagger && \
    chmod 644 /app/api/openapi.yaml

# Expose the port the app runs on
EXPOSE 8080

# Command to run the application
ENTRYPOINT ["./sink"]

# Default command arguments
CMD ["--config", "/app/configs/config.yaml"]
