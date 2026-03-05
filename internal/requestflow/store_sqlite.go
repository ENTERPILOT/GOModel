package requestflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SQLiteStore persists request flow data in SQLite.
type SQLiteStore struct {
	db            *sql.DB
	retentionDays int
	stopCleanup   chan struct{}
	closeOnce     sync.Once
}

// NewSQLiteStore creates a SQLite-backed request flow store.
func NewSQLiteStore(db *sql.DB, retentionDays int) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS flow_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 0,
			model TEXT,
			api_key_hash TEXT,
			team TEXT,
			user_id TEXT,
			source TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_definitions_enabled ON flow_definitions(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_definitions_model ON flow_definitions(model)`,
		`CREATE TABLE IF NOT EXISTS flow_executions (
			id TEXT PRIMARY KEY,
			request_id TEXT,
			timestamp DATETIME NOT NULL,
			model TEXT,
			provider TEXT,
			endpoint TEXT,
			plan_name TEXT,
			status TEXT,
			retries_made INTEGER NOT NULL DEFAULT 0,
			failover_used INTEGER NOT NULL DEFAULT 0,
			data JSON NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_timestamp ON flow_executions(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_request_id ON flow_executions(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_model ON flow_executions(model)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_status ON flow_executions(status)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("initialize request flow sqlite store: %w", err)
		}
	}
	store := &SQLiteStore{db: db, retentionDays: retentionDays, stopCleanup: make(chan struct{})}
	if retentionDays > 0 {
		go runCleanupLoop(store.stopCleanup, store.cleanup)
	}
	return store, nil
}

// ListDefinitions returns all stored definitions.
func (s *SQLiteStore) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM flow_definitions ORDER BY priority DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	defs := make([]*Definition, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var def Definition
		if err := json.Unmarshal([]byte(data), &def); err != nil {
			return nil, fmt.Errorf("decode definition: %w", err)
		}
		defs = append(defs, &def)
	}
	return defs, rows.Err()
}

// SaveDefinition upserts a definition.
func (s *SQLiteStore) SaveDefinition(ctx context.Context, def *Definition) error {
	if def == nil {
		return fmt.Errorf("definition is required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO flow_definitions (
			id, name, enabled, priority, model, api_key_hash, team, user_id, source, created_at, updated_at, data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			enabled = excluded.enabled,
			priority = excluded.priority,
			model = excluded.model,
			api_key_hash = excluded.api_key_hash,
			team = excluded.team,
			user_id = excluded.user_id,
			source = excluded.source,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			data = excluded.data
	`,
		def.ID,
		def.Name,
		boolToInt(def.Enabled),
		def.Priority,
		nullableString(def.Match.Model),
		nullableString(def.Match.APIKeyHash),
		nullableString(def.Match.Team),
		nullableString(def.Match.User),
		def.Source,
		def.CreatedAt.UTC().Format(time.RFC3339Nano),
		def.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(marshalDefinition(def)),
	)
	return err
}

// DeleteDefinition removes a definition.
func (s *SQLiteStore) DeleteDefinition(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM flow_definitions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

// WriteExecutionBatch inserts execution records.
func (s *SQLiteStore) WriteExecutionBatch(ctx context.Context, entries []*Execution) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO flow_executions (
			id, request_id, timestamp, model, provider, endpoint, plan_name, status, retries_made, failover_used, data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if _, err := stmt.ExecContext(ctx,
			entry.ID,
			entry.RequestID,
			entry.Timestamp.UTC().Format(time.RFC3339Nano),
			entry.Model,
			entry.Provider,
			entry.Endpoint,
			entry.PlanName,
			entry.Status,
			entry.RetriesMade,
			boolToInt(entry.FailoverUsed),
			string(marshalExecution(entry)),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListExecutions returns paginated execution history.
func (s *SQLiteStore) ListExecutions(ctx context.Context, params ExecutionQueryParams) (*ExecutionLogResult, error) {
	conditions := []string{"1=1"}
	args := make([]interface{}, 0)
	if params.RequestID != "" {
		conditions = append(conditions, "request_id = ?")
		args = append(args, params.RequestID)
	}
	if params.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, params.Model)
	}
	if params.Search != "" {
		like := "%" + params.Search + "%"
		conditions = append(conditions, "(request_id LIKE ? OR model LIKE ? OR provider LIKE ? OR plan_name LIKE ? OR status LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	where := strings.Join(conditions, " AND ")

	countQuery := `SELECT COUNT(*) FROM flow_executions WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	dataArgs := append([]interface{}{}, args...)
	dataArgs = append(dataArgs, params.Limit, params.Offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT data FROM flow_executions
		WHERE `+where+`
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]*Execution, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var entry Execution
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			return nil, fmt.Errorf("decode execution: %w", err)
		}
		entries = append(entries, &entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ExecutionLogResult{Entries: entries, Total: total, Limit: params.Limit, Offset: params.Offset}, nil
}

// Flush is synchronous for SQLite.
func (s *SQLiteStore) Flush(_ context.Context) error { return nil }

// Close stops cleanup routines.
func (s *SQLiteStore) Close() error {
	if s.retentionDays > 0 {
		s.closeOnce.Do(func() { close(s.stopCleanup) })
	}
	return nil
}

func (s *SQLiteStore) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays).UTC().Format(time.RFC3339Nano)
	_, _ = s.db.Exec(`DELETE FROM flow_executions WHERE timestamp < ?`, cutoff)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableString(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
