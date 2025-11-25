package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/sreeram77/gpu-tel/internal/config"
	"github.com/sreeram77/gpu-tel/internal/mq/storage"
)

func TestNewServer(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPC: config.GRPCServerConfig{
				Port: 50051,
			},
		},
	}

	// Execute
	server := NewServer(logger, cfg)

	// Verify
	assert.NotNil(t, server)
	assert.NotNil(t, server.grpcServer)
}

func TestServer_StartAndStop(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPC: config.GRPCServerConfig{
				Port: 0, // Let OS choose an available port
			},
		},
	}

	server := NewServer(logger, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Run(ctx)
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running by checking if the port is open
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err == nil {
		conn.Close()
	} else {
		t.Fatalf("Failed to connect to server: %v", err)
	}

	// Test graceful shutdown
	cancel()

	// Wait for server to stop
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

func TestServer_StopWithoutStart(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPC: config.GRPCServerConfig{
				Port: 0,
			},
		},
	}

	server := NewServer(logger, cfg)

	// Test stopping a server that wasn't started
	err := server.Stop()
	assert.NoError(t, err)
}

func TestServer_RunWithError(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	// Use a privileged port that requires root to bind to
	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPC: config.GRPCServerConfig{
				Port: 1, // Port 1 is a privileged port
			},
		},
	}

	server := NewServer(logger, cfg)

	// Test starting server with invalid port
	err := server.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to listen on :1")
}

func TestServer_MessageStoreClosure(t *testing.T) {
	// Setup
	logger := zerolog.Nop()
	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPC: config.GRPCServerConfig{
				Port: 0,
			},
		},
	}

	// Create a mock message store that tracks if Close was called
	mockStore := &mockMessageStore{
		closeCalled: make(chan struct{}),
	}

	// Create server with our mock store
	server := &Server{
		logger:       logger,
		config:       cfg,
		grpcServer:   grpc.NewServer(),
		messageStore: mockStore,
	}

	// Start server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Run(ctx)
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	cancel()

	// Verify message store Close was called
	select {
	case <-mockStore.closeCalled:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Message store Close() was not called during server shutdown")
	}
}

// mockMessageStore implements storage.MessageStore for testing
type mockMessageStore struct {
	storage.MessageStore
	closeCalled chan struct{}
}

func (m *mockMessageStore) Close() error {
	close(m.closeCalled)
	return nil
}

// Implement other required methods of storage.MessageStore with no-op implementations
func (m *mockMessageStore) Store(ctx context.Context, msg *mq.Message) error {
	return nil
}

func (m *mockMessageStore) Get(ctx context.Context, id string) (*mq.Message, error) {
	return nil, nil
}

func (m *mockMessageStore) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockMessageStore) Subscribe(ctx context.Context, topic string) (<-chan *mq.Message, error) {
	return make(chan *mq.Message), nil
}

func (m *mockMessageStore) Acknowledge(ctx context.Context, msgID string, consumerID string) error {
	return nil
}

func (m *mockMessageStore) AddSubscriber(ctx context.Context, sub storage.Subscriber) error {
	return nil
}

func (m *mockMessageStore) RemoveSubscriber(ctx context.Context, id, topic string) error {
	return nil
}

func (m *mockMessageStore) GetMessages(ctx context.Context, topic, consumerID string, batchSize int) ([]*mq.Message, error) {
	return nil, nil
}
