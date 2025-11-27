package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

// Server represents the API server
type Server struct {
	router     *gin.Engine
	logger     zerolog.Logger
	httpServer *http.Server
	storage    telemetry.TelemetryStorage
}

// NewServer creates a new API server instance
func NewServer(logger zerolog.Logger, storage telemetry.TelemetryStorage) *Server {
	srv := &Server{
		logger:  logger,
		storage: storage,
	}

	// Configure Gin
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	srv.router = gin.New()
	srv.router.Use(
		gin.Recovery(),
		requestLogger(logger),
	)

	// Register routes
	srv.registerRoutes()

	// Create HTTP server
	srv.httpServer = &http.Server{
		Addr:    ":8080",
		Handler: srv.router,
	}

	return srv
}

// Start starts the API server
func (s *Server) Start() error {
	s.logger.Info().Str("addr", s.httpServer.Addr).Msg("Starting API server")

	// Start server in a separate goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for either an error or interrupt signal
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("Shutting down server...")

	// Create a deadline to wait for
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	// Shutdown the server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error().Err(err).Msg("Error during server shutdown")
		return err
	}

	s.logger.Info().Msg("Server stopped")
	return nil
}

// setupSignalHandler sets up signal handling for graceful shutdown
func (s *Server) setupSignalHandler() <-chan os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	return quit
}

// registerRoutes registers all API routes
func (s *Server) registerRoutes() {
	// Health check
	s.router.GET("/health", s.healthCheck)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// GPU endpoints
		gpus := v1.Group("/gpus")
		{
			gpus.GET("", s.listGPUs)
			gpus.GET("/:id/telemetry", s.getGPUTelemetry)
		}
	}

	// API Documentation Endpoints
	// 1. Raw OpenAPI YAML specification
	s.router.GET("/openapi.yaml", func(c *gin.Context) {
		c.File("./api/openapi.yaml")
	})

	// 2. Swagger UI - SPA for API documentation
	s.router.Static("/swagger", "./swagger")
	s.router.GET("/swagger/", func(c *gin.Context) {
		c.File("./swagger/index.html")
	})
}

// healthCheck handles the health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0",
	})
}

// requestLogger is a middleware that logs HTTP requests
func requestLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		statusCode := c.Writer.Status()
		errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String()

		event := logger.Info()
		if statusCode >= 400 {
			event = logger.Error().Str("error", errMsg)
		}

		event = event.Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Int("status", statusCode).
			Str("ip", c.ClientIP()).
			Str("user-agent", c.Request.UserAgent()).
			Dur("latency", latency)

		if query != "" {
			event.Str("query", query)
		}

		event.Msg("Request processed")
	}
}
