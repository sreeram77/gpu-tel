# Build stage
FROM --platform=$BUILDPLATFORM gpu-tel-base:latest as builder

# Final stage
FROM alpine:3.18

WORKDIR /app

# Copy the binary from the builder
COPY --from=gpu-tel-builder /app/bin/api-server /app/

# Copy configuration
COPY configs/ /app/configs/

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["/app/api-server"]
