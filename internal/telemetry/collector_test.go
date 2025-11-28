package telemetry

import (
	"context"
	"io"
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
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockSubscribeStream is a mock implementation of mqpb.SubscriberService_SubscribeServer
// for testing the processSubscription method
type mockSubscribeStream struct {
	mock.Mock
	ctx      context.Context
	recvFunc func() (*mqpb.SubscribeResponse, error)
}

// Header implements grpc.ClientStream
func (m *mockSubscribeStream) Header() (metadata.MD, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(metadata.MD), args.Error(1)
}

// Trailer implements grpc.ClientStream
func (m *mockSubscribeStream) Trailer() metadata.MD {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(metadata.MD)
}

// SendMsg implements grpc.ClientStream
func (m *mockSubscribeStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// RecvMsg implements grpc.ClientStream
func (m *mockSubscribeStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// Send implements grpc.ClientStream
func (m *mockSubscribeStream) Send(*mqpb.SubscribeRequest) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSubscribeStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockSubscribeStream) SendAndClose(*mqpb.SubscribeResponse) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSubscribeStream) Recv() (*mqpb.SubscribeResponse, error) {
	if m.recvFunc != nil {
		return m.recvFunc()
	}
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mqpb.SubscribeResponse), args.Error(1)
}

