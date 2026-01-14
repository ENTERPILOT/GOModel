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
		cfg.Path = ".cache/gomodel.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open database with WAL mode and busy timeout
	// WAL mode allows concurrent reads while writing
	dsn := fmt.Sprintf("%s?_journal=WAL&_busy_timeout=5000&_synchronous=NORMAL", cfg.Path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Set connection pool settings for SQLite
	// SQLite doesn't benefit from multiple connections for writes,
	// but we allow some for concurrent reads
	db.SetMaxOpenConns(1) // SQLite only allows one writer at a time
	db.SetMaxIdleConns(1)

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
