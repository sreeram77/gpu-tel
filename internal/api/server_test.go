package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// stripAnsiCodes removes ANSI color codes from a string
func stripAnsiCodes(str string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(str, "")
}

// MockTelemetryStorage is a mock implementation of TelemetryStorage for testing
type MockTelemetryStorage struct {
	mock.Mock
}

func (m *MockTelemetryStorage) Store(ctx context.Context, batch telemetry.TelemetryBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockTelemetryStorage) ListGPUs(ctx context.Context) ([]telemetry.GPU, error) {
	args := m.Called(ctx)
	return args.Get(0).([]telemetry.GPU), args.Error(1)
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
	expectedGPUs := []telemetry.GPU{
		{
			GPUIndex: "0",
			Hostname: "host1",
			Device:   "NVIDIA A100",
		},
		{
			GPUIndex: "1",
			Hostname: "host2",
			Device:   "NVIDIA V100",
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
	t.Run("with valid GPU ID", func(t *testing.T) {
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
	})

	t.Run("with empty GPU ID", func(t *testing.T) {
		// Setup
		logger := zerolog.Nop()
		server := NewServer(logger, nil) // No need for storage mock as it shouldn't be called

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)

		req, _ := http.NewRequest("GET", "/api/v1/gpus//telemetry", nil)
		ctx.Request = req
		ctx.Params = []gin.Param{{Key: "id", Value: ""}}

		// Execute
		server.getGPUTelemetry(ctx)

		// Assert
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var resp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Equal(t, "GPU ID is required", resp.Message)
	})
}

func TestRequestLogger(t *testing.T) {
	// Create a test logger that captures output
	logOutput := ""
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out: &testWriter{output: &logOutput},
	}).Level(zerolog.DebugLevel)

	// Create a test router with the requestLogger middleware
	router := gin.New()
	router.Use(requestLogger(logger))

	// Test case 1: Successful request
	t.Run("successful request", func(t *testing.T) {
		logOutput = "" // Reset log output
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req, _ := http.NewRequest("GET", "/test?foo=bar", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.RemoteAddr = "127.0.0.1:12345"

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		// Strip ANSI color codes from log output
		cleanLogs := stripAnsiCodes(logOutput)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, cleanLogs, "Request processed")
		assert.Contains(t, cleanLogs, "method=GET")
		assert.Contains(t, cleanLogs, "path=/test")
		assert.Contains(t, cleanLogs, "status=200")
		assert.Contains(t, cleanLogs, "query=foo=bar")
		assert.Contains(t, cleanLogs, "user-agent=test-agent")
	})

	// Test case 2: Error response
	t.Run("error response", func(t *testing.T) {
		logOutput = "" // Reset log output
		errorMsg := "test error"
		router.GET("/error", func(c *gin.Context) {
			c.Error(gin.Error{Err: assert.AnError, Type: gin.ErrorTypePrivate})
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
		})

		req, _ := http.NewRequest("GET", "/error", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		// Strip ANSI color codes from log output
		cleanLogs := stripAnsiCodes(logOutput)

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
		assert.Contains(t, cleanLogs, "Request processed")
		assert.Contains(t, cleanLogs, "status=500")
		assert.Contains(t, cleanLogs, "ERR")
		assert.Contains(t, cleanLogs, "assert.AnError")
	})
}

// testWriter is a helper to capture log output
type testWriter struct {
	output *string
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	*w.output += string(p)
	return len(p), nil
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
