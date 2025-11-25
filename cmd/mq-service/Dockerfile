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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/mq-service ./cmd/mq-service

# Final stage
FROM alpine:3.18

WORKDIR /app/bin

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/bin/mq-service /app/bin/
COPY configs/ /app/configs/

# Expose the ports
EXPOSE 50051 5672

# Command to run the executable
ENTRYPOINT ["./mq-service"]

# Default command arguments
CMD ["--config", "/app/configs/config.yaml"]
