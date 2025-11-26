package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/sreeram77/gpu-tel/internal/config"
	"github.com/sreeram77/gpu-tel/internal/storage"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

func main() {
	// Initialize logger
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", "gpu-collector").
		Logger()

	// Initialize configuration
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize PostgreSQL storage
	pgStorage, err := storage.NewPostgresStorage(logger, &storage.PostgresConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PostgreSQL storage")
	}
	defer pgStorage.Close()

	// Initialize collector with PostgreSQL storage
	collector, err := telemetry.NewCollector(logger, pgStorage, &telemetry.CollectorConfig{
		MQAddr:            cfg.MessageQueue.Address,
		Topic:             "gpu_metrics",
		ConsumerGroup:     "gpu-collector",
		BatchSize:         cfg.Collector.BatchSize,
		MaxInFlight:       cfg.Collector.MaxInFlight,
		AckTimeoutSeconds: cfg.Collector.AckTimeoutSeconds,
		WorkerCount:       cfg.Collector.WorkerCount,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize collector")
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start collector
	if err := collector.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start collector")
	}

	logger.Info().
		Str("topic", "gpu_metrics").
		Int("worker_count", cfg.Collector.WorkerCount).
		Str("storage", "postgresql").
		Str("host", cfg.Database.Host).
		Int("port", cfg.Database.Port).
		Str("dbname", cfg.Database.DBName).
		Msg("GPU Telemetry Collector started")

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down...")

	// Shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop collector
	if err := collector.Stop(); err != nil {
		logger.Error().Err(err).Msg("Error during collector shutdown")
	}

	// Wait for all goroutines to finish or timeout
	select {
	case <-shutdownCtx.Done():
		logger.Warn().Msg("Shutdown timed out, forcing exit")
	}

	logger.Info().Msg("Shutdown complete")
}
