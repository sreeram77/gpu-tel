package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sreeram77/gpu-tel/internal/api"
	"github.com/sreeram77/gpu-tel/internal/config"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

func main() {
	// Initialize logger
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().
		Timestamp().
		Logger()

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Log the MQ address being used
	logger.Info().
		Str("mq_addr", cfg.MessageQueue.Address).
		Str("topic", cfg.MessageQueue.Topic).
		Msg("Connecting to message queue")

	// Initialize in-memory storage
	memStorage := telemetry.NewMemoryStorage()
	defer memStorage.Close()

	// Initialize collector
	collector, err := telemetry.NewCollector(logger, memStorage, &telemetry.CollectorConfig{
		MQAddr:            cfg.MessageQueue.Address,
		Topic:             cfg.MessageQueue.Topic,
		ConsumerGroup:     cfg.MessageQueue.ConsumerGroup,
		BatchSize:         cfg.Collector.BatchSize,
		MaxInFlight:       cfg.Collector.MaxInFlight,
		AckTimeoutSeconds: cfg.Collector.AckTimeoutSeconds,
		WorkerCount:       cfg.Collector.WorkerCount,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize collector")
	}

	// Create and start API server
	server := api.NewServer(logger, memStorage)

	// Create a context that listens for the interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the API server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start API server")
		}
	}()

	time.Sleep(5 * time.Second)

	// Start the collector
	if err := collector.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start collector")
	}

	logger.Info().
		Str("service", "sink").
		Str("mq_address", cfg.MessageQueue.Address).
		Msg("GPU Telemetry Service started")

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info().Msg("Shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown API server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Error during API server shutdown")
	}

	// Stop collector
	if err := collector.Stop(); err != nil {
		logger.Error().Err(err).Msg("Error during collector shutdown")
	}

	<-shutdownCtx.Done()

	logger.Info().Msg("Shutdown complete")
}
