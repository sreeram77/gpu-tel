#!/bin/bash

# Exit on error
set -e

# Set the version (dev for local development)
VERSION="dev"

# Build all Docker images
echo "Building Docker images..."
make docker-build VERSION=${VERSION}

# Create namespace if it doesn't exist
kubectl create namespace gpu-tel --dry-run=client -o yaml | kubectl apply -f -

# Add Bitnami Helm repository for PostgreSQL
echo "Adding Bitnami Helm repository..."
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install or upgrade the Helm chart
echo "Deploying GPU Telemetry to Kubernetes..."
helm upgrade --install gpu-tel ./deploy/charts/gpu-tel \
  --namespace gpu-tel \
  --set postgresql.auth.postgresPassword=postgres \
  --set postgresql.auth.password=mysecretpassword \
  --set postgresql.auth.database=gputel \
  --set images.apiServer.repository=gpu-tel-api \
  --set images.apiServer.tag=${VERSION} \
  --set images.mqService.repository=gpu-tel-mq \
  --set images.mqService.tag=${VERSION} \
  --set images.telemetryCollector.repository=gpu-tel-collector \
  --set images.telemetryCollector.tag=${VERSION} \
  --set images.telemetryStreamer.repository=gpu-tel-streamer \
  --set images.telemetryStreamer.tag=${VERSION}

# Wait for the pods to be ready
echo "Waiting for pods to be ready..."
kubectl wait --namespace gpu-tel \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/name=gpu-tel \
  --timeout=90s

# Show the status of the pods
echo "\nPods status:"
kubectl get pods -n gpu-tel

echo "\nDeployment complete!"
echo "To access the API server, run:"
echo "kubectl port-forward -n gpu-tel svc/gpu-tel-api-service 8080:8080"
echo "Then open http://localhost:8080 in your browser"
