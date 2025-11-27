package storage_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"sync"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sreeram77/gpu-tel/internal/storage"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

const (
	testDBHost     = "localhost"
	testDBPort     = 5432
	testDBUser     = "gputel"
	testDBPassword = "mysecretpassword"
	testDBName     = "postgres" // Use the default postgres database
)

func TestPostgresStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	connStr, testDB := setupTestDB(t)
	defer testDB.Close()

	// Parse connection string to get config
	cfg, err := parseConnString(connStr)
	require.NoError(t, err, "Failed to parse connection string")

	// Create logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Initialize the storage to create tables
	store, err := storage.NewPostgresStorage(logger, cfg)
	require.NoError(t, err, "Failed to create PostgresStorage")
	defer store.Close()

	// Store will create the table if it doesn't exist
	err = store.Store(context.Background(), telemetry.TelemetryBatch{})
	require.NoError(t, err, "Failed to initialize test tables")

	t.Run("NewPostgresStorage", func(t *testing.T) {
		store, err := storage.NewPostgresStorage(logger, cfg)
		require.NoError(t, err, "Failed to create PostgresStorage")
		require.NotNil(t, store)
		defer store.Close()

		// Verify tables were created
		// We can check this by trying to insert data
		testBatch := telemetry.TelemetryBatch{
			Telemetry: []telemetry.GPUTelemetry{
				createTestTelemetry("gpu-1", "gpu_utilization", 75.5),
			},
		}

		err = store.Store(context.Background(), testBatch)
		assert.NoError(t, err, "Failed to store telemetry data")
	})

	t.Run("StoreAndRetrieveTelemetry", func(t *testing.T) {
		store, err := storage.NewPostgresStorage(logger, cfg)
		require.NoError(t, err)
		defer store.Close()

		// Test data
		gpuID := "test-gpu-1"
		gpuUUID := "GPU-12345678-90ab-cdef-ghij-klmnopqrstuv"
		now := time.Now()
		
		// Create test telemetry with consistent UUID and GPU ID
		testTelemetry := []telemetry.GPUTelemetry{
			createTestTelemetryWithTime(gpuID, "gpu_utilization", 75.5, now.Add(-2*time.Minute)),
			createTestTelemetryWithTime(gpuID, "gpu_utilization", 80.2, now.Add(-1*time.Minute)),
			createTestTelemetryWithTime(gpuID, "gpu_utilization", 82.7, now),
			createTestTelemetryWithTime(gpuID, "gpu_memory_used", 1024.0, now),
		}

		// Set consistent UUID and GPU ID for all test data
		for i := range testTelemetry {
			testTelemetry[i].UUID = gpuUUID
			testTelemetry[i].GPUIndex = gpuID // Ensure consistent GPU ID
		}

		testBatch := telemetry.TelemetryBatch{
			Telemetry: testTelemetry,
		}

		// Clear any existing data first
		_, err = testDB.Exec("TRUNCATE TABLE gpu_telemetry CASCADE")
		require.NoError(t, err, "Failed to clear test data")

		// Store test data and ensure it's committed
		ctx := context.Background()
		err = store.Store(ctx, testBatch)
		assert.NoError(t, err)

		// Small delay to ensure data is committed
		time.Sleep(100 * time.Millisecond)

		// Test GetGPUTelemetry
		t.Run("GetGPUTelemetry", func(t *testing.T) {
			// Test with time range
			start := now.Add(-5 * time.Minute)
			end := now.Add(1 * time.Minute)

			// Use the UUID for querying, not the GPU ID
			metrics, err := store.GetGPUTelemetry(context.Background(), gpuUUID, start, end)
			require.NoError(t, err, "Failed to get GPU telemetry")
		
			// Filter for gpu_utilization metrics only
			var utilizationMetrics []telemetry.GPUTelemetry
			for _, m := range metrics {
				if m.MetricName == "gpu_utilization" {
					utilizationMetrics = append(utilizationMetrics, m)
				}
			}
			assert.Len(t, utilizationMetrics, 3, "Should return 3 metrics for gpu_utilization")

			// Verify data integrity
			for _, m := range metrics {
				assert.Equal(t, gpuID, m.GPUIndex)
				assert.NotEmpty(t, m.Device)
				assert.NotEmpty(t, m.UUID)
				assert.NotEmpty(t, m.ModelName)
				assert.NotEmpty(t, m.Hostname)
			}
		})

		// Test ListGPUs
		t.Run("ListGPUs", func(t *testing.T) {
			gpus, err := store.ListGPUs(context.Background())
			assert.NoError(t, err)
			
			// Verify we got at least one GPU
			assert.Greater(t, len(gpus), 0, "Should return at least one GPU")
			
			// Find the test GPU in the results
			found := false
			expectedUUID := "GPU-12345678-90ab-cdef-ghij-klmnopqrstuv"
			expectedModel := "NVIDIA A100-SXM4-40GB"
			expectedHostname := "test-host"
			
			for _, gpu := range gpus {
				if gpu.UUID == expectedUUID && gpu.ModelName == expectedModel && gpu.Hostname == expectedHostname {
					found = true
					break
				}
			}
			
			assert.True(t, found, "Should contain the test GPU with expected details")
		})

		// Test with non-existent GPU
		t.Run("NonExistentGPU", func(t *testing.T) {
			metrics, err := store.GetGPUTelemetry(context.Background(), "non-existent-uuid", now.Add(-1*time.Hour), now)
			assert.NoError(t, err)
			assert.Empty(t, metrics, "Should return empty slice for non-existent GPU")
		})
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		store, err := storage.NewPostgresStorage(logger, cfg)
		require.NoError(t, err)
		defer store.Close()

		// Number of concurrent operations
		const numOps = 10
		var wg sync.WaitGroup
		wg.Add(numOps)

		for i := 0; i < numOps; i++ {
			go func(i int) {
				defer wg.Done()
				gpuID := fmt.Sprintf("concurrent-gpu-%d", i%3) // Only 3 unique GPUs

				// Store data
				testBatch := telemetry.TelemetryBatch{
					Telemetry: []telemetry.GPUTelemetry{
						createTestTelemetry(gpuID, "gpu_utilization", float64(i%100)),
					},
				}

				err := store.Store(context.Background(), testBatch)
				assert.NoError(t, err, "Concurrent store operation failed")

				// Read back
				_, err = store.GetGPUTelemetry(context.Background(), gpuID, time.Now().Add(-1*time.Hour), time.Now())
				assert.NoError(t, err, "Concurrent read operation failed")
			}(i)
		}

		// Wait for all operations to complete
		wg.Wait()

		// Verify all GPUs are listed
		gpus, err := store.ListGPUs(context.Background())
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(gpus), 3, "Should have at least 3 unique GPUs from concurrent operations")
	})
}

