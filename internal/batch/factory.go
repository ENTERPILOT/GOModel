package batch

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"gomodel/config"
	"gomodel/internal/storage"
)

// Result holds the initialized batch store and optional owned storage.
type Result struct {
	Store   Store
	Storage storage.Storage
}

// Close releases resources held by the batch store.
func (r *Result) Close() error {
	var errs []error
	if r.Store != nil {
		if err := r.Store.Close(); err != nil {
			errs = append(errs, fmt.Errorf("store close: %w", err))
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

// New creates a batch store from app configuration.
func New(ctx context.Context, cfg *config.Config) (*Result, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	storageCfg := buildStorageConfig(cfg)
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	batchStore, err := createStore(ctx, store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	return &Result{
		Store:   batchStore,
		Storage: store,
	}, nil
}

// NewWithSharedStorage creates a batch store using a shared storage connection.
func NewWithSharedStorage(ctx context.Context, shared storage.Storage) (*Result, error) {
	if shared == nil {
		return nil, fmt.Errorf("shared storage is required")
	}
	batchStore, err := createStore(ctx, shared)
	if err != nil {
		return nil, err
	}
	return &Result{
		Store: batchStore,
	}, nil
}

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

	if storageCfg.Type == "" {
		storageCfg.Type = storage.TypeSQLite
	}
	if storageCfg.SQLite.Path == "" {
		storageCfg.SQLite.Path = storage.DefaultSQLitePath
	}
	if storageCfg.MongoDB.Database == "" {
		storageCfg.MongoDB.Database = "gomodel"
	}
	return storageCfg
}

func createStore(ctx context.Context, store storage.Storage) (Store, error) {
	switch store.Type() {
	case storage.TypeSQLite:
		return NewSQLiteStore(store.SQLiteDB())
	case storage.TypePostgreSQL:
		pool := store.PostgreSQLPool()
		if pool == nil {
			return nil, fmt.Errorf("PostgreSQL pool is nil")
		}
		pgxPool, ok := pool.(*pgxpool.Pool)
		if !ok {
			return nil, fmt.Errorf("invalid PostgreSQL pool type: %T", pool)
		}
		return NewPostgreSQLStore(ctx, pgxPool)
	case storage.TypeMongoDB:
		db := store.MongoDatabase()
		if db == nil {
			return nil, fmt.Errorf("MongoDB database is nil")
		}
		mongoDB, ok := db.(*mongo.Database)
		if !ok {
			return nil, fmt.Errorf("invalid MongoDB database type: %T", db)
		}
		return NewMongoDBStore(mongoDB)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", store.Type())
	}
}
