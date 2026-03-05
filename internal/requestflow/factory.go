package requestflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"gomodel/config"
	"gomodel/internal/storage"
)

// Result holds the initialized request flow manager and any owned storage.
type Result struct {
	Manager *Manager
	Storage storage.Storage
}

// Close flushes flow logs and releases owned resources.
func (r *Result) Close() error {
	var err error
	if r.Manager != nil {
		err = r.Manager.Close()
	}
	if r.Storage != nil {
		if closeErr := r.Storage.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// New creates the request flow subsystem.
func New(ctx context.Context, cfg *config.Config, shared storage.Storage) (*Result, error) {
	if cfg == nil || !cfg.Flow.Enabled {
		return &Result{}, nil
	}
	sourceMode := normalizeSourceMode(cfg.Flow.Source)
	useYAML := sourceMode == "yaml" || sourceMode == "hybrid" || (sourceMode == "db" && cfg.Flow.DBFallbackToYAML)
	useDB := sourceMode == "db" || sourceMode == "hybrid"

	var yamlDefs []*Definition
	var err error
	if useYAML {
		yamlDefs, err = loadYAMLDefinitions(cfg.Flow.YAMLPath)
		if err != nil {
			return nil, err
		}
	}

	var flowStorage storage.Storage
	ownedStorage := false
	needStore := useDB || cfg.Flow.TrackExecutions
	if needStore {
		flowStorage = shared
		if flowStorage == nil {
			flowStorage, err = storage.New(ctx, storage.Config{
				Type:       cfg.Storage.Type,
				SQLite:     storage.SQLiteConfig{Path: cfg.Storage.SQLite.Path},
				PostgreSQL: storage.PostgreSQLConfig{URL: cfg.Storage.PostgreSQL.URL, MaxConns: cfg.Storage.PostgreSQL.MaxConns},
				MongoDB:    storage.MongoDBConfig{URL: cfg.Storage.MongoDB.URL, Database: cfg.Storage.MongoDB.Database},
			})
			if err != nil {
				return nil, fmt.Errorf("initialize request flow storage: %w", err)
			}
			ownedStorage = true
		}
	}

	var store Store
	if flowStorage != nil {
		store, err = newStoreFromStorage(flowStorage, cfg.Flow.RetentionDays)
		if err != nil {
			if ownedStorage {
				_ = flowStorage.Close()
			}
			return nil, err
		}
	}

	var dbDefs []*Definition
	if useDB {
		if store == nil {
			return nil, fmt.Errorf("request flow source %q requires database storage", sourceMode)
		}
		dbDefs, err = store.ListDefinitions(ctx)
		if err != nil {
			if ownedStorage {
				_ = flowStorage.Close()
			}
			return nil, fmt.Errorf("load request flow definitions: %w", err)
		}
	}

	var logger LoggerInterface = &NoopLogger{}
	if cfg.Flow.TrackExecutions && store != nil {
		logger = NewLogger(store, LoggerConfig{
			Enabled:       true,
			BufferSize:    cfg.Flow.BufferSize,
			FlushInterval: time.Duration(cfg.Flow.FlushInterval) * time.Second,
		})
	}

	manager := NewManager(Options{
		Store:      store,
		Logger:     logger,
		YAMLDefs:   yamlDefs,
		DBDefs:     dbDefs,
		BaseRetry:  cfg.Resilience.Retry,
		BaseRules:  guardrailBaseRules(cfg),
		CacheTTL:   time.Duration(cfg.Flow.CacheTTLSeconds) * time.Second,
		SourceMode: sourceMode,
		Writable:   useDB,
	})
	result := &Result{Manager: manager}
	if ownedStorage {
		result.Storage = flowStorage
	}
	return result, nil
}

func guardrailBaseRules(cfg *config.Config) []GuardrailRule {
	if cfg == nil || !cfg.Guardrails.Enabled {
		return nil
	}
	out := make([]GuardrailRule, len(cfg.Guardrails.Rules))
	for i, rule := range cfg.Guardrails.Rules {
		out[i] = GuardrailRule{
			Name:  rule.Name,
			Type:  rule.Type,
			Order: rule.Order,
			SystemPrompt: SystemPromptSettings{
				Mode:    rule.SystemPrompt.Mode,
				Content: rule.SystemPrompt.Content,
			},
		}
	}
	return out
}

func normalizeSourceMode(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "db", "hybrid", "yaml":
		return source
	default:
		return "yaml"
	}
}

func newStoreFromStorage(store storage.Storage, retentionDays int) (Store, error) {
	if store == nil {
		return nil, nil
	}
	switch store.Type() {
	case storage.TypeSQLite:
		return NewSQLiteStore(store.SQLiteDB(), retentionDays)
	case storage.TypePostgreSQL:
		pool, ok := store.PostgreSQLPool().(*pgxpool.Pool)
		if !ok || pool == nil {
			return nil, fmt.Errorf("invalid PostgreSQL pool for request flow store")
		}
		return NewPostgreSQLStore(pool, retentionDays)
	case storage.TypeMongoDB:
		database, ok := store.MongoDatabase().(*mongo.Database)
		if !ok || database == nil {
			return nil, fmt.Errorf("invalid MongoDB database for request flow store")
		}
		return NewMongoDBStore(database, retentionDays)
	default:
		return nil, fmt.Errorf("unsupported storage type for request flow store: %s", store.Type())
	}
}
