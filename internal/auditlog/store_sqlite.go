package auditlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SQLiteStore implements LogStore for SQLite databases.
type SQLiteStore struct {
	db            *sql.DB
	retentionDays int
	stopCleanup   chan struct{}
}

// NewSQLiteStore creates a new SQLite audit log store.
// It creates the audit_logs table if it doesn't exist and starts
// a background cleanup goroutine if retention is configured.
func NewSQLiteStore(db *sql.DB, retentionDays int) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Create table with JSON data field
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			duration_ns INTEGER DEFAULT 0,
			model TEXT,
			provider TEXT,
			status_code INTEGER DEFAULT 0,
			data JSON
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
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			slog.Warn("failed to create index", "error", err)
		}
	}

	store := &SQLiteStore{
		db:            db,
		retentionDays: retentionDays,
		stopCleanup:   make(chan struct{}),
	}

	// Start background cleanup if retention is configured
	if retentionDays > 0 {
		go store.cleanupLoop()
	}

	return store, nil
}

// WriteBatch writes multiple log entries to SQLite using batch insert.
func (s *SQLiteStore) WriteBatch(ctx context.Context, entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Build batch insert query
	placeholders := make([]string, len(entries))
	values := make([]interface{}, 0, len(entries)*7)

	for i, e := range entries {
		placeholders[i] = "(?, ?, ?, ?, ?, ?, ?)"

		// Marshal data to JSON
		var dataJSON []byte
		if e.Data != nil {
			var err error
			dataJSON, err = json.Marshal(e.Data)
			if err != nil {
				slog.Warn("failed to marshal log data", "error", err, "id", e.ID)
				dataJSON = []byte("{}")
			}
		}

		values = append(values,
			e.ID,
			e.Timestamp.UTC().Format(time.RFC3339Nano),
			e.DurationNs,
			e.Model,
			e.Provider,
			e.StatusCode,
			string(dataJSON),
		)
	}

	query := `INSERT OR IGNORE INTO audit_logs (id, timestamp, duration_ns, model, provider, status_code, data) VALUES ` +
		strings.Join(placeholders, ",")

	_, err := s.db.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to insert audit logs: %w", err)
	}

	return nil
}

// Flush is a no-op for SQLite as writes are synchronous.
func (s *SQLiteStore) Flush(_ context.Context) error {
	return nil
}

// Close stops the cleanup goroutine.
// Note: We don't close the DB here as it's managed by the storage layer.
func (s *SQLiteStore) Close() error {
	if s.retentionDays > 0 {
		close(s.stopCleanup)
	}
	return nil
}

// cleanupLoop runs periodically to delete old log entries.
func (s *SQLiteStore) cleanupLoop() {
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
func (s *SQLiteStore) cleanup() {
	if s.retentionDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays).UTC().Format(time.RFC3339)

	result, err := s.db.Exec("DELETE FROM audit_logs WHERE timestamp < ?", cutoff)
	if err != nil {
		slog.Error("failed to cleanup old audit logs", "error", err)
		return
	}

	if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
		slog.Info("cleaned up old audit logs", "deleted", rowsAffected)
	}
}
