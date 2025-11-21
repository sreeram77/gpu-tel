package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

func main() {
	// Setup logger
	zerolog.TimeFieldFormat = time.RFC3339
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	// Create streamer config
	streamerCfg := &telemetry.Config{
		StreamInterval: 5 * time.Second,
		BatchSize:      10,
	}

	// Message queue server address
	mqAddr := "localhost:50051" // Default address, can be made configurable

	// Create telemetry streamer
	streamer, err := telemetry.NewStreamer(logger, streamerCfg, mqAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create telemetry streamer")
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the streamer
	if err := streamer.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start streamer")
	}

	// Handle interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigCh
	logger.Info().Msg("Shutting down...")

	// Stop the streamer
	if err := streamer.Stop(); err != nil {
		logger.Error().Err(err).Msg("Error stopping streamer")
	}

	logger.Info().Msg("Streamer stopped")
}
