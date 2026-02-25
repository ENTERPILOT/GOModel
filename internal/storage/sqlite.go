package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// sqliteStorage implements Storage for SQLite
type sqliteStorage struct {
	db *sql.DB
}

// NewSQLite creates a new SQLite storage connection.
// It enables WAL mode for better concurrent read/write performance.
func NewSQLite(cfg SQLiteConfig) (Storage, error) {
	if cfg.Path == "" {
		cfg.Path = DefaultSQLitePath
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open database with WAL mode and busy timeout
	dsn := fmt.Sprintf("%s?_journal=WAL&_busy_timeout=5000&_synchronous=NORMAL", cfg.Path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Allow multiple readers to operate concurrently. WAL mode handles this natively.
	// We keep a modest pool to prevent file descriptor exhaustion.
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	return &sqliteStorage{db: db}, nil
}

func (s *sqliteStorage) Type() string {
	return TypeSQLite
}

func (s *sqliteStorage) SQLiteDB() *sql.DB {
	return s.db
}

func (s *sqliteStorage) PostgreSQLPool() interface{} {
	return nil
}

func (s *sqliteStorage) MongoDatabase() interface{} {
	return nil
}

func (s *sqliteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
