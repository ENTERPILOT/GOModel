package auditlog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLStore implements LogStore for PostgreSQL databases.
type PostgreSQLStore struct {
	pool          *pgxpool.Pool
	retentionDays int
	stopCleanup   chan struct{}
}

// NewPostgreSQLStore creates a new PostgreSQL audit log store.
// It creates the audit_logs table if it doesn't exist and starts
// a background cleanup goroutine if retention is configured.
func NewPostgreSQLStore(pool *pgxpool.Pool, retentionDays int) (*PostgreSQLStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}

	ctx := context.Background()

	// Create table with JSONB data field
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL,
			duration_ns BIGINT DEFAULT 0,
			model TEXT,
			provider TEXT,
			status_code INTEGER DEFAULT 0,
			data JSONB
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit_logs table: %w", err)
	}

	// Create indexes for common queries
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_audit_model ON audit_logs(model)",
		"CREATE INDEX IF NOT EXISTS idx_audit_status ON audit_logs(status_code)",
		"CREATE INDEX IF NOT EXISTS idx_audit_provider ON audit_logs(provider)",
		"CREATE INDEX IF NOT EXISTS idx_audit_data_gin ON audit_logs USING GIN (data)",
	}
	for _, idx := range indexes {
		if _, err := pool.Exec(ctx, idx); err != nil {
			slog.Warn("failed to create index", "error", err)
		}
	}

	store := &PostgreSQLStore{
		pool:          pool,
		retentionDays: retentionDays,
		stopCleanup:   make(chan struct{}),
	}

	// Start background cleanup if retention is configured
	if retentionDays > 0 {
		go store.cleanupLoop()
	}

	return store, nil
}

// WriteBatch writes multiple log entries to PostgreSQL using batch insert.
func (s *PostgreSQLStore) WriteBatch(ctx context.Context, entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Use COPY for better performance with large batches
	// But for smaller batches, use a simple INSERT
	if len(entries) < 10 {
		return s.writeBatchSmall(ctx, entries)
	}

	return s.writeBatchLarge(ctx, entries)
}

// writeBatchSmall uses INSERT for small batches
func (s *PostgreSQLStore) writeBatchSmall(ctx context.Context, entries []*LogEntry) error {
	for _, e := range entries {
		var dataJSON []byte
		if e.Data != nil {
			var err error
			dataJSON, err = json.Marshal(e.Data)
			if err != nil {
				slog.Warn("failed to marshal log data", "error", err, "id", e.ID)
				dataJSON = []byte("{}")
			}
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO audit_logs (id, timestamp, duration_ns, model, provider, status_code, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO NOTHING
		`, e.ID, e.Timestamp, e.DurationNs, e.Model, e.Provider, e.StatusCode, dataJSON)

		if err != nil {
			slog.Warn("failed to insert audit log", "error", err, "id", e.ID)
		}
	}
	return nil
}

// writeBatchLarge uses batch insert for larger batches
func (s *PostgreSQLStore) writeBatchLarge(ctx context.Context, entries []*LogEntry) error {
	batch := &pgxpool.Pool{}
	_ = batch // Placeholder - we'll use a simpler approach

	// For larger batches, still use individual inserts but in a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, e := range entries {
		var dataJSON []byte
		if e.Data != nil {
			dataJSON, err = json.Marshal(e.Data)
			if err != nil {
				slog.Warn("failed to marshal log data", "error", err, "id", e.ID)
				dataJSON = []byte("{}")
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (id, timestamp, duration_ns, model, provider, status_code, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO NOTHING
		`, e.ID, e.Timestamp, e.DurationNs, e.Model, e.Provider, e.StatusCode, dataJSON)

		if err != nil {
			slog.Warn("failed to insert audit log in batch", "error", err, "id", e.ID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Flush is a no-op for PostgreSQL as writes are synchronous.
func (s *PostgreSQLStore) Flush(_ context.Context) error {
	return nil
}

// Close stops the cleanup goroutine.
// Note: We don't close the pool here as it's managed by the storage layer.
func (s *PostgreSQLStore) Close() error {
	if s.retentionDays > 0 {
		close(s.stopCleanup)
	}
	return nil
}

// cleanupLoop runs periodically to delete old log entries.
func (s *PostgreSQLStore) cleanupLoop() {
	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup
	s.cleanup()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCleanup:
			return
		}
	}
}

// cleanup deletes log entries older than the retention period.
func (s *PostgreSQLStore) cleanup() {
	if s.retentionDays <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)

	result, err := s.pool.Exec(ctx, "DELETE FROM audit_logs WHERE timestamp < $1", cutoff)
	if err != nil {
		slog.Error("failed to cleanup old audit logs", "error", err)
		return
	}

	if result.RowsAffected() > 0 {
		slog.Info("cleaned up old audit logs", "deleted", result.RowsAffected())
	}
}
