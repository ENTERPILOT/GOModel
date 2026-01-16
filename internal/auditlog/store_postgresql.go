package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLStore implements LogStore for PostgreSQL databases.
type PostgreSQLStore struct {
	pool          *pgxpool.Pool
	retentionDays int
	stopCleanup   chan struct{}
	closeOnce     sync.Once
}

// NewPostgreSQLStore creates a new PostgreSQL audit log store.
// It creates the audit_logs table if it doesn't exist and starts
// a background cleanup goroutine if retention is configured.
func NewPostgreSQLStore(pool *pgxpool.Pool, retentionDays int) (*PostgreSQLStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}

	ctx := context.Background()

	// Create table with commonly-filtered fields as columns
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL,
			duration_ns BIGINT DEFAULT 0,
			model TEXT,
			provider TEXT,
			status_code INTEGER DEFAULT 0,
			request_id TEXT,
			client_ip TEXT,
			method TEXT,
			path TEXT,
			stream BOOLEAN DEFAULT FALSE,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0,
			error_type TEXT,
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
		"CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_logs(request_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_client_ip ON audit_logs(client_ip)",
		"CREATE INDEX IF NOT EXISTS idx_audit_path ON audit_logs(path)",
		"CREATE INDEX IF NOT EXISTS idx_audit_error_type ON audit_logs(error_type)",
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

	// For larger batches, use a transaction to ensure atomicity
	// For smaller batches, use individual inserts without transaction overhead
	if len(entries) < 10 {
		return s.writeBatchSmall(ctx, entries)
	}

	return s.writeBatchLarge(ctx, entries)
}

// writeBatchSmall uses INSERT for small batches
func (s *PostgreSQLStore) writeBatchSmall(ctx context.Context, entries []*LogEntry) error {
	var errs []error

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
			INSERT INTO audit_logs (id, timestamp, duration_ns, model, provider, status_code,
				request_id, client_ip, method, path, stream,
				prompt_tokens, completion_tokens, total_tokens, error_type, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			ON CONFLICT (id) DO NOTHING
		`, e.ID, e.Timestamp, e.DurationNs, e.Model, e.Provider, e.StatusCode,
			e.RequestID, e.ClientIP, e.Method, e.Path, e.Stream,
			e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.ErrorType, dataJSON)

		if err != nil {
			slog.Warn("failed to insert audit log", "error", err, "id", e.ID)
			errs = append(errs, fmt.Errorf("insert %s: %w", e.ID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to insert %d of %d audit logs: %w", len(errs), len(entries), errors.Join(errs...))
	}
	return nil
}

// writeBatchLarge uses batch insert for larger batches
func (s *PostgreSQLStore) writeBatchLarge(ctx context.Context, entries []*LogEntry) error {
	// For larger batches, use individual inserts in a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var errs []error

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
			INSERT INTO audit_logs (id, timestamp, duration_ns, model, provider, status_code,
				request_id, client_ip, method, path, stream,
				prompt_tokens, completion_tokens, total_tokens, error_type, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			ON CONFLICT (id) DO NOTHING
		`, e.ID, e.Timestamp, e.DurationNs, e.Model, e.Provider, e.StatusCode,
			e.RequestID, e.ClientIP, e.Method, e.Path, e.Stream,
			e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.ErrorType, dataJSON)

		if err != nil {
			slog.Warn("failed to insert audit log in batch", "error", err, "id", e.ID)
			errs = append(errs, fmt.Errorf("insert %s: %w", e.ID, err))
		}
	}

	// If any inserts failed, rollback and return error (consistent with writeBatchSmall)
	if len(errs) > 0 {
		return fmt.Errorf("failed to insert %d of %d audit logs: %w", len(errs), len(entries), errors.Join(errs...))
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
// Safe to call multiple times.
func (s *PostgreSQLStore) Close() error {
	if s.retentionDays > 0 && s.stopCleanup != nil {
		s.closeOnce.Do(func() {
			close(s.stopCleanup)
		})
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
