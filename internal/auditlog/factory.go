package auditlog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"gomodel/config"
	"gomodel/internal/storage"
)

// Result holds the initialized audit logger and its dependencies.
// The caller is responsible for calling Close() to release resources.
type Result struct {
	Logger  LoggerInterface
	Storage storage.Storage
}

// Close releases all resources held by the audit logger.
// Safe to call multiple times.
func (r *Result) Close() error {
	var errs []error
	if r.Logger != nil {
		if err := r.Logger.Close(); err != nil {
			errs = append(errs, fmt.Errorf("logger close: %w", err))
		}
	}
	if r.Storage != nil {
		if err := r.Storage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("storage close: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %w", errors.Join(errs...))
	}
	return nil
}

// New creates an audit logger from configuration.
// Returns a Result containing the logger and storage for lifecycle management.
// The caller must call Result.Close() during shutdown.
//
// If logging is disabled in the config, returns a NoopLogger with nil storage.
func New(ctx context.Context, cfg *config.Config) (*Result, error) {
	// Return noop if logging is disabled
	if !cfg.Logging.Enabled {
		return &Result{
			Logger:  &NoopLogger{},
			Storage: nil,
		}, nil
	}

	// Create storage configuration
	storageCfg := buildStorageConfig(cfg)

	// Create storage connection
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create the log store based on storage type
	logStore, err := createLogStore(store, cfg.Logging.RetentionDays)
	if err != nil {
		store.Close()
		return nil, err
	}

	// Create logger configuration
	logCfg := buildLoggerConfig(cfg.Logging)

	return &Result{
		Logger:  NewLogger(logStore, logCfg),
		Storage: store,
	}, nil
}

// buildStorageConfig creates a storage.Config from the application config.
func buildStorageConfig(cfg *config.Config) storage.Config {
	storageCfg := storage.Config{
		Type: cfg.Storage.Type,
		SQLite: storage.SQLiteConfig{
			Path: cfg.Storage.SQLite.Path,
		},
		PostgreSQL: storage.PostgreSQLConfig{
			URL:      cfg.Storage.PostgreSQL.URL,
			MaxConns: cfg.Storage.PostgreSQL.MaxConns,
		},
		MongoDB: storage.MongoDBConfig{
			URL:      cfg.Storage.MongoDB.URL,
			Database: cfg.Storage.MongoDB.Database,
		},
	}

	// Apply defaults
	if storageCfg.Type == "" {
		storageCfg.Type = storage.TypeSQLite
	}
	if storageCfg.SQLite.Path == "" {
		storageCfg.SQLite.Path = ".cache/gomodel.db"
	}
	if storageCfg.MongoDB.Database == "" {
		storageCfg.MongoDB.Database = "gomodel"
	}

	return storageCfg
}

// createLogStore creates the appropriate LogStore for the given storage backend.
func createLogStore(store storage.Storage, retentionDays int) (LogStore, error) {
	switch store.Type() {
	case storage.TypeSQLite:
		return NewSQLiteStore(store.SQLiteDB(), retentionDays)

	case storage.TypePostgreSQL:
		pool := store.PostgreSQLPool()
		if pool == nil {
			return nil, fmt.Errorf("PostgreSQL pool is nil")
		}
		pgxPool, ok := pool.(*pgxpool.Pool)
		if !ok {
			return nil, fmt.Errorf("invalid PostgreSQL pool type: %T", pool)
		}
		return NewPostgreSQLStore(pgxPool, retentionDays)

	case storage.TypeMongoDB:
		db := store.MongoDatabase()
		if db == nil {
			return nil, fmt.Errorf("MongoDB database is nil")
		}
		mongoDB, ok := db.(*mongo.Database)
		if !ok {
			return nil, fmt.Errorf("invalid MongoDB database type: %T", db)
		}
		return NewMongoDBStore(mongoDB, retentionDays)

	default:
		return nil, fmt.Errorf("unknown storage type: %s", store.Type())
	}
}

// buildLoggerConfig creates an auditlog.Config from config.LogConfig.
func buildLoggerConfig(logCfg config.LogConfig) Config {
	cfg := Config{
		Enabled:               logCfg.Enabled,
		LogBodies:             logCfg.LogBodies,
		LogHeaders:            logCfg.LogHeaders,
		BufferSize:            logCfg.BufferSize,
		FlushInterval:         time.Duration(logCfg.FlushInterval) * time.Second,
		RetentionDays:         logCfg.RetentionDays,
		OnlyModelInteractions: logCfg.OnlyModelInteractions,
	}

	// Apply defaults
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	return cfg
}
