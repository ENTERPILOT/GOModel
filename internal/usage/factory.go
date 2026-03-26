package usage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gomodel/config"
	"gomodel/internal/storage"
)

// Result holds the initialized usage logger and its dependencies.
// The caller is responsible for calling Close() to release resources.
type Result struct {
	Logger  LoggerInterface
	Storage storage.Storage
}

// Close releases all resources held by the usage logger.
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

// New creates a usage logger from configuration.
// Returns a Result containing the logger and storage for lifecycle management.
// The caller must call Result.Close() during shutdown.
//
// If usage tracking is disabled in the config, returns a NoopLogger with nil storage.
func New(ctx context.Context, cfg *config.Config) (*Result, error) {
	// Return noop if usage tracking is disabled
	if !cfg.Usage.Enabled {
		return &Result{
			Logger:  NewNoopLogger(buildLoggerConfig(cfg.Usage)),
			Storage: nil,
		}, nil
	}

	// Create storage configuration - reuse the same storage backend as logging
	storageCfg := cfg.Storage.BackendConfig()

	// Create storage connection
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create the usage store based on storage type
	usageStore, err := createUsageStore(store, cfg.Usage.RetentionDays)
	if err != nil {
		store.Close()
		return nil, err
	}

	// Create logger configuration
	logCfg := buildLoggerConfig(cfg.Usage)

	return &Result{
		Logger:  NewLogger(usageStore, logCfg),
		Storage: store,
	}, nil
}

// NewWithSharedStorage creates a usage logger using a shared storage connection.
// This is useful when you want to share the database connection with audit logging.
// The caller is responsible for closing the storage separately.
func NewWithSharedStorage(ctx context.Context, cfg *config.Config, store storage.Storage) (*Result, error) {
	// Return noop if usage tracking is disabled
	if !cfg.Usage.Enabled {
		return &Result{
			Logger:  NewNoopLogger(buildLoggerConfig(cfg.Usage)),
			Storage: nil,
		}, nil
	}

	if store == nil {
		return nil, fmt.Errorf("storage is required when usage tracking is enabled")
	}

	// Create the usage store based on storage type
	usageStore, err := createUsageStore(store, cfg.Usage.RetentionDays)
	if err != nil {
		return nil, err
	}

	// Create logger configuration
	logCfg := buildLoggerConfig(cfg.Usage)

	return &Result{
		Logger:  NewLogger(usageStore, logCfg),
		Storage: nil, // Don't set storage since it's shared
	}, nil
}

// NewReader creates a UsageReader from a storage backend.
// Returns nil if the storage is nil (usage data not available).
func NewReader(store storage.Storage) (UsageReader, error) {
	if store == nil {
		return nil, nil
	}

	switch store := store.(type) {
	case storage.SQLiteStorage:
		return NewSQLiteReader(store.DB())
	case storage.PostgreSQLStorage:
		return NewPostgreSQLReader(store.Pool())
	case storage.MongoDBStorage:
		return NewMongoDBReader(store.Database())
	default:
		return nil, fmt.Errorf("unsupported storage backend %T", store)
	}
}

// createUsageStore creates the appropriate UsageStore for the given storage backend.
func createUsageStore(store storage.Storage, retentionDays int) (UsageStore, error) {
	switch store := store.(type) {
	case storage.SQLiteStorage:
		return NewSQLiteStore(store.DB(), retentionDays)
	case storage.PostgreSQLStorage:
		return NewPostgreSQLStore(store.Pool(), retentionDays)
	case storage.MongoDBStorage:
		return NewMongoDBStore(store.Database(), retentionDays)
	default:
		return nil, fmt.Errorf("unsupported storage backend %T", store)
	}
}

// buildLoggerConfig creates a usage.Config from config.UsageConfig.
func buildLoggerConfig(usageCfg config.UsageConfig) Config {
	cfg := Config{
		Enabled:                   usageCfg.Enabled,
		EnforceReturningUsageData: usageCfg.EnforceReturningUsageData,
		BufferSize:                usageCfg.BufferSize,
		FlushInterval:             time.Duration(usageCfg.FlushInterval) * time.Second,
		RetentionDays:             usageCfg.RetentionDays,
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
