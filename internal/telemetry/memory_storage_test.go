package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStorage(t *testing.T) {
	t.Run("should initialize empty storage", func(t *testing.T) {
		storage := NewMemoryStorage()
		defer storage.Close()

		gpus, err := storage.ListGPUs(context.Background())
		require.NoError(t, err)
		assert.Empty(t, gpus)
	})
}

func TestMemoryStorage_Store(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	t.Run("should store telemetry data", func(t *testing.T) {
		batch := TelemetryBatch{
			BatchID: "test-batch-1",
			Telemetry: []GPUTelemetry{
				{
					UUID:       "gpu-1",
					GPUIndex:   "0",
					Device:     "nvidia0",
					Hostname:   "test-host",
					Timestamp:  time.Now(),
					MetricName: "gpu_utilization",
					Value:      42.5,
				},
			},
		}

		err := storage.Store(context.Background(), batch)
		require.NoError(t, err)

		// Verify GPU was added to the list
		gpus, err := storage.ListGPUs(context.Background())
		require.NoError(t, err)
		require.Len(t, gpus, 1)
		assert.Equal(t, "gpu-1", gpus[0].UUID)
	})

	t.Run("should handle multiple GPUs", func(t *testing.T) {
		batch := TelemetryBatch{
			BatchID: "test-batch-2",
			Telemetry: []GPUTelemetry{
				{
					UUID:       "gpu-2",
					GPUIndex:   "1",
					Device:     "nvidia1",
					Hostname:   "test-host",
					Timestamp:  time.Now(),
					MetricName: "gpu_utilization",
					Value:      30.0,
				},
			},
		}

		err := storage.Store(context.Background(), batch)
		require.NoError(t, err)

		gpus, err := storage.ListGPUs(context.Background())
		require.NoError(t, err)
		assert.Len(t, gpus, 2)
	})
}

func TestMemoryStorage_GetGPUTelemetry(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	now := time.Now()
	testData := []GPUTelemetry{
		{
			UUID:       "gpu-1",
			GPUIndex:   "0",
			Device:     "nvidia0",
			Hostname:   "test-host",
			Timestamp:  now.Add(-2 * time.Hour),
			MetricName: "gpu_utilization",
			Value:      10.0,
		},
		{
			UUID:       "gpu-1",
			GPUIndex:   "0",
			Device:     "nvidia0",
			Hostname:   "test-host",
			Timestamp:  now.Add(-1 * time.Hour),
			MetricName: "gpu_utilization",
			Value:      20.0,
		},
		{
			UUID:       "gpu-1",
			GPUIndex:   "0",
			Device:     "nvidia0",
			Hostname:   "test-host",
			Timestamp:  now.Add(1 * time.Hour),
			MetricName: "gpu_utilization",
			Value:      30.0,
		},
	}

	// Store test data
	err := storage.Store(context.Background(), TelemetryBatch{
		BatchID:   "test-batch-3",
		Telemetry: testData,
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		gpuID     string
		startTime time.Time
		endTime   time.Time
		expected  int
	}{
		{
			name:      "should return all telemetry within time range",
			gpuID:     "gpu-1",
			startTime: now.Add(-3 * time.Hour),
			endTime:   now.Add(2 * time.Hour),
			expected:  3,
		},
		{
			name:      "should filter by time range",
			gpuID:     "gpu-1",
			startTime: now.Add(-90 * time.Minute),
			endTime:   now.Add(30 * time.Minute),
			expected:  1,
		},
		{
			name:      "should return empty for non-existent GPU",
			gpuID:     "non-existent",
			startTime: now.Add(-24 * time.Hour),
			endTime:   now.Add(24 * time.Hour),
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := storage.GetGPUTelemetry(context.Background(), tt.gpuID, tt.startTime, tt.endTime)
			require.NoError(t, err)
			assert.Len(t, result, tt.expected)

			// Verify results are sorted by timestamp
			for i := 1; i < len(result); i++ {
				assert.True(t, result[i-1].Timestamp.Before(result[i].Timestamp) || 
					result[i-1].Timestamp.Equal(result[i].Timestamp))
			}
		})
	}
}