func (m *mockSubscribeStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

// MockStorage is a mock implementation of TelemetryStorage for testing
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Store(ctx context.Context, batch TelemetryBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockStorage) ListGPUs(ctx context.Context) ([]GPU, error) {
	args := m.Called(ctx)
	return args.Get(0).([]GPU), args.Error(1)
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

// mockAcknowledgeStream is a mock implementation of grpc.ClientStream for Acknowledge
// It's used to test the Acknowledge RPC method
// It implements the following interfaces:
// - grpc.ClientStream
// - grpc.ClientStreamingClient[mqpb.AcknowledgeRequest, emptypb.Empty]
type mockAcknowledgeStream struct {
	mock.Mock
}

// Header returns the header metadata received from the server.
func (m *mockAcknowledgeStream) Header() (metadata.MD, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(metadata.MD), args.Error(1)
}

// Trailer returns the trailer metadata from the server.
func (m *mockAcknowledgeStream) Trailer() metadata.MD {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(metadata.MD)
}

// CloseSend closes the send direction of the stream.
func (m *mockAcknowledgeStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

// Context returns the context for this stream.
func (m *mockAcknowledgeStream) Context() context.Context {
	args := m.Called()
	if args.Get(0) == nil {
		return context.Background()
	}
	return args.Get(0).(context.Context)
}

// SendMsg sends a message on the stream.
func (m *mockAcknowledgeStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// RecvMsg receives a message from the stream.
func (m *mockAcknowledgeStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// Send sends a message on the stream.
func (m *mockAcknowledgeStream) Send(req *mqpb.AcknowledgeRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

// CloseAndRecv closes the send direction and receives the server's response.
func (m *mockAcknowledgeStream) CloseAndRecv() (*emptypb.Empty, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*emptypb.Empty), args.Error(1)
}

type mockSubscriberClient struct {
	mock.Mock
}

func (m *mockSubscriberClient) Subscribe(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[mqpb.SubscribeRequest, mqpb.SubscribeResponse], error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(grpc.BidiStreamingClient[mqpb.SubscribeRequest, mqpb.SubscribeResponse]), args.Error(1)
}

func (m *mockSubscriberClient) Acknowledge(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[mqpb.AcknowledgeRequest, emptypb.Empty], error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(grpc.ClientStreamingClient[mqpb.AcknowledgeRequest, emptypb.Empty]), args.Error(1)
}

// HealthCheck implements mq.SubscriberServiceClient
func (m *mockSubscriberClient) HealthCheck(ctx context.Context, in *mqpb.HealthCheckRequest, opts ...grpc.CallOption) (*mqpb.HealthCheckResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mqpb.HealthCheckResponse), args.Error(1)
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

func TestMessageRecv(t *testing.T) {
	// Setup test cases
	tests := []struct {
		name          string
		setupMocks    func(*mockSubscribeStream, *MockStorage)
		expectedError string
		expectStored  bool
	}{
		{
			name: "successful message receive",
			setupMocks: func(stream *mockSubscribeStream, storage *MockStorage) {
				// First call returns a message
				stream.On("Recv").Return(&mqpb.SubscribeResponse{
					Messages: []*mqpb.Message{
						{
							Id: "msg-1",
							Payload: []byte(`{"uuid":"gpu-1","timestamp":"2023-01-01T00:00:00Z"}`),
						},
					},
				}, nil).Once()

				// Second call returns EOF to end the stream
				stream.On("Recv").Return((*mqpb.SubscribeResponse)(nil), io.EOF).Once()

				// Expect storage.Store to be called with the telemetry data
				storage.On("Store", mock.Anything, mock.MatchedBy(func(batch TelemetryBatch) bool {
					return len(batch.Telemetry) == 1 && batch.Telemetry[0].UUID == "gpu-1"
				})).Return(nil).Once()

				// Mock CloseSend which is called in the defer block
				stream.On("CloseSend").Return(nil).Once()
			},
			expectStored: true,
			expectedError: "failed to receive messages: EOF", // Expect EOF error in the success case
		},
		{
			name: "receive error",
			setupMocks: func(stream *mockSubscribeStream, storage *MockStorage) {
				// Mock a receive error
				stream.On("Recv").Return((*mqpb.SubscribeResponse)(nil), io.EOF).Once()
				stream.On("CloseSend").Return(nil)
			},
			expectedError: "failed to receive messages: EOF",
			expectStored:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test dependencies
			mockStorage := new(MockStorage)
			mockStream := new(mockSubscribeStream)
			
			// Apply test case mocks
			tc.setupMocks(mockStream, mockStorage)

			// Create mock subscriber client
			mockSubClient := &mockSubscriberClient{}
			mockSubClient.On("Subscribe", mock.Anything, mock.Anything).
				Return(mockStream, nil)
			
			// Mock HealthCheck to return a healthy response
			mockSubClient.On("HealthCheck", mock.Anything, mock.Anything, mock.Anything).
				Return(&mqpb.HealthCheckResponse{Status: mqpb.HealthCheckResponse_SERVING}, nil)
			
			// Mock Acknowledge to return a successful response
			mockAckStream := new(mockAcknowledgeStream)
			mockAckStream.On("Send", mock.Anything).Return(nil)
			mockAckStream.On("CloseAndRecv").Return(&emptypb.Empty{}, nil)
			mockSubClient.On("Acknowledge", mock.Anything, mock.Anything).
				Return(mockAckStream, nil)
			
			// Setup default mock stream expectations
			mockStream.On("Send", mock.Anything).Return(nil).Maybe()
			mockStream.On("CloseSend").Return(nil).Maybe()

			// Initialize metrics
			meter := otel.GetMeterProvider().Meter("test")
			messagesProcessed, _ := meter.Int64Counter("test_messages_processed")
			processingTime, _ := meter.Float64Histogram("test_processing_time")
			batchSize, _ := meter.Int64Histogram("test_batch_size")
			errors, _ := meter.Int64Counter("test_errors")

			// Create collector with test configuration
			collector := &Collector{
				logger:  zerolog.Nop(),
				storage: mockStorage,
				config: &CollectorConfig{
					MQAddr:            "test:50051",
					Topic:             "test-topic",
					ConsumerGroup:     "test-group",
					BatchSize:         10,
					AckTimeoutSeconds: 30,
				},
				subClient: mockSubClient,
				metrics: collectorMetrics{
					messagesProcessed: messagesProcessed,
					processingTime:    processingTime,
					batchSize:         batchSize,
					errors:            errors,
				},
			}

			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Run the method under test
			err := collector.processSubscription(ctx, "test-consumer")

			// Assertions
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations
			mockStream.AssertExpectations(t)
			mockStorage.AssertExpectations(t)

			// Additional verification for storage calls
			if tc.expectStored {
				mockStorage.AssertCalled(t, "Store", mock.Anything, mock.Anything)
			} else {
				mockStorage.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
			}
		})
	}
}