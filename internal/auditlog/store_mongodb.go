package auditlog

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoDBStore implements LogStore for MongoDB.
type MongoDBStore struct {
	collection    *mongo.Collection
	retentionDays int
}

// NewMongoDBStore creates a new MongoDB audit log store.
// It creates the collection and indexes if they don't exist.
// MongoDB handles TTL-based cleanup automatically via TTL indexes.
func NewMongoDBStore(database *mongo.Database, retentionDays int) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}

	collection := database.Collection("audit_logs")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create indexes
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "model", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status_code", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "provider", Value: 1}},
		},
	}

	// Add TTL index if retention is configured
	if retentionDays > 0 {
		ttlSeconds := int32(retentionDays * 24 * 60 * 60)
		indexes = append(indexes, mongo.IndexModel{
			Keys:    bson.D{{Key: "timestamp", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
		})
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// Log warning but don't fail - indexes may already exist
		slog.Warn("failed to create some MongoDB indexes", "error", err)
	}

	return &MongoDBStore{
		collection:    collection,
		retentionDays: retentionDays,
	}, nil
}

// WriteBatch writes multiple log entries to MongoDB using InsertMany.
func (s *MongoDBStore) WriteBatch(ctx context.Context, entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Convert entries to BSON documents
	docs := make([]interface{}, len(entries))
	for i, e := range entries {
		docs[i] = e
	}

	// Use unordered insert for better performance (continues on errors)
	opts := options.InsertMany().SetOrdered(false)
	_, err := s.collection.InsertMany(ctx, docs, opts)
	if err != nil {
		// Check if it's a bulk write error with some successes
		if bulkErr, ok := err.(mongo.BulkWriteException); ok {
			// Some documents may have been inserted successfully
			slog.Warn("partial audit log insert failure",
				"total", len(entries),
				"errors", len(bulkErr.WriteErrors),
			)
			return nil
		}
		return fmt.Errorf("failed to insert audit logs: %w", err)
	}

	return nil
}

// Flush is a no-op for MongoDB as writes are synchronous.
func (s *MongoDBStore) Flush(_ context.Context) error {
	return nil
}

// Close is a no-op for MongoDB as the client is managed by the storage layer.
func (s *MongoDBStore) Close() error {
	return nil
}
