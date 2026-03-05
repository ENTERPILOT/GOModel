package requestflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoDBStore persists request flow data in MongoDB.
type MongoDBStore struct {
	definitions *mongo.Collection
	executions  *mongo.Collection
}

// NewMongoDBStore creates a MongoDB-backed request flow store.
func NewMongoDBStore(database *mongo.Database, retentionDays int) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	store := &MongoDBStore{
		definitions: database.Collection("flow_definitions"),
		executions:  database.Collection("flow_executions"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "enabled", Value: 1}}},
		{Keys: bson.D{{Key: "match.model", Value: 1}}},
	}
	execIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "request_id", Value: 1}}},
		{Keys: bson.D{{Key: "model", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	}
	if retentionDays > 0 {
		ttlSeconds := int32(int64(retentionDays) * 24 * 60 * 60)
		execIndexes = append(execIndexes, mongo.IndexModel{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
		})
	} else {
		execIndexes = append(execIndexes, mongo.IndexModel{Keys: bson.D{{Key: "timestamp", Value: -1}}})
	}
	if _, err := store.definitions.Indexes().CreateMany(ctx, defIndexes); err != nil {
		slog.Warn("failed to create request flow definition indexes", "error", err)
	}
	if _, err := store.executions.Indexes().CreateMany(ctx, execIndexes); err != nil {
		slog.Warn("failed to create request flow execution indexes", "error", err)
	}
	return store, nil
}

func (s *MongoDBStore) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	cursor, err := s.definitions.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "updated_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	defs := make([]*Definition, 0)
	for cursor.Next(ctx) {
		var def Definition
		if err := cursor.Decode(&def); err != nil {
			return nil, err
		}
		defs = append(defs, &def)
	}
	return defs, cursor.Err()
}

func (s *MongoDBStore) SaveDefinition(ctx context.Context, def *Definition) error {
	_, err := s.definitions.ReplaceOne(ctx, bson.D{{Key: "_id", Value: def.ID}}, def, options.Replace().SetUpsert(true))
	return err
}

func (s *MongoDBStore) DeleteDefinition(ctx context.Context, id string) error {
	res, err := s.definitions.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MongoDBStore) WriteExecutionBatch(ctx context.Context, entries []*Execution) error {
	if len(entries) == 0 {
		return nil
	}
	docs := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		if entry != nil {
			docs = append(docs, entry)
		}
	}
	if len(docs) == 0 {
		return nil
	}
	_, err := s.executions.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	return err
}

func (s *MongoDBStore) ListExecutions(ctx context.Context, params ExecutionQueryParams) (*ExecutionLogResult, error) {
	filter := bson.D{}
	if params.RequestID != "" {
		filter = append(filter, bson.E{Key: "request_id", Value: params.RequestID})
	}
	if params.Model != "" {
		filter = append(filter, bson.E{Key: "model", Value: params.Model})
	}
	if params.Search != "" {
		regex := bson.D{{Key: "$regex", Value: params.Search}, {Key: "$options", Value: "i"}}
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "request_id", Value: regex}},
			bson.D{{Key: "model", Value: regex}},
			bson.D{{Key: "provider", Value: regex}},
			bson.D{{Key: "plan_name", Value: regex}},
			bson.D{{Key: "status", Value: regex}},
		}})
	}
	total, err := s.executions.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	cursor, err := s.executions.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetSkip(int64(params.Offset)).SetLimit(int64(params.Limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	entries := make([]*Execution, 0)
	for cursor.Next(ctx) {
		var entry Execution
		if err := cursor.Decode(&entry); err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return &ExecutionLogResult{Entries: entries, Total: int(total), Limit: params.Limit, Offset: params.Offset}, nil
}

func (s *MongoDBStore) Flush(_ context.Context) error { return nil }

func (s *MongoDBStore) Close() error { return nil }
