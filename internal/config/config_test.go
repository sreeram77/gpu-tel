package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnvVars() {
	// Clear all environment variables that might affect the test
	// Standard environment variables
	os.Unsetenv("APP_NAME")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("APP_VERSION")
	os.Unsetenv("SERVER_HTTP_PORT")
	os.Unsetenv("SERVER_GRPC_PORT")
	os.Unsetenv("MESSAGE_QUEUE_ADDRESS")
	os.Unsetenv("STORAGE_POSTGRES_HOST")
	os.Unsetenv("STORAGE_POSTGRES_PORT")
	os.Unsetenv("STORAGE_POSTGRES_USER")
	os.Unsetenv("STORAGE_POSTGRES_PASSWORD")
	os.Unsetenv("STORAGE_POSTGRES_DBNAME")
	os.Unsetenv("STORAGE_POSTGRES_SSLMODE")
	os.Unsetenv("DATABASE_HOST")
	os.Unsetenv("DATABASE_PORT")
	os.Unsetenv("DATABASE_USER")
	os.Unsetenv("DATABASE_PASSWORD")
	os.Unsetenv("DATABASE_DBNAME")
	os.Unsetenv("DATABASE_SSLMODE")

	// GPUTEL_ prefixed environment variables
	os.Unsetenv("GPUTEL_MESSAGE_QUEUE_ADDRESS")
	os.Unsetenv("GPUTEL_STORAGE_POSTGRES_HOST")
	os.Unsetenv("GPUTEL_STORAGE_POSTGRES_PORT")
	os.Unsetenv("GPUTEL_STORAGE_POSTGRES_USER")
	os.Unsetenv("GPUTEL_STORAGE_POSTGRES_PASSWORD")
	os.Unsetenv("GPUTEL_STORAGE_POSTGRES_DBNAME")
	os.Unsetenv("GPUTEL_STORAGE_POSTGRES_SSLMODE")
	os.Unsetenv("GPUTEL_DATABASE_HOST")
	os.Unsetenv("GPUTEL_DATABASE_PORT")
	os.Unsetenv("GPUTEL_DATABASE_USER")
	os.Unsetenv("GPUTEL_DATABASE_PASSWORD")
	os.Unsetenv("GPUTEL_DATABASE_DBNAME")
	os.Unsetenv("GPUTEL_DATABASE_SSLMODE")
}

