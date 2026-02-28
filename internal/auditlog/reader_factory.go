package auditlog

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"gomodel/internal/storage"
)

// NewReader creates an audit log Reader from a storage backend.
// Returns nil when store is nil.
func NewReader(store storage.Storage) (Reader, error) {
	if store == nil {
		return nil, nil
	}

	switch store.Type() {
	case storage.TypeSQLite:
		return NewSQLiteReader(store.SQLiteDB())

	case storage.TypePostgreSQL:
		pool := store.PostgreSQLPool()
		if pool == nil {
			return nil, fmt.Errorf("PostgreSQL pool is nil")
		}
		pgxPool, ok := pool.(*pgxpool.Pool)
		if !ok {
			return nil, fmt.Errorf("invalid PostgreSQL pool type: %T", pool)
		}
		return NewPostgreSQLReader(pgxPool)

	case storage.TypeMongoDB:
		db := store.MongoDatabase()
		if db == nil {
			return nil, fmt.Errorf("MongoDB database is nil")
		}
		mongoDB, ok := db.(*mongo.Database)
		if !ok {
			return nil, fmt.Errorf("invalid MongoDB database type: %T", db)
		}
		return NewMongoDBReader(mongoDB)

	default:
		return nil, fmt.Errorf("unknown storage type: %s", store.Type())
	}
}
