package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	Server       ServerConfig       `mapstructure:"server"`
	MessageQueue MessageQueueConfig `mapstructure:"message_queue"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Log          LogConfig          `mapstructure:"log"`
	Telemetry    TelemetryConfig    `mapstructure:"telemetry"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Collector    CollectorConfig    `mapstructure:"collector"`
}

type CollectorConfig struct {
	BatchSize         int `mapstructure:"batch_size"`
	MaxInFlight       int `mapstructure:"max_in_flight"`
	AckTimeoutSeconds int `mapstructure:"ack_timeout_seconds"`
	WorkerCount       int `mapstructure:"worker_count"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
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
	Address string `mapstructure:"address"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Type     string         `mapstructure:"type"`
	Postgres PostgresConfig `mapstructure:"postgres"`
}

// PostgresConfig holds PostgreSQL configuration
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
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
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set the base name of the config file (without extension)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")
	v.AddConfigPath("../../configs")

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Read environment variables with the prefix "GPUTEL_"
	v.SetEnvPrefix("GPUTEL")
	v.AutomaticEnv()

	// Unmarshal the config into the Config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
