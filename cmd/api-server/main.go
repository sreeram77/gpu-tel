package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sreeram77/gpu-tel/internal/api"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

func main() {
	// Initialize logger
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().
		Timestamp().
		Logger()

	// Initialize in-memory storage
	memStorage := telemetry.NewMemoryStorage()
	defer memStorage.Close()

	// Create and start API server
	server := api.NewServer(logger, memStorage)

	// Create a context that listens for the interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start API server")
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	<-ctx.Done()
	logger.Info().Msg("Shutting down...")

	// Attempt to gracefully shut down the server
	if err := server.Shutdown(context.Background()); err != nil {
		logger.Error().Err(err).Msg("Error during server shutdown")
	}

	logger.Info().Msg("Server stopped")
}