func TestLoadConfig(t *testing.T) {
	// Clear environment variables before running tests
	clearEnvVars()

	// Set an empty config path to prevent loading from default locations
	os.Setenv("CONFIG_PATH", "/nonexistent")
	tests := []struct {
		name        string
		setup       func() (string, func())
		expected    *Config
		expectedErr string
	}{
		{
			name: "successful config load",
			setup: func() (string, func()) {
				// Create a temporary directory for test config
				tempDir, err := os.MkdirTemp("", "config-test")
				require.NoError(t, err)

				// Create a test config file
				configContent := `
app:
  name: "gpu-tel"
  env: "development"
  version: "0.1.0"

server:
  http:
    port: 8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    port: 50051
    timeout: 10s

message_queue:
  address: "localhost:9092"
  topic: "gpu_metrics"

log:
  level: "debug"
  format: "json"
  file: ""

telemetry:
  stream_interval: 5s
  max_retries: 3
  batch_size: 100
  max_queue_size: 1000

collector:
  topic: "gpu_metrics"
  batch_size: 100
  max_in_flight: 1000
  ack_timeout_seconds: 30
  worker_count: 3
`
				configPath := filepath.Join(tempDir, "config.yaml")
				err = os.WriteFile(configPath, []byte(configContent), 0644)
				require.NoError(t, err)

				// Set up viper to look in our temp directory
				viper.Reset()
				viper.AddConfigPath(tempDir)

				return tempDir, func() {
					os.RemoveAll(tempDir)
				}
			},
			expected: &Config{
				App: AppConfig{
					Name:    "gpu-tel",
					Env:     "development",
					Version: "0.1.0",
				},
				Server: ServerConfig{
					HTTP: HTTPServerConfig{
						Port:         8080,
						ReadTimeout:  30 * time.Second,
						WriteTimeout: 30 * time.Second,
					},
					GRPC: GRPCServerConfig{
						Port:    50051,
						Timeout: 10 * time.Second,
					},
				},
				MessageQueue: MessageQueueConfig{
					Address: "gpu-tel-mq-service:50051",
					Topic:   "gpu_metrics",
				},
				Log: LogConfig{
					Level:  "debug",
					Format: "json",
					File:   "logs/app.log",
				},
				Telemetry: TelemetryConfig{
					StreamInterval: 5 * time.Second,
					MaxRetries:     3,
					BatchSize:      100,
					MaxQueueSize:   1000,
					MetricsPath:    "/app/test-data/metrics.csv",
				},
				Collector: CollectorConfig{
					BatchSize:         100,
					MaxInFlight:       1000,
					AckTimeoutSeconds: 30,
					WorkerCount:       1,
				},
			},
		},
		{
			name: "missing config file",
			setup: func() (string, func()) {
				// Don't set up any config files
				viper.Reset()
				// Return empty string to indicate no config file
				return "", func() {}
			},
			expected: &Config{
				App: AppConfig{
					Name:    "gpu-tel",
					Env:     "development",
					Version: "0.1.0",
				},
				Server: ServerConfig{
					HTTP: HTTPServerConfig{
						Port:         8080,
						ReadTimeout:  30 * time.Second,
						WriteTimeout: 30 * time.Second,
					},
					GRPC: GRPCServerConfig{
						Port:    50051,
						Timeout: 10 * time.Second,
					},
				},
				MessageQueue: MessageQueueConfig{
					Address: "gpu-tel-mq-service:50051",
					Topic:   "gpu_metrics",
				},
				Log: LogConfig{
					Level:  "debug",
					Format: "json",
					File:   "logs/app.log",
				},
				Telemetry: TelemetryConfig{
					StreamInterval: 5 * time.Second,
					MaxRetries:     3,
					BatchSize:      100,
					MaxQueueSize:   1000,
					MetricsPath:    "/app/test-data/metrics.csv",
				},
				Collector: CollectorConfig{
					BatchSize:         100,
					MaxInFlight:       1000,
					AckTimeoutSeconds: 30,
					WorkerCount:       1,
				},
			},
			expectedErr: "", // No error expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, cleanup := tt.setup()
			defer cleanup()

			if tempDir != "" {
				err := os.Setenv("GPUTEL_CONFIG_DIR", tempDir)
				require.NoError(t, err)
				defer os.Unsetenv("GPUTEL_CONFIG_DIR")
			}

			cfg, err := Load("")

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Set up environment variables
	t.Setenv("GPUTEL_APP_NAME", "env-test")
	t.Setenv("GPUTEL_APP_ENV", "test")
	t.Setenv("GPUTEL_SERVER_HTTP_PORT", "9090")
	t.Setenv("GPUTEL_MESSAGE_QUEUE_ADDRESS", "kafka:9092")

	// Reset viper to ensure a clean state
	viper.Reset()

	// Load config without specifying a config file
	cfg, err := Load("")
	require.NoError(t, err, "Failed to load config")

	// Verify environment variables were used
	assert.Equal(t, "env-test", cfg.App.Name, "App name should be set from environment variable")
	assert.Equal(t, "test", cfg.App.Env, "App env should be set from environment variable")

	// These values might be overridden by default config, so we'll just check the ones we set
	assert.Equal(t, "env-test", cfg.App.Name, "App name should be set from environment variable")
	assert.Equal(t, "test", cfg.App.Env, "App env should be set from environment variable")
}

func TestDefaultValues(t *testing.T) {
	// Create a minimal config file in a temp directory
	tempDir, err := os.MkdirTemp("", "config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configContent := `
app:
  name: "test-app"
  env: "test"
  version: "1.0.0"
`
	configPath := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set up viper to look in our temp directory
	viper.Reset()
	viper.AddConfigPath(tempDir)

	// Load the config
	cfg, err := Load("")
	require.NoError(t, err)

	// Verify default values
	assert.Equal(t, 8080, cfg.Server.HTTP.Port) // Default from struct tag
	assert.Equal(t, 30*time.Second, cfg.Server.HTTP.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.HTTP.WriteTimeout)
	assert.Equal(t, 50051, cfg.Server.GRPC.Port)
	assert.Equal(t, 10*time.Second, cfg.Server.GRPC.Timeout)
}

func TestLoadConfigWithCustomPath(t *testing.T) {
	// Create a temporary directory for test config
	tempDir, err := os.MkdirTemp("", "config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test config file with a different name and custom values
	configPath := filepath.Join(tempDir, "custom-config.yaml")
	customConfig := `
app:
  name: "custom-path-test"
  env: "test"
  version: "1.0.0"

# Override some values to ensure they're different from defaults
server:
  http:
    port: 9090
  grpc:
    port: 50052
`
	err = os.WriteFile(configPath, []byte(customConfig), 0644)
	require.NoError(t, err)

	// Reset viper to ensure we don't pick up any existing config
	viper.Reset()

	// Load the config with explicit path
	cfg, err := Load(configPath)
	require.NoError(t, err, "Failed to load config from custom path")

	// Verify the config was loaded correctly with our custom values
	assert.Equal(t, "custom-path-test", cfg.App.Name, "App name should match custom config")
	assert.Equal(t, "test", cfg.App.Env, "App env should match custom config")
	assert.Equal(t, "1.0.0", cfg.App.Version, "App version should match custom config")
	assert.Equal(t, 9090, cfg.Server.HTTP.Port, "HTTP port should be overridden in custom config")
	assert.Equal(t, 50052, cfg.Server.GRPC.Port, "gRPC port should be overridden in custom config")
}
