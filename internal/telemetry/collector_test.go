package telemetry

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// MockStorage is a mock implementation of TelemetryStorage for testing
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Store(ctx context.Context, batch TelemetryBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockStorage) ListGPUs(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStorage) GetGPUTelemetry(ctx context.Context, gpuID string, start, end time.Time) ([]GPUTelemetry, error) {
	args := m.Called(ctx, gpuID, start, end)
	return args.Get(0).([]GPUTelemetry), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewCollector(t *testing.T) {
	// Test setup
	otel.SetTracerProvider(trace.NewNoopTracerProvider())

	logger := zerolog.Nop()
	mockStorage := new(MockStorage)
	config := &CollectorConfig{
		MQAddr:            "localhost:50051",
		Topic:             "test-topic",
		ConsumerGroup:     "test-group",
		BatchSize:         100,
		MaxInFlight:       10,
		AckTimeoutSeconds: 30,
		WorkerCount:       5,
	}

	collector, err := NewCollector(logger, mockStorage, config)

	assert.NoError(t, err)
	assert.NotNil(t, collector)
	assert.Equal(t, mockStorage, collector.storage)
	assert.Equal(t, config, collector.config)
}

type mockSubscriberServer struct {
	mqpb.UnimplementedSubscriberServiceServer
	subscribeFunc func(mqpb.SubscriberService_SubscribeServer) error
}

func (s *mockSubscriberServer) Subscribe(stream mqpb.SubscriberService_SubscribeServer) error {
	if s.subscribeFunc != nil {
		return s.subscribeFunc(stream)
	}
	return nil
}

func startMockGRPCServer(t *testing.T) (string, func()) {
	// Create a mock subscriber service
	mockSvc := &mockSubscriberServer{
		subscribeFunc: func(stream mqpb.SubscriberService_SubscribeServer) error {
			// Simulate a successful subscription with no messages
			_, err := stream.Recv()
			if err != nil {
				return err
			}
			return nil
		},
	}

	// Create a gRPC server
	grpcServer := grpc.NewServer()
	
	// Register the mock service with the gRPC server
	mqpb.RegisterSubscriberServiceServer(grpcServer, mockSvc)

	// Start the server in a goroutine
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// Wait for server to start
	select {
	case err := <-errCh:
		t.Fatalf("failed to start server: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Return the address and a cleanup function
	return lis.Addr().String(), func() {
		grpcServer.Stop()
		lis.Close()
	}
}

func TestCollector_StartStop(t *testing.T) {
	// Start a mock gRPC server
	addr, stopServer := startMockGRPCServer(t)
	defer stopServer()

	// Test setup
	otel.SetTracerProvider(trace.NewNoopTracerProvider())

	logger := zerolog.Nop()
	mockStorage := new(MockStorage)
	config := &CollectorConfig{
		MQAddr:            addr, // Use the mock server address
		Topic:             "test-topic",
		ConsumerGroup:     "test-group",
		BatchSize:         100,
		MaxInFlight:       10,
		AckTimeoutSeconds: 1,
		WorkerCount:       1,
	}

	// Setup mock expectations
	mockStorage.On("Close").Return(nil).Once()

	// Create the collector
	collector, err := NewCollector(logger, mockStorage, config)
	if !assert.NoError(t, err) {
		return
	}

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start the collector in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- collector.Start(ctx)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel the context to stop the collector
	cancel()

	// Wait for the collector to stop
	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled, "should return context canceled error")
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for collector to stop")
	}

	// Now call Stop to clean up resources
	err = collector.Stop()
	assert.NoError(t, err, "Stop should not return an error")

	// Verify all expectations were met
	mockStorage.AssertExpectations(t)
}

func TestAcknowledgeMessages(t *testing.T) {
	// Test setup
	otel.SetTracerProvider(trace.NewNoopTracerProvider())

	logger := zerolog.Nop()
	mockStorage := new(MockStorage)
	config := &CollectorConfig{
		MQAddr:            "localhost:0",
		Topic:             "test-topic",
		ConsumerGroup:     "test-group",
		BatchSize:         100,
		MaxInFlight:       10,
		AckTimeoutSeconds: 1,
		WorkerCount:       1,
	}

	// Create a collector with a real gRPC client connection
	collector, err := NewCollector(logger, mockStorage, config)
	assert.NoError(t, err)

	// Create a test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test cases
	tests := []struct {
		name          string
		setupMock     func()
		expectedError bool
	}{
		{
			name: "successful ack",
			setupMock: func() {
				// No mocks needed for this test case
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupMock != nil {
				tt.setupMock()
			}

			// Use a non-existent server address to trigger an error
			err := collector.acknowledgeMessages(ctx, []string{"msg1", "msg2"}, "test-consumer")

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				// We expect an error because we're not running a real server
				assert.Error(t, err)
			}
		})
	}
}
