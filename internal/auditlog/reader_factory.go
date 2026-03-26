package auditlog

import (
	"fmt"

	"gomodel/internal/storage"
)

// NewReader creates an audit log Reader from a storage backend.
// Returns nil when store is nil.
func NewReader(store storage.Storage) (Reader, error) {
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
