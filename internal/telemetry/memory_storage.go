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
	mu        sync.RWMutex
	gpus      map[string]GPU
	telemetry map[string][]GPUTelemetry
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		gpus:      make(map[string]GPU),
		telemetry: make(map[string][]GPUTelemetry),
	}
}

// Store stores a batch of telemetry data in memory
func (m *MemoryStorage) Store(ctx context.Context, batch TelemetryBatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range batch.Telemetry {
		// Store UUID in gpus map
		m.gpus[t.UUID] = GPU{
			UUID:     t.UUID,
			GPUIndex: t.GPUIndex,
			Device:   t.Device,
			Hostname: t.Hostname,
		}
		// Append telemetry data for this UUID
		m.telemetry[t.UUID] = append(m.telemetry[t.UUID], t)
	}

	return nil
}

// GetGPUTelemetry retrieves telemetry data for a specific GPU within a time range
func (m *MemoryStorage) GetGPUTelemetry(ctx context.Context, gpuID string, startTime, endTime time.Time) ([]GPUTelemetry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []GPUTelemetry
	if telemetry, exists := m.telemetry[gpuID]; exists {
		for _, t := range telemetry {
			if !t.Timestamp.Before(startTime) && !t.Timestamp.After(endTime) {
				result = append(result, t)
			}
		}
	}

	// Sort by timestamp for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result, nil
}

// ListGPUs returns a list of GPU UUIDs that have telemetry data
func (m *MemoryStorage) ListGPUs(ctx context.Context) ([]GPU, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []GPU

	for _, gpu := range m.gpus {
		result = append(result, gpu)
	}

	return result, nil
}

// Close cleans up the storage resources
func (m *MemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear all data
	m.gpus = make(map[string]GPU)
	m.telemetry = make(map[string][]GPUTelemetry)
	return nil
}

// Ensure MemoryStorage implements TelemetryStorage
var _ TelemetryStorage = (*MemoryStorage)(nil)
