package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/sreeram77/gpu-tel/internal/telemetry"
)

// PostgresConfig holds PostgreSQL configuration
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// PostgresStorage implements Storage interface for PostgreSQL
type PostgresStorage struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(logger zerolog.Logger, cfg *PostgresConfig) (*PostgresStorage, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &PostgresStorage{
		db:     db,
		logger: logger,
	}, nil
}

// createTables creates the necessary tables if they don't exist
func createTables(db *sql.DB) error {
	// Create telemetry table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS gpu_telemetry (
			id BIGSERIAL PRIMARY KEY,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			metric_name TEXT NOT NULL,
			gpu_id TEXT NOT NULL,
			device TEXT NOT NULL,
			uuid TEXT NOT NULL,
			model_name TEXT NOT NULL,
			hostname TEXT NOT NULL,
			container TEXT,
			pod TEXT,
			namespace TEXT,
			value DOUBLE PRECISION NOT NULL,
			labels_raw TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		-- Indexes for common query patterns
		CREATE INDEX IF NOT EXISTS idx_telemetry_timestamp ON gpu_telemetry (timestamp);
		CREATE INDEX IF NOT EXISTS idx_telemetry_gpu_id ON gpu_telemetry (gpu_id);
		CREATE INDEX IF NOT EXISTS idx_telemetry_uuid ON gpu_telemetry (uuid);
		CREATE INDEX IF NOT EXISTS idx_telemetry_metric_name ON gpu_telemetry (metric_name);
		CREATE INDEX IF NOT EXISTS idx_telemetry_hostname ON gpu_telemetry (hostname);
		CREATE INDEX IF NOT EXISTS idx_telemetry_composite ON gpu_telemetry (gpu_id, metric_name, timestamp);
	`)
	if err != nil {
		return fmt.Errorf("failed to create gpu_telemetry table: %w", err)
	}

	return nil
}

// Store implements Storage.Store
func (s *PostgresStorage) Store(ctx context.Context, batch telemetry.TelemetryBatch) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare telemetry statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO gpu_telemetry (
			timestamp, metric_name, gpu_id, device, uuid, 
			model_name, hostname, container, pod, namespace, 
			value, labels_raw
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare telemetry statement: %w", err)
	}
	defer stmt.Close()

	// Process each telemetry point in the batch
	for _, t := range batch.Telemetry {
		_, err := stmt.ExecContext(
			ctx,
			t.Timestamp,
			t.MetricName,
			t.GPUIndex, // Stored as gpu_id in the database
			t.Device,
			t.UUID,
			t.ModelName,
			t.Hostname,
			t.Container,
			t.Pod,
			t.Namespace,
			t.Value,
			t.LabelsRaw,
		)
		if err != nil {
			s.logger.Error().Err(err).
				Str("uuid", t.UUID).
				Str("metric", t.MetricName).
				Msg("failed to insert telemetry")
			continue
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetGPUTelemetry implements Storage.GetGPUTelemetry
func (s *PostgresStorage) GetGPUTelemetry(
	ctx context.Context,
	gpuID string,
	startTime, endTime time.Time,
) ([]telemetry.GPUTelemetry, error) {
	query := `
		SELECT 
			timestamp, metric_name, gpu_id, device, uuid,
			model_name, hostname, container, pod, namespace,
			value, labels_raw
		FROM gpu_telemetry
		WHERE uuid = $1
	`

	args := []interface{}{gpuID}
	argIndex := 2

	if !startTime.IsZero() {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, startTime)
		argIndex++
	}

	if !endTime.IsZero() {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query telemetry: %w", err)
	}
	defer rows.Close()

	var result []telemetry.GPUTelemetry

	for rows.Next() {
		var t telemetry.GPUTelemetry
		err := rows.Scan(
			&t.Timestamp,
			&t.MetricName,
			&t.GPUIndex,
			&t.Device,
			&t.UUID,
			&t.ModelName,
			&t.Hostname,
			&t.Container,
			&t.Pod,
			&t.Namespace,
			&t.Value,
			&t.LabelsRaw,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan telemetry row: %w", err)
		}
		result = append(result, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating telemetry rows: %w", err)
	}

	return result, nil
}

// ListGPUs implements Storage.ListGPUs
func (s *PostgresStorage) ListGPUs(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT uuid, gpu_id, hostname, model_name 
		FROM gpu_telemetry 
		ORDER BY hostname, gpu_id
	`
	
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query GPUs: %w", err)
	}
	defer rows.Close()

	var gpus []string
	for rows.Next() {
		var uuid, gpuID, hostname, modelName string
		if err := rows.Scan(&uuid, &gpuID, &hostname, &modelName); err != nil {
			return nil, fmt.Errorf("failed to scan GPU row: %w", err)
		}
		// Format: "{hostname} - GPU {id} ({model}) - {uuid}"
		gpus = append(gpus, fmt.Sprintf("%s - GPU %s (%s) - %s", 
			hostname, gpuID, modelName, uuid))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating GPU rows: %w", err)
	}

	return gpus, nil
}

// Close implements Storage.Close
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
