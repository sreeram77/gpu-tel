package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	Server       ServerConfig       `mapstructure:"server"`
	MessageQueue MessageQueueConfig `mapstructure:"message_queue"`
	Log          LogConfig          `mapstructure:"log"`
	Telemetry    TelemetryConfig    `mapstructure:"telemetry"`
	Collector    CollectorConfig    `mapstructure:"collector"`
}

type CollectorConfig struct {
	BatchSize         int `mapstructure:"batch_size"`
	MaxInFlight       int `mapstructure:"max_in_flight"`
	AckTimeoutSeconds int `mapstructure:"ack_timeout_seconds"`
	WorkerCount       int `mapstructure:"worker_count"`
}

// AppConfig holds application configuration
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"`
	Version string `mapstructure:"version"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	HTTP HTTPServerConfig `mapstructure:"http"`
	GRPC GRPCServerConfig `mapstructure:"grpc"`
}

// HTTPServerConfig holds HTTP server configuration
type HTTPServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// GRPCServerConfig holds gRPC server configuration
type GRPCServerConfig struct {
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// MessageQueueConfig holds message queue configuration
type MessageQueueConfig struct {
	Address       string `mapstructure:"address"`
	Topic         string `mapstructure:"topic"`
	ConsumerGroup string `mapstructure:"consumer_group"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	File   string `mapstructure:"file"`
}

// TelemetryConfig holds telemetry configuration
type TelemetryConfig struct {
	StreamInterval time.Duration `mapstructure:"stream_interval"`
	MaxRetries     int           `mapstructure:"max_retries"`
	BatchSize      int           `mapstructure:"batch_size"`
	MaxQueueSize   int           `mapstructure:"max_queue_size"`
	MetricsPath    string        `mapstructure:"metrics_path"`
}

// Load loads configuration from file and environment variables.
// If configPath is provided, it will be used to load the configuration from that specific file.
// Otherwise, it will look for config.yaml in standard locations.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set up environment variables
	v.SetEnvPrefix("GPUTEL")
	v.AutomaticEnv()

	// Enable environment variable binding
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set default values
	setDefaults(v)

	// If a config path is provided, use that
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Otherwise look for config.yaml in standard locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
		v.AddConfigPath("../../configs")
	}

	// Read the config file if it exists
	err := v.ReadInConfig()
	if err != nil {
		// If we have a specific config path and it doesn't exist, return error
		if configPath != "" {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// For default config paths, it's okay if no config file is found
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal the config into the Config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for the configuration
func setDefaults(v *viper.Viper) {
	// Set default values for the config
	v.SetDefault("app.name", "gpu-tel")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.version", "0.1.0")

	// Server defaults
	v.SetDefault("server.http.port", 8080)
	v.SetDefault("server.http.read_timeout", 30*time.Second)
	v.SetDefault("server.http.write_timeout", 30*time.Second)
	v.SetDefault("server.grpc.port", 50051)
	v.SetDefault("server.grpc.timeout", 10*time.Second)

	// Message queue defaults
	v.SetDefault("message_queue.address", "localhost:50051")
	v.SetDefault("message_queue.topic", "gpu_metrics")
	v.SetDefault("message_queue.consumer_group", "")

	// Log defaults
	v.SetDefault("log.level", "debug")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.file", "logs/app.log")

	// Telemetry defaults
	v.SetDefault("telemetry.stream_interval", 5*time.Second)
	v.SetDefault("telemetry.max_retries", 3)
	v.SetDefault("telemetry.batch_size", 100)
	v.SetDefault("telemetry.max_queue_size", 1000)
	v.SetDefault("telemetry.metrics_path", "/app/test-data/metrics.csv")

	// Collector defaults
	v.SetDefault("collector.batch_size", 100)
	v.SetDefault("collector.max_in_flight", 1000)
	v.SetDefault("collector.ack_timeout_seconds", 30)
	v.SetDefault("collector.worker_count", 1)
}