// Helper function to create a test telemetry entry
func createTestTelemetry(gpuID, metricName string, value float64) telemetry.GPUTelemetry {
	return createTestTelemetryWithTime(gpuID, metricName, value, time.Now())
}

// Helper function to create a test telemetry entry with specific timestamp
func createTestTelemetryWithTime(gpuID, metricName string, value float64, timestamp time.Time) telemetry.GPUTelemetry {
	return telemetry.GPUTelemetry{
		Timestamp:  timestamp,
		MetricName: metricName,
		GPUIndex:   gpuID,
		Device:     "/dev/nvidia0",
		UUID:       "GPU-12345678-90ab-cdef-ghij-klmnopqrstuv",
		ModelName:  "NVIDIA A100-SXM4-40GB",
		Hostname:   "test-host",
		Value:      value,
	}
}

// setupTestDB sets up the test database and returns a connection string and db connection
func setupTestDB(t *testing.T) (string, *sql.DB) {
	t.Helper()

	// Connect to the default postgres database
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		testDBHost, testDBPort, testDBUser, testDBPassword, testDBName)

	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err, "Failed to connect to PostgreSQL")

	// Clean up after tests
	t.Cleanup(func() {
		// Truncate tables instead of dropping to avoid permission issues
		tables := []string{"gpu_telemetry", "gpu_info"}
		for _, table := range tables {
			_, _ = db.Exec(fmt.Sprintf(`TRUNCATE TABLE %s CASCADE`, table))
		}
		db.Close()
	})

	return connStr, db
}

// parseConnString parses a connection string into a PostgresConfig
func parseConnString(connStr string) (*storage.PostgresConfig, error) {
	// This is a simplified parser for test purposes
	// In a real application, use a proper URL parser
	var cfg storage.PostgresConfig
	pairs := strings.Split(connStr, " ")

	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {
		case "host":
			cfg.Host = value
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %w", err)
			}
			cfg.Port = port
		case "user":
			cfg.User = value
		case "password":
			cfg.Password = value
		case "dbname":
			cfg.DBName = value
		case "sslmode":
			cfg.SSLMode = value
		}
	}

	if cfg.Host == "" || cfg.Port == 0 || cfg.User == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("incomplete connection string")
	}

	return &cfg, nil
}
