package telemetry

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"github.com/rs/zerolog"

	mqpb "github.com/sreeram77/gpu-tel/api/v1/mq"
)

// GPUMetric represents a single GPU metric from the CSV file
type GPUMetric struct {
	Timestamp  time.Time `json:"timestamp"`
	MetricName string    `json:"metric_name"`
	GPUIndex   string    `json:"gpu_index"`
	Device     string    `json:"device"`
	UUID       string    `json:"uuid"`
	ModelName  string    `json:"model_name"`
	Hostname   string    `json:"hostname"`
	Container  string    `json:"container"`
	Pod        string    `json:"pod"`
	Namespace  string    `json:"namespace"`
	Value      string    `json:"value"`
	Labels     string    `json:"labels_raw"`
}

// Streamer implements the TelemetryStreamer interface
type Streamer struct {
	logger     zerolog.Logger
	config     *Config
	metrics    []GPUMetric
	currentIdx int
	mqClient   mqpb.PublisherServiceClient
	conn       *grpc.ClientConn
	done       chan struct{}
}

// Config holds configuration for the Streamer
type Config struct {
	StreamInterval time.Duration
	BatchSize      int
}

// NewStreamer creates a new Streamer instance
func NewStreamer(logger zerolog.Logger, cfg *Config, mqAddr string) (*Streamer, error) {
	if cfg == nil {
		cfg = &Config{
			StreamInterval: time.Second,
			BatchSize:      10,
		}
	}

	// Set up a connection to the message queue server
	conn, err := grpc.Dial(mqAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to message queue: %w", err)
	}

	// Create gRPC client
	mqClient := mqpb.NewPublisherServiceClient(conn)

	return &Streamer{
		logger:   logger,
		config:   cfg,
		mqClient: mqClient,
		conn:     conn,
		done:     make(chan struct{}),
	}, nil
}

// Start begins streaming telemetry data
func (s *Streamer) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.config.StreamInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Collect and process telemetry data
			s.collectAndProcess(ctx)
		}
	}
}

// Stop gracefully stops the streamer
func (s *Streamer) Stop() error {
	close(s.done)
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// loadMetrics loads metrics from the CSV file
func (s *Streamer) loadMetrics() error {
	// Get the absolute path to the metrics file
	metricsPath := filepath.Join("./test-data", "metrics.csv")

	// Open the CSV file
	file, err := os.Open(metricsPath)
	if err != nil {
		return fmt.Errorf("failed to open metrics file: %w", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)

	// Read the header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Create a map of column indices
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[col] = i
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV records: %w", err)
	}

	// Parse records into GPUMetric structs
	for _, record := range records {
		if len(record) < len(header) {
			continue // Skip malformed records
		}

		timestamp, _ := time.Parse(time.RFC3339, record[colMap["timestamp"]])
		metric := GPUMetric{
			Timestamp:  timestamp,
			MetricName: record[colMap["metric_name"]],
			GPUIndex:   record[colMap["gpu_id"]],
			Device:     record[colMap["device"]],
			UUID:       record[colMap["uuid"]],
			ModelName:  record[colMap["modelName"]],
			Hostname:   record[colMap["Hostname"]],
			Container:  record[colMap["container"]],
			Pod:        record[colMap["pod"]],
			Namespace:  record[colMap["namespace"]],
			Value:      record[colMap["value"]],
			Labels:     record[colMap["labels_raw"]],
		}

		s.metrics = append(s.metrics, metric)
	}

	s.logger.Info().Int("count", len(s.metrics)).Msg("Loaded metrics from CSV")
	return nil
}

// collectAndProcess collects and processes a batch of metrics
func (s *Streamer) collectAndProcess(ctx context.Context) {
	// Lazy load metrics on first run
	if s.metrics == nil {
		if err := s.loadMetrics(); err != nil {
			s.logger.Error().Err(err).Msg("Failed to load metrics")
			return
		}
	}

	// If we've reached the end, start over
	if s.currentIdx >= len(s.metrics) {
		s.currentIdx = 0
	}

	// Get the next batch of metrics
	endIdx := s.currentIdx + s.config.BatchSize
	if endIdx > len(s.metrics) {
		endIdx = len(s.metrics)
	}

	batch := s.metrics[s.currentIdx:endIdx]
	s.currentIdx = endIdx

	// Create a stream to the message queue
	stream, err := s.mqClient.Publish(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create publish stream")
		return
	}

	// Process each metric in the batch
	for _, metric := range batch {
		value, _ := strconv.ParseFloat(metric.Value, 64)
		memoryTotal := uint64(80 * 1024 * 1024 * 1024)               // 80GB in bytes for H100
		memoryUsed := uint64(float64(memoryTotal) * (value / 100.0)) // Estimate based on utilization

		// Create telemetry data using the local GPUTelemetry struct
		telemetry := GPUTelemetry{
			ID:             metric.UUID,
			GPUIndex:       mustAtoi(metric.GPUIndex),
			GPUName:        metric.ModelName,
			GPUTemperature: 60.0 + (value / 2), // Simulate temperature based on load
			GPULoad:        value,
			MemoryUsed:     memoryUsed,
			MemoryTotal:    memoryTotal,
			PowerDraw:      300.0 * (value / 100.0), // Simulate power draw based on load (max 300W)
			PowerLimit:     300.0,                   // H100 power limit
			FanSpeed:       30.0 + (value * 0.7),    // Simulate fan speed
			Timestamp:      time.Now(),
		}

		// Marshal telemetry to JSON for the message payload
		payload, err := json.Marshal(telemetry)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to marshal telemetry")
			continue
		}

		// Create publish request
		req := &mqpb.PublishRequest{
			Message: &mqpb.Message{
				Id:        fmt.Sprintf("msg-%s-%d", metric.UUID, time.Now().UnixNano()),
				Topic:     "gpu_metrics",
				Payload:   payload,
				Timestamp: timestamppb.Now(),
			},
			WaitForAck:      true,
			AckTimeoutSeconds: 5, // 5 second timeout for acknowledgment
		}

		// Send the message
		if err := stream.Send(req); err != nil {
			s.logger.Error().Err(err).Str("gpu_id", metric.UUID).Msg("Failed to send telemetry")
			continue
		}
	}

	// Close the send direction of the stream
	if err := stream.CloseSend(); err != nil {
		s.logger.Error().Err(err).Msg("Error closing send direction of stream")
	}

	s.logger.Info().
		Int("batch_size", len(batch)).
		Int("total_metrics", len(s.metrics)).
		Msg("Processed telemetry batch")
}

// Helper function to convert string to int, returns 0 on error
func mustAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
