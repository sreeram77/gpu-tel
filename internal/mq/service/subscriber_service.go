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

	logger.Info().Msg("Successfully subscribed to topic")

	// Start a goroutine to handle message delivery
	deliverTicker := time.NewTicker(100 * time.Millisecond)
	defer deliverTicker.Stop()

	// Channel to track in-flight messages
	inFlight := make(chan struct{}, maxInFlight)

	// Channel to signal when to deliver the next batch
	deliverChan := make(chan struct{}, 1)
	defer close(deliverChan)

	// Initial delivery
	deliverChan <- struct{}{}

	// Function to deliver a batch of messages
	deliverBatch := func() {
		// Skip if we're already at max in-flight
		if len(inFlight) >= maxInFlight {
			return
		}

		// Get a batch of messages
		messages, err := s.messageStore.GetMessages(stream.Context(), req.Topic, consumerID, batchSize)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get messages")
			return
		}

		if len(messages) == 0 {
			// Send a heartbeat if no messages
			if err := stream.Send(&mqpb.SubscribeResponse{
				Heartbeat: true,
			}); err != nil {
				logger.Error().Err(err).Msg("Failed to send heartbeat")
			}
			return
		}

		// Add to in-flight count
		for range messages {
			select {
			case inFlight <- struct{}{}:
			default:
				// Shouldn't happen due to the check at the start
				logger.Warn().Msg("In-flight queue full, skipping message delivery")
				return
			}
		}

		// Send the batch
		resp := &mqpb.SubscribeResponse{
			Messages:  messages,
			Heartbeat: false,
		}

		if err := stream.Send(resp); err != nil {
			logger.Error().Err(err).Msg("Failed to send messages")
			// Remove from in-flight count on error
			for range messages {
				<-inFlight
			}
		}
	}

	// Handle incoming control messages in a goroutine
	go func() {
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				// Client closed the stream
				return
			}
			if err != nil {
				logger.Error().Err(err).Msg("Error receiving from stream")
				return
			}

			// Handle control messages if needed
			// For now, just trigger a new batch delivery
			select {
			case deliverChan <- struct{}{}:
			default:
				// Skip if we already have a pending delivery
			}
		}
	}()

	// Main loop for message delivery
	for {
		select {
		case <-stream.Context().Done():
			// Client disconnected
			return nil

		case <-deliverChan:
			// Deliver a batch of messages
			deliverBatch()

		case <-deliverTicker.C:
			// Periodically check for new messages
			select {
			case deliverChan <- struct{}{}:
			default:
				// Skip if we already have a pending delivery
			}
		}
	}
}

// Acknowledge handles the Acknowledge RPC (client streaming)
func (s *SubscriberService) Acknowledge(stream mqpb.SubscriberService_AcknowledgeServer) error {
	// Get peer info for logging
	peer, _ := peer.FromContext(stream.Context())
	log := s.logger.With().
		Str("method", "Acknowledge").
		Str("peer", peer.Addr.String()).
		Logger()

	// Track number of processed acknowledgments
	ackCount := 0

	// Process incoming acknowledgment requests
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Client is done sending requests
			log.Info().Int("ack_count", ackCount).Msg("Client finished sending acknowledgments")
			return stream.SendAndClose(&emptypb.Empty{})
		}
		if err != nil {
			log.Error().Err(err).Msg("Failed to receive acknowledgment request")
			return status.Error(codes.Internal, "failed to receive request")
		}

		reqLog := log.With().
			Str("message_id", req.MessageId).
			Str("consumer_id", req.ConsumerId).
			Bool("success", req.Success).
			Logger()

		// Validate request
		if req.MessageId == "" {
			reqLog.Warn().Msg("Received invalid ack request: message_id is required")
			continue
		}
		if req.ConsumerId == "" {
			reqLog.Warn().Msg("Received invalid ack request: consumer_id is required")
			continue
		}

		// Acknowledge the message
		err = s.messageStore.Acknowledge(stream.Context(), req.MessageId, req.ConsumerId)
		if err != nil {
			reqLog.Error().Err(err).Msg("Failed to acknowledge message")
			continue
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
