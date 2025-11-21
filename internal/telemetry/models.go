package telemetry

import (
	"context"
	"time"
)

// GPUTelemetry represents a single telemetry data point for a GPU
type GPUTelemetry struct {
	ID             string    `json:"id"`
	GPUIndex       int       `json:"gpu_index"`
	GPUName        string    `json:"gpu_name"`
	GPUTemperature float64   `json:"gpu_temperature"`
	GPULoad        float64   `json:"gpu_load"`
	MemoryUsed     uint64    `json:"memory_used"`
	MemoryTotal    uint64    `json:"memory_total"`
	PowerDraw      float64   `json:"power_draw"`
	PowerLimit     float64   `json:"power_limit"`
	FanSpeed       float64   `json:"fan_speed"`
	Timestamp      time.Time `json:"timestamp"`
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
	// ListGPUs lists all available GPUs with telemetry data
	ListGPUs(ctx context.Context) ([]string, error)
	// Close closes the storage connection
	Close() error
}
