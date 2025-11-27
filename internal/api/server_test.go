package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTelemetryStorage is a mock implementation of TelemetryStorage for testing
type MockTelemetryStorage struct {
	mock.Mock
}

func (m *MockTelemetryStorage) Store(ctx context.Context, batch telemetry.TelemetryBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockTelemetryStorage) ListGPUs(ctx context.Context) ([]telemetry.GPUTelemetry, error) {
	args := m.Called(ctx)
	return args.Get(0).([]telemetry.GPUTelemetry), args.Error(1)
}

func (m *MockTelemetryStorage) GetGPUTelemetry(ctx context.Context, gpuID string, start, end time.Time) ([]telemetry.GPUTelemetry, error) {
	args := m.Called(ctx, gpuID, start, end)
	return args.Get(0).([]telemetry.GPUTelemetry), args.Error(1)
}

func (m *MockTelemetryStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewServer(t *testing.T) {
	logger := zerolog.Nop()
	mockStorage := new(MockTelemetryStorage)

	server := NewServer(logger, mockStorage)

	assert.NotNil(t, server)
	assert.NotNil(t, server.router)
}

func TestHealthCheck(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	mockStorage := new(MockTelemetryStorage)
	server := NewServer(logger, mockStorage)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	req, _ := http.NewRequest("GET", "/health", nil)
	ctx.Request = req

	// Execute
	server.healthCheck(ctx)

	// Assert
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestListGPUs(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	mockStorage := new(MockTelemetryStorage)
	server := NewServer(logger, mockStorage)

	// Mock the expected call
	expectedGPUs := []telemetry.GPUTelemetry{
		{
			GPUIndex:  "0",
			Hostname:  "host1",
			ModelName: "NVIDIA A100",
		},
		{
			GPUIndex:  "1",
			Hostname:  "host2",
			ModelName: "NVIDIA V100",
		},
	}
	mockStorage.On("ListGPUs", mock.Anything).Return(expectedGPUs, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	req, _ := http.NewRequest("GET", "/api/v1/gpus", nil)
	ctx.Request = req

	// Execute
	server.listGPUs(ctx)

	// Assert
	assert.Equal(t, http.StatusOK, recorder.Code)
	mockStorage.AssertExpectations(t)
}

func TestGetGPUTelemetry(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	mockStorage := new(MockTelemetryStorage)
	server := NewServer(logger, mockStorage)

	// Mock data
	expectedTelemetry := []telemetry.GPUTelemetry{
		{
			Timestamp:  time.Now(),
			MetricName: "gpu_utilization",
			GPUIndex:   "0",
			Value:      42.5,
		},
	}

	// Mock the expected call
	mockStorage.On("GetGPUTelemetry", mock.Anything, "0", mock.Anything, mock.Anything).Return(expectedTelemetry, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	req, _ := http.NewRequest("GET", "/api/v1/gpus/0/telemetry?start=2023-01-01T00:00:00Z&end=2023-01-02T00:00:00Z", nil)
	ctx.Request = req
	ctx.Params = []gin.Param{{Key: "id", Value: "0"}}

	// Execute
	server.getGPUTelemetry(ctx)

	// Assert
	assert.Equal(t, http.StatusOK, recorder.Code)
	mockStorage.AssertExpectations(t)
}

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		name        string
		startTime   string
		endTime     string
		expectError bool
	}{
		{
			name:        "valid time range",
			startTime:   "2023-01-01T00:00:00Z",
			endTime:     "2023-01-02T00:00:00Z",
			expectError: false,
		},
		{
			name:        "invalid start time",
			startTime:   "invalid-time",
			endTime:     "2023-01-02T00:00:00Z",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			req, _ := http.NewRequest("GET", "/test", nil)
			q := req.URL.Query()
			if tt.startTime != "" {
				q.Add("start", tt.startTime)
			}
			if tt.endTime != "" {
				q.Add("end", tt.endTime)
			}
			req.URL.RawQuery = q.Encode()
			ctx.Request = req

			_, _, err := parseTimeRange(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
