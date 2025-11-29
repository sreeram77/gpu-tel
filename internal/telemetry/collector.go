package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
)

// Collector implements the TelemetryCollector interface
type Collector struct {
	logger    zerolog.Logger
	config    *CollectorConfig
	storage   TelemetryStorage
	subClient mqpb.SubscriberServiceClient
	conn      *grpc.ClientConn

	cancel context.CancelFunc
	wg     sync.WaitGroup

	metrics collectorMetrics
}

// CollectorConfig holds configuration for the Collector
type CollectorConfig struct {
	MQAddr            string
	Topic             string
	ConsumerGroup     string
	BatchSize         int
	MaxInFlight       int
	AckTimeoutSeconds int
	WorkerCount       int
}

// collectorMetrics holds all metrics for the collector
type collectorMetrics struct {
	messagesProcessed metric.Int64Counter
	processingTime    metric.Float64Histogram
	batchSize         metric.Int64Histogram
	errors            metric.Int64Counter
}

// NewCollector creates a new Collector instance
func NewCollector(logger zerolog.Logger, storage TelemetryStorage, cfg *CollectorConfig) (*Collector, error) {
	if cfg == nil {
		cfg = &CollectorConfig{
			Topic:             "gpu_metrics",
			ConsumerGroup:     "gpu-collector",
			BatchSize:         100,
			MaxInFlight:       1000,
			AckTimeoutSeconds: 30,
			WorkerCount:       3,
		}
	}

	// Set up a connection to the message queue server
	conn, err := grpc.Dial(cfg.MQAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to message queue: %w", err)
	}

	// Create gRPC client
	subClient := mqpb.NewSubscriberServiceClient(conn)

	// Initialize metrics
	meter := otel.GetMeterProvider().Meter("github.com/sreeram77/gpu-tel/collector")
	messagesProcessed, _ := meter.Int64Counter(
		"collector_messages_processed_total",
		metric.WithDescription("Total number of messages processed"),
	)
	processingTime, _ := meter.Float64Histogram(
		"collector_processing_time_seconds",
		metric.WithDescription("Time taken to process a batch of messages"),
		metric.WithUnit("s"),
	)
	batchSize, _ := meter.Int64Histogram(
		"collector_batch_size",
		metric.WithDescription("Size of message batches being processed"),
	)
	errors, _ := meter.Int64Counter(
		"collector_errors_total",
		metric.WithDescription("Total number of processing errors"),
	)

	return &Collector{
		logger:    logger,
		config:    cfg,
		storage:   storage,
		subClient: subClient,
		conn:      conn,
		metrics: collectorMetrics{
			messagesProcessed: messagesProcessed,
			processingTime:    processingTime,
			batchSize:         batchSize,
			errors:            errors,
		},
	}, nil
}

