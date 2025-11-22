package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
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
	done      chan struct{}
	wg        sync.WaitGroup
	metrics   collectorMetrics
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
		done:      make(chan struct{}),
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
	// Start multiple worker goroutines
	for i := 0; i < c.config.WorkerCount; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}

	c.logger.Info().
		Int("worker_count", c.config.WorkerCount).
		Str("topic", c.config.Topic).
		Msg("Started telemetry collector workers")

	return nil
}

// worker is a single worker goroutine that processes messages
func (c *Collector) worker(ctx context.Context, workerID int) {
	defer c.wg.Done()

	logger := c.logger.With().Int("worker_id", workerID).Logger()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Stopping worker due to context cancellation")
			return
		case <-c.done:
			logger.Info().Msg("Stopping worker")
			return
		default:
			if err := c.processBatch(ctx, workerID); err != nil {
				logger.Error().Err(err).Msg("Error processing batch")
				// Add a small delay to prevent tight loop on errors
				time.Sleep(time.Second)
			}
		}
	}
}

// processBatch processes a single batch of messages
func (c *Collector) processBatch(ctx context.Context, workerID int) error {
	ctx, span := otel.Tracer("collector").Start(ctx, "processBatch")
	defer span.End()

	// Create a unique consumer ID for this worker
	consumerID := fmt.Sprintf("%s-worker-%d", c.config.ConsumerGroup, workerID)

	// Subscribe to the message queue
	stream, err := c.subClient.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to create subscribe stream: %w", err)
	}
	defer func() {
		if err := stream.CloseSend(); err != nil {
			c.logger.Error().Err(err).Msg("Error closing subscribe stream")
		}
	}()

	// Send subscription request
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

	// Receive messages
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive messages: %w", err)
	}

	// Skip heartbeats
	if resp.Heartbeat || len(resp.Messages) == 0 {
		return nil
	}

	startTime := time.Now()
	batchSize := len(resp.Messages)

	// Process messages in batch
	batch := TelemetryBatch{
		BatchID:   fmt.Sprintf("batch-%d-%s", workerID, time.Now().Format("20060102-150405.000")),
		Timestamp: time.Now(),
		Telemetry: make([]GPUTelemetry, 0, batchSize),
	}

	// Track message IDs for acknowledgment
	messageIDs := make([]string, 0, batchSize)

	for _, msg := range resp.Messages {
		// Parse the message payload
		var telemetry GPUTelemetry
		if err := json.Unmarshal(msg.Payload, &telemetry); err != nil {
			c.logger.Error().Err(err).Str("message_id", msg.Id).Msg("Failed to unmarshal telemetry")
			c.metrics.errors.Add(ctx, 1)
			continue
		}

		batch.Telemetry = append(batch.Telemetry, telemetry)
		messageIDs = append(messageIDs, msg.Id)
	}

	// Store the batch if we have any valid telemetry
	if len(batch.Telemetry) > 0 {
		if err := c.storage.Store(ctx, batch); err != nil {
			c.logger.Error().Err(err).Msg("Failed to store telemetry batch")
			c.metrics.errors.Add(ctx, 1)
			return fmt.Errorf("failed to store telemetry batch: %w", err)
		}
	}

	// Acknowledge processed messages
	if err := c.acknowledgeMessages(ctx, messageIDs, consumerID); err != nil {
		c.logger.Error().Err(err).Msg("Failed to acknowledge messages")
		return fmt.Errorf("failed to acknowledge messages: %w", err)
	}

	// Record metrics
	processingTime := time.Since(startTime).Seconds()
	c.metrics.messagesProcessed.Add(ctx, int64(len(batch.Telemetry)))
	c.metrics.processingTime.Record(ctx, processingTime)
	c.metrics.batchSize.Record(ctx, int64(batchSize))

	c.logger.Debug().
		Int("batch_size", len(batch.Telemetry)).
		Float64("processing_time_seconds", processingTime).
		Msg("Processed batch of telemetry")

	return nil
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
	close(c.done)
	c.wg.Wait()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
