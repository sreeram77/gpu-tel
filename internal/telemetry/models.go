package telemetry

import (
	"context"
	"time"
)

type GPU struct {
	UUID     string `json:"uuid"`
	GPUIndex string `json:"gpu_id"`
	Device   string `json:"device"`
	Hostname string `json:"hostname"`
}

// GPUTelemetry represents a single telemetry data point for a GPU
type GPUTelemetry struct {
	// Fields from CSV
	Timestamp  time.Time `json:"timestamp"`
	MetricName string    `json:"metric_name"`
	GPUIndex   string    `json:"gpu_id"`
	Device     string    `json:"device"`
	UUID       string    `json:"uuid"`
	ModelName  string    `json:"model_name"`
	Hostname   string    `json:"hostname"`
	Container  string    `json:"container,omitempty"`
	Pod        string    `json:"pod,omitempty"`
	Namespace  string    `json:"namespace,omitempty"`
	Value      float64   `json:"value"`
	LabelsRaw  string    `json:"labels_raw,omitempty"`
}

// TelemetryBatch represents a batch of GPU telemetry data points
type TelemetryBatch struct {
	BatchID   string         `json:"batch_id"`
	Telemetry []GPUTelemetry `json:"telemetry"`
	Timestamp time.Time      `json:"timestamp"`
}

// TelemetryStreamer defines the interface for telemetry streaming
type TelemetryStreamer interface {
	// Start begins streaming telemetry data
	Start(ctx context.Context) error
	// Stop gracefully stops the streamer
	Stop() error
}

// TelemetryCollector defines the interface for telemetry collection
type TelemetryCollector interface {
	// Start begins collecting telemetry data
	Start(ctx context.Context) error
	// Stop gracefully stops the collector
	Stop() error
}

// TelemetryStorage defines the interface for telemetry data storage
type TelemetryStorage interface {
	// Store stores a batch of telemetry data
	Store(ctx context.Context, batch TelemetryBatch) error
	// GetGPUTelemetry retrieves telemetry data for a specific GPU
	GetGPUTelemetry(ctx context.Context, gpuID string, startTime, endTime time.Time) ([]GPUTelemetry, error)
	// ListGPUs lists all available GPUs with their telemetry data
	ListGPUs(ctx context.Context) ([]GPU, error)
	// Close closes the storage connection
	Close() error
}
