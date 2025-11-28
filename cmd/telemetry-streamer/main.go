package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sreeram77/gpu-tel/internal/config"

	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

func main() {
	// Setup logger
	zerolog.TimeFieldFormat = time.RFC3339
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Resolve metrics path to absolute path if it's not already absolute
	if !filepath.IsAbs(cfg.Telemetry.MetricsPath) {
		cwd, err := os.Getwd()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to get current working directory")
		}
		cfg.Telemetry.MetricsPath = filepath.Clean(filepath.Join(cwd, cfg.Telemetry.MetricsPath))
	}

	logger.Info().Str("metrics_path", cfg.Telemetry.MetricsPath).Msg("Using metrics file")

	// Create streamer config from loaded configuration
	streamerCfg := &telemetry.Config{
		StreamInterval: cfg.Telemetry.StreamInterval,
		BatchSize:      cfg.Telemetry.BatchSize,
		MetricsPath:    cfg.Telemetry.MetricsPath,
	}

	logger.Info().
		Str("metrics_path", streamerCfg.MetricsPath).
		Dur("stream_interval", streamerCfg.StreamInterval).
		Int("batch_size", streamerCfg.BatchSize).
		Msg("Starting telemetry streamer with config")

	mqAddr := cfg.MessageQueue.Address

	// Create telemetry streamer
	logger.Info().
		Str("mq_addr", mqAddr).
		Str("metrics_path", streamerCfg.MetricsPath).
		Msg("Creating telemetry streamer")

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
