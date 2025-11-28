package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
)

// MockPublisherServiceClient is a mock implementation of mqpb.PublisherServiceClient
type MockPublisherServiceClient struct {
	mock.Mock
}

func (m *MockPublisherServiceClient) HealthCheck(ctx context.Context, in *mqpb.HealthCheckRequest, opts ...grpc.CallOption) (*mqpb.HealthCheckResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mqpb.HealthCheckResponse), args.Error(1)
}

func (m *MockPublisherServiceClient) Publish(ctx context.Context, opts ...grpc.CallOption) (mqpb.PublisherService_PublishClient, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(mqpb.PublisherService_PublishClient), args.Error(1)
}

// MockPublishClient is a mock implementation of mqpb.PublisherService_PublishClient
type MockPublishClient struct {
	mock.Mock
	msgs []*mqpb.PublishRequest
	grpc.ClientStream
}

func (m *MockPublishClient) Recv() (*mqpb.PublishResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mqpb.PublishResponse), args.Error(1)
}

func (m *MockPublishClient) Send(req *mqpb.PublishRequest) error {
	m.msgs = append(m.msgs, req)
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockPublishClient) CloseAndRecv() (*mqpb.PublishResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mqpb.PublishResponse), args.Error(1)
}

func (m *MockPublishClient) Header() (metadata.MD, error) {
	args := m.Called()
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *MockPublishClient) Trailer() metadata.MD {
	args := m.Called()
	return args.Get(0).(metadata.MD)
}

func (m *MockPublishClient) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPublishClient) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockPublishClient) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockPublishClient) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func TestNewStreamer(t *testing.T) {
	// Create a test logger
	logger := zerolog.New(os.Stdout)

	tests := []struct {
		name    string
		config  *Config
		mqAddr  string
		wantErr bool
	}{
		{
			name:    "with default config",
			config:  nil,
			mqAddr:  "localhost:50051",
			wantErr: false,
		},
		{
			name: "with custom config",
			config: &Config{
				StreamInterval: 2 * time.Second,
				BatchSize:      20,
				MetricsPath:    "./test-data/metrics.csv",
			},
			mqAddr:  "localhost:50051",
			wantErr: false,
		},
		{
			name:    "with invalid mq address",
			config:  nil,
			mqAddr:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamer, err := NewStreamer(logger, tt.config, tt.mqAddr)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, streamer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, streamer)
				
				// Cleanup
				if streamer != nil {
					_ = streamer.Stop()
				}
			}
		})
	}
}

