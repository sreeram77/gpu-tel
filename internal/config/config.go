package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App         AppConfig         `mapstructure:"app"`
	Server      ServerConfig      `mapstructure:"server"`
	MessageQueue MessageQueueConfig `mapstructure:"message_queue"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Log         LogConfig         `mapstructure:"log"`
	Telemetry   TelemetryConfig   `mapstructure:"telemetry"`
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
	Port        int           `mapstructure:"port"`
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// GRPCServerConfig holds gRPC server configuration
type GRPCServerConfig struct {
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// MessageQueueConfig holds message queue configuration
type MessageQueueConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	VHost           string `mapstructure:"vhost"`
	Exchange        string `mapstructure:"exchange"`
	Queue           string `mapstructure:"queue"`
	Consumer        string `mapstructure:"consumer"`
	PrefetchCount   int    `mapstructure:"prefetch_count"`
	PrefetchSize    int    `mapstructure:"prefetch_size"`
	RequeueOnError  bool   `mapstructure:"requeue_on_error"`
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

// GetDSN returns the PostgreSQL DSN string
func (p *PostgresConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode)
}

// GetMessageQueueURL returns the message queue connection URL
func (m *MessageQueueConfig) GetMessageQueueURL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		m.Username, m.Password, m.Host, m.Port, m.VHost)
}
