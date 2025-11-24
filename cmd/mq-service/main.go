package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/sreeram77/gpu-tel/internal/config"
	mqgrpc "github.com/sreeram77/gpu-tel/internal/mq/transport/grpc"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "./configs/config.yaml", "path to config file")
	flag.Parse()

	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", "mq-service").
		Logger()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Configure log level
	logLevel, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Create and start gRPC server
	server := mqgrpc.NewServer(logger, cfg)

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Create a context that cancels when an interrupt signal is received
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		if err := server.Run(ctx); err != nil {
			logger.Fatal().Err(err).Msg("Failed to run server")
		}
	}()

	logger.Info().
		Str("version", "0.1.0").
		Msg("Message Queue Service started")

	// Wait for interrupt signal
	<-stop
	logger.Info().Msg("Shutting down server...")

	// Cancel the context to trigger graceful shutdown
	cancel()

	logger.Info().Msg("Server stopped")
}