// Start begins collecting telemetry data
func (c *Collector) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	errCh := make(chan error, 1)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		// Forward any errors from runSubscriptionLoop
		errCh <- c.runSubscriptionLoop(ctx)
	}()

	c.logger.Info().
		Str("topic", c.config.Topic).
		Str("consumer_group", c.config.ConsumerGroup).
		Msg("Started telemetry collector")

	// Wait for either context cancellation or an error from the subscription loop
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runSubscriptionLoop continuously processes messages from the subscription
func (c *Collector) runSubscriptionLoop(ctx context.Context) error {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			podName = fmt.Sprintf("sink-%d", time.Now().UnixNano())
		} else {
			podName = hostname
		}
	}

	consumerID := fmt.Sprintf("%s-%s", c.config.ConsumerGroup, podName)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("Stopping subscription loop due to context cancellation")
			return ctx.Err()
		default:
			if err := c.processSubscription(ctx, consumerID); err != nil {
				if ctx.Err() != nil {
					// If context was canceled, return the context error
					return ctx.Err()
				}
				c.logger.Error().Err(err).Msg("Error in subscription loop")
				// Add a small delay to prevent tight loop on errors
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// processSubscription handles the subscription and processing of messages
func (c *Collector) processSubscription(ctx context.Context, consumerID string) error {
	ctx, span := otel.Tracer("collector").Start(ctx, "processSubscription")
	defer span.End()

	// Create a new stream for this subscription
	stream, err := c.subClient.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to create subscribe stream: %w", err)
	}
	defer func() {
		if err := stream.CloseSend(); err != nil {
			c.logger.Error().Err(err).Msg("Error closing subscribe stream")
		}
	}()

	// Send initial subscription request
	subReq := &mqpb.SubscribeRequest{
		Topic:             c.config.Topic,
		ConsumerId:        consumerID,
		BatchSize:         int32(c.config.BatchSize),
		MaxInFlight:       int32(c.config.MaxInFlight),
		AckTimeoutSeconds: int32(c.config.AckTimeoutSeconds),
	}

	if err := stream.Send(subReq); err != nil {
		return fmt.Errorf("failed to send subscribe request: %w", err)
	}

	// Continuously receive and process messages
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Receive messages
			resp, err := stream.Recv()
			if err != nil {
				return fmt.Errorf("failed to receive messages: %w", err)
			}

			// Skip heartbeats and empty messages
			if resp.Heartbeat || len(resp.Messages) == 0 {
				continue
			}

			// Log the received messages
			c.logger.Info().
				Int("batch_size", len(resp.Messages)).
				Msg("Received batch of messages")

			startTime := time.Now()
			batchSize := len(resp.Messages)

			// Process messages in batch
			batch := TelemetryBatch{
				BatchID:   fmt.Sprintf("batch-%s", time.Now().Format("20060102-150405.000")),
				Timestamp: time.Now(),
				Telemetry: make([]GPUTelemetry, 0, batchSize),
			}

			// Track message IDs for acknowledgment
			messageIDs := make([]string, 0, batchSize)

			for _, msg := range resp.Messages {
				// Parse the message payload
				var telemetry GPUTelemetry
				if err := json.Unmarshal(msg.Payload, &telemetry); err != nil {
					c.logger.Error().
						Err(err).
						Str("message_id", msg.Id).
						Msg("Failed to unmarshal telemetry data")
					continue
				}

				batch.Telemetry = append(batch.Telemetry, telemetry)
				messageIDs = append(messageIDs, msg.Id)
			}

			// Store the batch if we have any valid messages
			if len(batch.Telemetry) > 0 {
				if err := c.storage.Store(ctx, batch); err != nil {
					c.logger.Error().
						Err(err).
						Msg("Failed to store batch, continuing to process messages")
					continue
				}

				// Record metrics
				c.metrics.messagesProcessed.Add(ctx, int64(len(batch.Telemetry)))
				c.metrics.batchSize.Record(ctx, int64(len(batch.Telemetry)))

				// Acknowledge processed messages
				if err := c.acknowledgeMessages(ctx, messageIDs, consumerID); err != nil {
					c.logger.Error().
						Err(err).
						Strs("message_ids", messageIDs).
						Msg("Failed to acknowledge messages")
					// Continue processing even if ack fails
				}

				// Record processing time
				c.metrics.processingTime.Record(ctx, time.Since(startTime).Seconds())
			}
		}
	}
}

// acknowledgeMessages sends acknowledgment for processed messages
func (c *Collector) acknowledgeMessages(ctx context.Context, messageIDs []string, consumerID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// Create a stream for acknowledgments
	ackStream, err := c.subClient.Acknowledge(ctx)
	if err != nil {
		return fmt.Errorf("failed to create ack stream: %w", err)
	}

	// Send acknowledgments in batches
	batchSize := 100 // Acknowledge in batches to avoid overwhelming the server
	for i := 0; i < len(messageIDs); i += batchSize {
		end := i + batchSize
		if end > len(messageIDs) {
			end = len(messageIDs)
		}

		batch := messageIDs[i:end]
		for _, msgID := range batch {
			ackReq := &mqpb.AcknowledgeRequest{
				MessageId:  msgID,
				ConsumerId: consumerID,
				Success:    true,
			}
			if err := ackStream.Send(ackReq); err != nil {
				return fmt.Errorf("failed to send ack for message %s: %w", msgID, err)
			}
		}

		// Close the stream after each batch
		if _, err := ackStream.CloseAndRecv(); err != nil {
			return fmt.Errorf("failed to close ack stream: %w", err)
		}
	}

	return nil
}

// Stop gracefully stops the collector
func (c *Collector) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	var errs []error

	// Close the storage connection
	if c.storage != nil {
		if err := c.storage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing storage: %w", err))
		}
	}

	// Close the gRPC connection
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing gRPC connection: %w", err))
		}
	}

	// Return the first error if any occurred
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
