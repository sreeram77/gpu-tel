package telemetry

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStorage is an in-memory implementation of TelemetryStorage
// It's safe for concurrent use by multiple goroutines
type MemoryStorage struct {
	mu         sync.RWMutex
	telemetry  []GPUTelemetry
	gpuIndexes map[string]struct{}
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		telemetry:  make([]GPUTelemetry, 0),
		gpuIndexes: make(map[string]struct{}),
	}
}

// Store stores a batch of telemetry data in memory
func (m *MemoryStorage) Store(ctx context.Context, batch TelemetryBatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range batch.Telemetry {
		m.telemetry = append(m.telemetry, t)
		m.gpuIndexes[t.GPUIndex] = struct{}{}
	}

	return nil
}

// GetGPUTelemetry retrieves telemetry data for a specific GPU within a time range
func (m *MemoryStorage) GetGPUTelemetry(ctx context.Context, gpuID string, startTime, endTime time.Time) ([]GPUTelemetry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []GPUTelemetry
	for _, t := range m.telemetry {
		if t.GPUIndex == gpuID && !t.Timestamp.Before(startTime) && !t.Timestamp.After(endTime) {
			result = append(result, t)
		}
	}

	// Sort by timestamp for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result, nil
}

// ListGPUs lists all available GPUs with their telemetry data
func (m *MemoryStorage) ListGPUs(ctx context.Context) ([]GPUTelemetry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use a map to store the most recent telemetry for each GPU
	latestTelemetry := make(map[string]GPUTelemetry)

	for _, t := range m.telemetry {
		if existing, exists := latestTelemetry[t.GPUIndex]; !exists || t.Timestamp.After(existing.Timestamp) {
			latestTelemetry[t.GPUIndex] = t
		}
	}

	// Convert map to slice
	result := make([]GPUTelemetry, 0, len(latestTelemetry))
	for _, t := range latestTelemetry {
		result = append(result, t)
	}

	return result, nil
}

// Close cleans up the storage resources
func (m *MemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear all data
	m.telemetry = nil
	m.gpuIndexes = make(map[string]struct{})
	return nil
}

// Ensure MemoryStorage implements TelemetryStorage
var _ TelemetryStorage = (*MemoryStorage)(nil)
