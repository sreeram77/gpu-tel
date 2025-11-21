package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/sreeram77/gpu-tel/internal/mq/storage"
)

// PublisherService implements the PublisherService gRPC service
type PublisherService struct {
	mqpb.UnimplementedPublisherServiceServer
	logger      zerolog.Logger
	messageStore storage.MessageStore
}

// NewPublisherService creates a new PublisherService
func NewPublisherService(logger zerolog.Logger, messageStore storage.MessageStore) *PublisherService {
	return &PublisherService{
		logger:       logger.With().Str("component", "publisher_service").Logger(),
		messageStore: messageStore,
	}
}

// Publish handles the Publish RPC
func (s *PublisherService) Publish(stream mqpb.PublisherService_PublishServer) error {
	ctx := stream.Context()
	log := s.logger.With().Str("method", "Publish").Logger()

	for {
		// Receive publish request from client
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				log.Debug().Msg("Client closed the stream")
				return nil
			}
			log.Error().Err(err).Msg("Failed to receive publish request")
			return status.Errorf(codes.Internal, "failed to receive publish request: %v", err)
		}

		// Validate request
		if req.Message == nil {
			return status.Error(codes.InvalidArgument, "message is required")
		}

		// Set message ID if not provided
		if req.Message.Id == "" {
			req.Message.Id = uuid.New().String()
		}

		// Set timestamp if not provided
		if req.Message.Timestamp == nil {
			req.Message.Timestamp = timestamppb.Now()
		}

		// Store the message
		if err := s.messageStore.Store(ctx, req.Message); err != nil {
			log.Error().Err(err).Str("message_id", req.Message.Id).Msg("Failed to store message")
			return status.Errorf(codes.Internal, "failed to store message: %v", err)
		}

		// Send acknowledgment if requested
		if req.WaitForAck {
			resp := &mqpb.PublishResponse{
				MessageId: req.Message.Id,
				Success:   true,
				Timestamp: timestamppb.Now(),
			}

			// Set a timeout for sending the acknowledgment
			timeout := time.Duration(req.AckTimeoutSeconds) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second // Default timeout
			}

			ackCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			select {
			case <-ackCtx.Done():
				log.Warn().Str("message_id", req.Message.Id).Msg("Timed out waiting to send acknowledgment")
				return status.Error(codes.DeadlineExceeded, "timed out waiting to send acknowledgment")
			default:
				if err := stream.Send(resp); err != nil {
					log.Error().Err(err).Str("message_id", req.Message.Id).Msg("Failed to send acknowledgment")
					return status.Errorf(codes.Internal, "failed to send acknowledgment: %v", err)
				}
			}
		}

		log.Debug().
			Str("message_id", req.Message.Id).
			Str("topic", req.Message.Topic).
			Msg("Message published successfully")
	}
}

// HealthCheck implements the HealthCheck RPC
func (s *PublisherService) HealthCheck(ctx context.Context, req *mqpb.HealthCheckRequest) (*mqpb.HealthCheckResponse, error) {
	return &mqpb.HealthCheckResponse{
		Status:  mqpb.HealthCheckResponse_SERVING,
		Message: "publisher service is healthy",
	}, nil
}