func TestMemoryStorage_ListGPUs(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	// Add test data
	err := storage.Store(context.Background(), TelemetryBatch{
		BatchID: "test-batch-4",
		Telemetry: []GPUTelemetry{
			{
				UUID:       "gpu-1",
				GPUIndex:   "0",
				Device:     "nvidia0",
				Hostname:   "test-host-1",
				Timestamp:  time.Now(),
				MetricName: "gpu_utilization",
				Value:      10.0,
			},
			{
				UUID:       "gpu-2",
				GPUIndex:   "1",
				Device:     "nvidia1",
				Hostname:   "test-host-2",
				Timestamp:  time.Now(),
				MetricName: "gpu_utilization",
				Value:      20.0,
			},
		},
	})
	require.NoError(t, err)

	gpus, err := storage.ListGPUs(context.Background())
	require.NoError(t, err)
	require.Len(t, gpus, 2)

	// Verify GPU details
	gpuMap := make(map[string]GPU)
	for _, gpu := range gpus {
		gpuMap[gpu.UUID] = gpu
	}

	assert.Contains(t, gpuMap, "gpu-1")
	assert.Equal(t, "0", gpuMap["gpu-1"].GPUIndex)
	assert.Equal(t, "nvidia0", gpuMap["gpu-1"].Device)
	assert.Equal(t, "test-host-1", gpuMap["gpu-1"].Hostname)

	assert.Contains(t, gpuMap, "gpu-2")
	assert.Equal(t, "1", gpuMap["gpu-2"].GPUIndex)
	assert.Equal(t, "nvidia1", gpuMap["gpu-2"].Device)
	assert.Equal(t, "test-host-2", gpuMap["gpu-2"].Hostname)
}

func TestMemoryStorage_Close(t *testing.T) {
	storage := NewMemoryStorage()

	// Add some test data
	err := storage.Store(context.Background(), TelemetryBatch{
		BatchID: "test-batch-5",
		Telemetry: []GPUTelemetry{
			{
				UUID:       "gpu-1",
				GPUIndex:   "0",
				Device:     "nvidia0",
				Hostname:   "test-host",
				Timestamp:  time.Now(),
				MetricName: "gpu_utilization",
				Value:      10.0,
			},
		},
	})
	require.NoError(t, err)

	// Close should clear all data
	err = storage.Close()
	require.NoError(t, err)

	// Verify data is cleared
	gpus, err := storage.ListGPUs(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gpus)

	telemetry, err := storage.GetGPUTelemetry(
		context.Background(),
		"gpu-1",
		time.Now().Add(-24*time.Hour),
		time.Now().Add(24*time.Hour),
	)
	require.NoError(t, err)
	assert.Empty(t, telemetry)
}

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	// Number of concurrent operations
	numOps := 100
	done := make(chan bool)
	errs := make(chan error, numOps*2)

	// Start multiple goroutines to read and write concurrently
	for i := 0; i < numOps; i++ {
		// Writer goroutine
		go func(id int) {
			batch := TelemetryBatch{
				BatchID: "concurrent-batch",
				Telemetry: []GPUTelemetry{
					{
						UUID:       "gpu-1",
						GPUIndex:   "0",
						Device:     "nvidia0",
						Hostname:   "test-host",
						Timestamp:  time.Now(),
						MetricName: "gpu_utilization",
						Value:      float64(id),
					},
				},
			}
			errs <- storage.Store(context.Background(), batch)
		}(i)

		// Reader goroutine
		go func() {
			_, err := storage.GetGPUTelemetry(
				context.Background(),
				"gpu-1",
				time.Now().Add(-time.Hour),
				time.Now().Add(time.Hour),
			)
			errs <- err
		}()
	}

	// Wait for all operations to complete
	go func() {
		for i := 0; i < numOps*2; i++ {
			if err := <-errs; err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// All operations completed
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out, possible deadlock")
	}

	// Verify data integrity
	gpus, err := storage.ListGPUs(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, gpus)
}
