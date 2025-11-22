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
	// Create GPUs table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS gpus (
			id TEXT PRIMARY KEY,
			gpu_index INTEGER NOT NULL,
			gpu_name TEXT NOT NULL,
			memory_total BIGINT NOT NULL,
			power_limit FLOAT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_gpus_id ON gpus (id);
		CREATE INDEX IF NOT EXISTS idx_gpus_gpu_index ON gpus (gpu_index);
	`)
	if err != nil {
		return fmt.Errorf("failed to create gpus table: %w", err)
	}

	// Create telemetry table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gpu_telemetry (
			id SERIAL PRIMARY KEY,
			gpu_id TEXT NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
			temperature FLOAT NOT NULL,
			load FLOAT NOT NULL,
			memory_used BIGINT NOT NULL,
			power_draw FLOAT NOT NULL,
			fan_speed FLOAT NOT NULL,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_gpu_id ON gpu_telemetry (gpu_id);
		CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_timestamp ON gpu_telemetry (timestamp);
		CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_gpu_id_timestamp ON gpu_telemetry (gpu_id, timestamp);
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

	// Prepare statements
	gpuStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO gpus (id, gpu_index, gpu_name, memory_total, power_limit, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE
		SET gpu_index = EXCLUDED.gpu_index,
			gpu_name = EXCLUDED.gpu_name,
			memory_total = EXCLUDED.memory_total,
			power_limit = EXCLUDED.power_limit,
			updated_at = NOW()
		RETURNING id;
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare GPU statement: %w", err)
	}
	defer gpuStmt.Close()

	telemetryStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO gpu_telemetry 
		(gpu_id, temperature, load, memory_used, power_draw, fan_speed, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7);
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare telemetry statement: %w", err)
	}
	defer telemetryStmt.Close()

	// Process each telemetry point in the batch
	for _, t := range batch.Telemetry {
		// Insert or update GPU information
		if _, err := gpuStmt.ExecContext(
			ctx,
			t.ID,
			t.GPUIndex,
			t.GPUName,
			t.MemoryTotal,
			t.PowerLimit,
		); err != nil {
			return fmt.Errorf("failed to insert/update GPU: %w", err)
		}

		// Insert telemetry data
		if _, err := telemetryStmt.ExecContext(
			ctx,
			t.ID,
			t.GPUTemperature,
			t.GPULoad,
			t.MemoryUsed,
			t.PowerDraw,
			t.FanSpeed,
			t.Timestamp,
		); err != nil {
			return fmt.Errorf("failed to insert telemetry: %w", err)
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
			g.id, g.gpu_index, g.gpu_name, 
			t.temperature, t.load, 
			t.memory_used, g.memory_total,
			t.power_draw, g.power_limit,
			t.fan_speed, t.timestamp
		FROM gpu_telemetry t
		JOIN gpus g ON t.gpu_id = g.id
		WHERE g.id = $1
	`

	args := []interface{}{gpuID}
	argIndex := 2

	if !startTime.IsZero() {
		query += fmt.Sprintf(" AND t.timestamp >= $%d", argIndex)
		args = append(args, startTime)
		argIndex++
	}

	if !endTime.IsZero() {
		query += fmt.Sprintf(" AND t.timestamp <= $%d", argIndex)
		args = append(args, endTime)
	}

	query += " ORDER BY t.timestamp ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query telemetry: %w", err)
	}
	defer rows.Close()

	var result []telemetry.GPUTelemetry
	for rows.Next() {
		var t telemetry.GPUTelemetry
		var memoryTotal uint64
		var powerLimit float64

		err := rows.Scan(
			&t.ID,
			&t.GPUIndex,
			&t.GPUName,
			&t.GPUTemperature,
			&t.GPULoad,
			&t.MemoryUsed,
			&memoryTotal,
			&t.PowerDraw,
			&powerLimit,
			&t.FanSpeed,
			&t.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan telemetry row: %w", err)
		}

		t.MemoryTotal = memoryTotal
		t.PowerLimit = powerLimit
		result = append(result, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating telemetry rows: %w", err)
	}

	return result, nil
}

// ListGPUs implements Storage.ListGPUs
func (s *PostgresStorage) ListGPUs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM gpus ORDER BY gpu_index")
	if err != nil {
		return nil, fmt.Errorf("failed to query GPUs: %w", err)
	}
	defer rows.Close()

	var gpus []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan GPU ID: %w", err)
		}
		gpus = append(gpus, id)
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
