package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/sreeram77/gpu-tel/internal/config"
	"github.com/sreeram77/gpu-tel/internal/mq/service"
	"github.com/sreeram77/gpu-tel/internal/mq/storage"
)

// Server represents the gRPC server
type Server struct {
	logger       zerolog.Logger
	config       *config.Config
	grpcServer   *grpc.Server
	messageStore storage.MessageStore
}

// NewServer creates a new gRPC server
func NewServer(logger zerolog.Logger, cfg *config.Config) *Server {
	// Create message store
	messageStore := storage.NewInMemoryStore()

	// Create gRPC server with options
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
	}

	grpcServer := grpc.NewServer(opts...)

	// Create services
	publisherService := service.NewPublisherService(logger, messageStore)
	subscriberService := service.NewSubscriberService(logger, messageStore)

	// Register services
	mqpb.RegisterPublisherServiceServer(grpcServer, publisherService)
	mqpb.RegisterSubscriberServiceServer(grpcServer, subscriberService)

	// Enable reflection for gRPC CLI tools like grpcurl
	reflection.Register(grpcServer)

	return &Server{
		logger:       logger,
		config:       cfg,
		grpcServer:   grpcServer,
		messageStore: messageStore,
	}
}

// Start starts the gRPC server
func (s *Server) Start() error {
	// Create listener
	addr := fmt.Sprintf(":%d", s.config.Server.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	s.logger.Info().
		Str("address", addr).
		Msg("Starting gRPC server")

	// Start serving
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() error {
	s.logger.Info().Msg("Shutting down gRPC server")

	// Graceful stop
	s.grpcServer.GracefulStop()

	// Close message store
	if err := s.messageStore.Close(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to close message store")
	}

	s.logger.Info().Msg("gRPC server stopped")
	return nil
}

// Run starts the server and handles graceful shutdown
func (s *Server) Run(ctx context.Context) error {
	// Channel to listen for errors from server goroutine
	serverErrors := make(chan error, 1)

	// Start the gRPC server in a goroutine
	go func() {
		if err := s.Start(); err != nil {
			serverErrors <- err
		}
	}()

	// Listen for context cancellation or server error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %v", err)
	case <-ctx.Done():
		// Context was cancelled, perform graceful shutdown
		return s.Stop()
	}
}
