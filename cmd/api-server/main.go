package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sreeram77/gpu-tel/internal/api"
	"github.com/sreeram77/gpu-tel/internal/storage"
	"github.com/sreeram77/gpu-tel/internal/config"
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

	// Create and start API server
	server := api.NewServer(logger, pgStorage)

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
