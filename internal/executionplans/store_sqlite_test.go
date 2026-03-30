package executionplans

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewSQLiteStore_SkipsExistingScopeUserPathMigration(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE execution_plan_versions (
			id TEXT PRIMARY KEY,
			scope_provider TEXT,
			scope_model TEXT,
			scope_user_path TEXT,
			scope_key TEXT NOT NULL,
			version INTEGER NOT NULL,
			active INTEGER NOT NULL DEFAULT 1,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			plan_payload JSON NOT NULL,
			plan_hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create execution_plan_versions table: %v", err)
	}

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewSQLiteStore() = nil, want store")
	}
}
