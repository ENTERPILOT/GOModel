package requestflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLStore persists request flow data in PostgreSQL.
type PostgreSQLStore struct {
	pool          *pgxpool.Pool
	retentionDays int
	stopCleanup   chan struct{}
	closeOnce     sync.Once
}

// NewPostgreSQLStore creates a PostgreSQL-backed request flow store.
func NewPostgreSQLStore(pool *pgxpool.Pool, retentionDays int) (*PostgreSQLStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}
	ctx := context.Background()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS flow_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			priority INTEGER NOT NULL DEFAULT 0,
			model TEXT,
			api_key_hash TEXT,
			team TEXT,
			user_id TEXT,
			source TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			data JSONB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_definitions_enabled ON flow_definitions(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_definitions_model ON flow_definitions(model)`,
		`CREATE TABLE IF NOT EXISTS flow_executions (
			id TEXT PRIMARY KEY,
			request_id TEXT,
			timestamp TIMESTAMPTZ NOT NULL,
			model TEXT,
			provider TEXT,
			endpoint TEXT,
			plan_name TEXT,
			status TEXT,
			retries_made INTEGER NOT NULL DEFAULT 0,
			failover_used BOOLEAN NOT NULL DEFAULT FALSE,
			data JSONB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_timestamp ON flow_executions(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_request_id ON flow_executions(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_model ON flow_executions(model)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_executions_status ON flow_executions(status)`,
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return nil, fmt.Errorf("initialize request flow postgres store: %w", err)
		}
	}
	store := &PostgreSQLStore{pool: pool, retentionDays: retentionDays, stopCleanup: make(chan struct{})}
	if retentionDays > 0 {
		go runCleanupLoop(store.stopCleanup, store.cleanup)
	}
	return store, nil
}

func (s *PostgreSQLStore) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	rows, err := s.pool.Query(ctx, `SELECT data FROM flow_definitions ORDER BY priority DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	defs := make([]*Definition, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var def Definition
		if err := json.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("decode definition: %w", err)
		}
		defs = append(defs, &def)
	}
	return defs, rows.Err()
}

func (s *PostgreSQLStore) SaveDefinition(ctx context.Context, def *Definition) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO flow_definitions (
			id, name, enabled, priority, model, api_key_hash, team, user_id, source, created_at, updated_at, data
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
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
	`, def.ID, def.Name, def.Enabled, def.Priority, nullableString(def.Match.Model), nullableString(def.Match.APIKeyHash), nullableString(def.Match.Team), nullableString(def.Match.User), def.Source, def.CreatedAt.UTC(), def.UpdatedAt.UTC(), marshalDefinition(def))
	return err
}

func (s *PostgreSQLStore) DeleteDefinition(ctx context.Context, id string) error {
	res, err := s.pool.Exec(ctx, `DELETE FROM flow_definitions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgreSQLStore) WriteExecutionBatch(ctx context.Context, entries []*Execution) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO flow_executions (
				id, request_id, timestamp, model, provider, endpoint, plan_name, status, retries_made, failover_used, data
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (id) DO NOTHING
		`, entry.ID, entry.RequestID, entry.Timestamp.UTC(), entry.Model, entry.Provider, entry.Endpoint, entry.PlanName, entry.Status, entry.RetriesMade, entry.FailoverUsed, marshalExecution(entry)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgreSQLStore) ListExecutions(ctx context.Context, params ExecutionQueryParams) (*ExecutionLogResult, error) {
	conditions := []string{"1=1"}
	args := make([]interface{}, 0)
	argPos := 1
	if params.RequestID != "" {
		conditions = append(conditions, fmt.Sprintf("request_id = $%d", argPos))
		args = append(args, params.RequestID)
		argPos++
	}
	if params.Model != "" {
		conditions = append(conditions, fmt.Sprintf("model = $%d", argPos))
		args = append(args, params.Model)
		argPos++
	}
	if params.Search != "" {
		like := "%" + params.Search + "%"
		conditions = append(conditions, fmt.Sprintf("(request_id ILIKE $%d OR model ILIKE $%d OR provider ILIKE $%d OR plan_name ILIKE $%d OR status ILIKE $%d)", argPos, argPos, argPos, argPos, argPos))
		args = append(args, like)
		argPos++
	}
	where := strings.Join(conditions, " AND ")
	countQuery := `SELECT COUNT(*) FROM flow_executions WHERE ` + where
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}
	dataArgs := append([]interface{}{}, args...)
	dataArgs = append(dataArgs, params.Limit, params.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT data FROM flow_executions
		WHERE %s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, where, argPos, argPos+1), dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]*Execution, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var entry Execution
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("decode execution: %w", err)
		}
		entries = append(entries, &entry)
	}
	return &ExecutionLogResult{Entries: entries, Total: total, Limit: params.Limit, Offset: params.Offset}, rows.Err()
}

func (s *PostgreSQLStore) Flush(_ context.Context) error { return nil }

func (s *PostgreSQLStore) Close() error {
	if s.retentionDays > 0 {
		s.closeOnce.Do(func() { close(s.stopCleanup) })
	}
	return nil
}

func (s *PostgreSQLStore) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, _ = s.pool.Exec(ctx, `DELETE FROM flow_executions WHERE timestamp < $1`, time.Now().AddDate(0, 0, -s.retentionDays).UTC())
}
