package guardrails

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLStore stores guardrail definitions in PostgreSQL.
type PostgreSQLStore struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLStore creates the guardrail table and indexes if needed.
func NewPostgreSQLStore(ctx context.Context, pool *pgxpool.Pool) (*PostgreSQLStore, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS guardrail_definitions (
			name TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			user_path TEXT,
			config JSONB NOT NULL,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`ALTER TABLE guardrail_definitions ADD COLUMN IF NOT EXISTS user_path TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_guardrail_definitions_type ON guardrail_definitions(type)`,
		`CREATE INDEX IF NOT EXISTS idx_guardrail_definitions_updated_at ON guardrail_definitions(updated_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			return nil, fmt.Errorf("initialize guardrail definitions table: %w", err)
		}
	}

	return &PostgreSQLStore{pool: pool}, nil
}

func (s *PostgreSQLStore) List(ctx context.Context) ([]Definition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT name, type, description, user_path, config, created_at, updated_at
		FROM guardrail_definitions
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list guardrails: %w", err)
	}
	defer rows.Close()
	return collectDefinitions(rows, scanPostgreSQLDefinition)
}

func (s *PostgreSQLStore) Get(ctx context.Context, name string) (*Definition, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT name, type, description, user_path, config, created_at, updated_at
		FROM guardrail_definitions
		WHERE name = $1
	`, normalizeDefinitionName(name))
	definition, err := scanPostgreSQLDefinition(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &definition, nil
}

func (s *PostgreSQLStore) Upsert(ctx context.Context, definition Definition) error {
	definition, err := normalizeDefinition(definition)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Unix()
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = time.Unix(now, 0).UTC()
	}
	definition.UpdatedAt = time.Unix(now, 0).UTC()

	_, err = s.pool.Exec(ctx, `
		INSERT INTO guardrail_definitions (name, type, description, user_path, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(name) DO UPDATE SET
			type = excluded.type,
			description = excluded.description,
			user_path = excluded.user_path,
			config = excluded.config,
			updated_at = excluded.updated_at
	`, definition.Name, definition.Type, definition.Description, nullableString(definition.UserPath), definition.Config, definition.CreatedAt.Unix(), definition.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("upsert guardrail: %w", err)
	}
	return nil
}

func (s *PostgreSQLStore) UpsertMany(ctx context.Context, definitions []Definition) error {
	if len(definitions) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin guardrail upsert transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := time.Now().UTC().Unix()
	for _, definition := range definitions {
		normalized, err := normalizeDefinition(definition)
		if err != nil {
			return err
		}
		if normalized.CreatedAt.IsZero() {
			normalized.CreatedAt = time.Unix(now, 0).UTC()
		}
		normalized.UpdatedAt = time.Unix(now, 0).UTC()

		if _, err := tx.Exec(ctx, `
			INSERT INTO guardrail_definitions (name, type, description, user_path, config, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(name) DO UPDATE SET
				type = excluded.type,
				description = excluded.description,
				user_path = excluded.user_path,
				config = excluded.config,
				updated_at = excluded.updated_at
		`, normalized.Name, normalized.Type, normalized.Description, nullableString(normalized.UserPath), normalized.Config, normalized.CreatedAt.Unix(), normalized.UpdatedAt.Unix()); err != nil {
			return fmt.Errorf("upsert guardrail %q: %w", normalized.Name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit guardrail upsert transaction: %w", err)
	}
	return nil
}

func (s *PostgreSQLStore) Delete(ctx context.Context, name string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM guardrail_definitions WHERE name = $1`, normalizeDefinitionName(name))
	if err != nil {
		return fmt.Errorf("delete guardrail: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgreSQLStore) Close() error {
	return nil
}

func scanPostgreSQLDefinition(scanner definitionScanner) (Definition, error) {
	var (
		definition    Definition
		userPath      sql.NullString
		configJSON    []byte
		createdAtUnix int64
		updatedAtUnix int64
	)
	if err := scanner.Scan(
		&definition.Name,
		&definition.Type,
		&definition.Description,
		&userPath,
		&configJSON,
		&createdAtUnix,
		&updatedAtUnix,
	); err != nil {
		return Definition{}, err
	}
	definition.UserPath = nullableStringValue(userPath)
	definition.Config = append([]byte(nil), configJSON...)
	definition.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	definition.UpdatedAt = time.Unix(updatedAtUnix, 0).UTC()
	return definition, nil
}