func TestStreamer_LoadMetrics(t *testing.T) {
	// Create a temporary test CSV file
	tempDir := t.TempDir()
	testCSV := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
2023-01-01T00:00:00Z,gpu_utilization,0,nvidia0,gpu-1,Tesla-V100,test-host,,,,"0.5",""
2023-01-01T00:01:00Z,gpu_memory_used,0,nvidia0,gpu-1,Tesla-V100,test-host,,,,"1024",""`

	testCSVPath := filepath.Join(tempDir, "test_metrics.csv")
	err := os.WriteFile(testCSVPath, []byte(testCSV), 0644)
	require.NoError(t, err)

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{
			name:     "valid csv file",
			filePath: testCSVPath,
			wantErr:  false,
		},
		{
			name:     "non-existent file",
			filePath: "/nonexistent/file.csv",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamer := &Streamer{
				logger: zerolog.New(os.Stdout),
				config: &Config{
					MetricsPath: tt.filePath,
				},
			}

			err := streamer.loadMetrics()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, streamer.metrics, 2)
				assert.Equal(t, "gpu_utilization", streamer.metrics[0].MetricName)
				assert.Equal(t, "gpu-1", streamer.metrics[0].UUID)
			}
		})
	}
}

func TestStreamer_CollectAndProcess(t *testing.T) {
	// Create a test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a mock publisher client
	mockClient := new(MockPublisherServiceClient)
	mockStream := new(MockPublishClient)

	// Set up expectations
	mockClient.On("Publish", mock.Anything, mock.Anything).Return(mockStream, nil)
	mockStream.On("Send", mock.Anything).Return(nil)
	mockStream.On("CloseSend").Return(nil)

	// Create a test streamer
	streamer := &Streamer{
		logger: zerolog.New(os.Stdout),
		config: &Config{
			BatchSize: 2,
		},
		mqClient: mockClient,
		metrics: []GPUMetric{
			{
				Timestamp:  time.Now(),
				MetricName: "gpu_utilization",
				GPUIndex:   "0",
				Device:     "nvidia0",
				UUID:       "gpu-1",
				ModelName:  "Tesla-V100",
				Hostname:   "test-host",
				Value:      "0.5",
			},
			{
				Timestamp:  time.Now(),
				MetricName: "gpu_memory_used",
				GPUIndex:   "0",
				Device:     "nvidia0",
				UUID:       "gpu-1",
				ModelName:  "Tesla-V100",
				Hostname:   "test-host",
				Value:      "1024",
			},
		},
	}

	// Test collectAndProcess
	streamer.collectAndProcess(ctx)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockStream.AssertExpectations(t)
	assert.Equal(t, 2, streamer.currentIdx) // Should process 2 metrics
}

func TestStreamer_StartStop(t *testing.T) {
	// Create a test context with timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a mock publisher client and stream
	mockClient := new(MockPublisherServiceClient)
	mockStream := new(MockPublishClient)

	// Set up a channel to signal when Send is called
	sendCalled := make(chan struct{}, 1)

	// Set up mock expectations
	mockClient.On("Publish", mock.Anything, mock.Anything).Return(mockStream, nil)
	mockStream.On("Send", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sendCalled <- struct{}{} // Signal that Send was called
	})
	mockStream.On("CloseSend").Return(nil)

	// Create a test streamer with a short interval
	streamer := &Streamer{
		logger:   zerolog.New(os.Stdout),
		config:   &Config{StreamInterval: 100 * time.Millisecond, BatchSize: 1},
		mqClient: mockClient,
		done:     make(chan struct{}),
		metrics: []GPUMetric{
			{
				Timestamp:  time.Now(),
				MetricName: "gpu_utilization",
				GPUIndex:   "0",
				Device:     "nvidia0",
				UUID:       "gpu-1",
				ModelName:  "Tesla-V100",
				Hostname:   "test-host",
				Value:      "0.5",
			},
		},
	}

	// Start the streamer in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- streamer.Start(ctx)
	}()

	// Wait for Send to be called or timeout
	select {
	case <-sendCalled:
		// Send was called, continue with the test
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for Send to be called")
	}

	// Stop the streamer
	err := streamer.Stop()
	assert.NoError(t, err)

	// Wait for the streamer to stop
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Streamer did not stop within the expected time")
	}

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockStream.AssertExpectations(t)
}

func TestStreamer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*MockPublisherServiceClient, *MockPublishClient)
		expectError bool
	}{
		{
			name: "publish stream error",
			setupMock: func(mockClient *MockPublisherServiceClient, mockStream *MockPublishClient) {
				mockClient.On("Publish", mock.Anything, mock.Anything).
					Return(nil, status.Error(codes.Internal, "failed to create stream"))
			},
			expectError: true,
		},
		{
			name: "send message error",
			setupMock: func(mockClient *MockPublisherServiceClient, mockStream *MockPublishClient) {
				mockClient.On("Publish", mock.Anything, mock.Anything).Return(mockStream, nil)
				mockStream.On("Send", mock.Anything).Return(status.Error(codes.Internal, "failed to send message"))
				mockStream.On("CloseSend").Return(nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Create mock client and stream
			mockClient := new(MockPublisherServiceClient)
			mockStream := new(MockPublishClient)

			// Set up mock expectations
			tt.setupMock(mockClient, mockStream)

			// Create a test streamer
			streamer := &Streamer{
				logger: zerolog.New(os.Stdout),
				config: &Config{
					BatchSize: 1,
				},
				mqClient: mockClient,
				metrics: []GPUMetric{
					{
						Timestamp:  time.Now(),
						MetricName: "gpu_utilization",
						GPUIndex:   "0",
						Device:     "nvidia0",
						UUID:       "gpu-1",
						ModelName:  "Tesla-V100",
						Hostname:   "test-host",
						Value:      "0.5",
					},
				},
			}

			// Test collectAndProcess
			streamer.collectAndProcess(ctx)

			// Verify expectations
			mockClient.AssertExpectations(t)
			mockStream.AssertExpectations(t)
		})
	}
}
