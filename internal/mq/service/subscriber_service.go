package service

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/sreeram77/gpu-tel/internal/mq/storage"
)

// SubscriberService implements the SubscriberService gRPC service
type SubscriberService struct {
	mqpb.UnimplementedSubscriberServiceServer
	logger       zerolog.Logger
	messageStore storage.MessageStore
}

// NewSubscriberService creates a new SubscriberService
func NewSubscriberService(logger zerolog.Logger, messageStore storage.MessageStore) *SubscriberService {
	return &SubscriberService{
		logger:       logger.With().Str("component", "subscriber_service").Logger(),
		messageStore: messageStore,
	}
}

// Subscribe handles the Subscribe RPC (bidirectional streaming)
func (s *SubscriberService) Subscribe(stream mqpb.SubscriberService_SubscribeServer) error {
	// Get the first request to initialize subscription
	req, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive initial request")
	}

	log := s.logger.With()
	if req.ConsumerId != "" {
		log = log.Str("consumer_id", req.ConsumerId)
	}
	if req.Topic != "" {
		log = log.Str("topic", req.Topic)
	}
	log = log.Str("method", "Subscribe")
	logger := log.Logger()

	// Validate request
	if req.Topic == "" {
		return status.Error(codes.InvalidArgument, "topic is required")
	}

	// Generate consumer ID if not provided
	consumerID := req.ConsumerId
	if consumerID == "" {
		consumerID = uuid.New().String()
		logger = logger.With().Str("generated_consumer_id", consumerID).Logger()
	}

	// Set default batch size if not specified
	batchSize := int(req.BatchSize)
	if batchSize <= 0 {
		batchSize = 10
	}

	// Set default max in-flight if not specified
	maxInFlight := int(req.MaxInFlight)
	if maxInFlight <= 0 {
		maxInFlight = 100
	}

	// Register the subscriber
	subscriber := storage.Subscriber{
		ID:            consumerID,
		Topic:         req.Topic,
		BatchSize:     batchSize,
		MessageFilter: req.Filter,
	}

	err = s.messageStore.AddSubscriber(stream.Context(), subscriber)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to add subscriber")
		return status.Error(codes.Internal, "failed to add subscriber")
	}
	defer s.messageStore.RemoveSubscriber(stream.Context(), consumerID, req.Topic)

	messageChan, err := s.messageStore.Subscribe(stream.Context(), req.Topic)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create subscription")
		return status.Error(codes.Internal, "failed to create subscription")
	}

	logger.Info().Msg("Successfully subscribed to topic")

	// Start a goroutine to handle message delivery
	deliverTicker := time.NewTicker(100 * time.Millisecond)
	defer deliverTicker.Stop()

	// Main loop for message delivery
	for {
		select {
		case msg := <-messageChan:
			s.logger.Debug().Msg("Sending to subscriber")

			// Send the message
			if err := stream.Send(&mqpb.SubscribeResponse{
				Messages: []*mqpb.Message{msg},
			}); err != nil {
				logger.Error().Err(err).Msg("Failed to send message")
			}
		case <-stream.Context().Done():
			// Client disconnected
			return nil
		}
	}
}

// Acknowledge handles the Acknowledge RPC (client streaming)
func (s *SubscriberService) Acknowledge(stream mqpb.SubscriberService_AcknowledgeServer) error {
	// Create logger with method field
	log := s.logger.With().Str("method", "Acknowledge")

	// Add peer info if available
	if p, ok := peer.FromContext(stream.Context()); ok && p.Addr != nil {
		log = log.Str("peer", p.Addr.String())
	}

	// Create the logger instance
	logger := log.Logger()

	// Track number of processed acknowledgments
	ackCount := 0

	// Process incoming acknowledgment requests
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Client is done sending requests
			logger.Info().Int("ack_count", ackCount).Msg("Client finished sending acknowledgments")
			return stream.SendAndClose(&emptypb.Empty{})
		}
		if err != nil {
			logger.Error().Err(err).Msg("Failed to receive acknowledgment request")
			return status.Error(codes.Internal, "failed to receive request")
		}

		reqLog := logger.With().
			Str("message_id", req.MessageId).
			Str("consumer_id", req.ConsumerId).
			Bool("success", req.Success).
			Logger()

		// Validate request
		if req.MessageId == "" {
			reqLog.Warn().Msg("Received invalid ack request: message_id is required")
			return status.Error(codes.InvalidArgument, "message_id is required")
		}
		if req.ConsumerId == "" {
			reqLog.Warn().Msg("Received invalid ack request: consumer_id is required")
			return status.Error(codes.InvalidArgument, "consumer_id is required")
		}

		// Acknowledge the message
		err = s.messageStore.Acknowledge(stream.Context(), req.MessageId, req.ConsumerId)
		if err != nil {
			reqLog.Error().Err(err).Msg("Failed to acknowledge message")
			return status.Error(codes.Internal, "failed to acknowledge message")
		}

		reqLog.Debug().Msg("Message acknowledged successfully")
		ackCount++
	}
}

// HealthCheck implements the HealthCheck RPC
func (s *SubscriberService) HealthCheck(ctx context.Context, req *mqpb.HealthCheckRequest) (*mqpb.HealthCheckResponse, error) {
	return &mqpb.HealthCheckResponse{
		Status:  mqpb.HealthCheckResponse_SERVING,
		Message: "subscriber service is healthy",
	}, nil
}
