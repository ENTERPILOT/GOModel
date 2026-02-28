package auditlog

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoDBReader implements Reader for MongoDB.
type MongoDBReader struct {
	collection *mongo.Collection
}

// NewMongoDBReader creates a new MongoDB audit log reader.
func NewMongoDBReader(database *mongo.Database) (*MongoDBReader, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &MongoDBReader{collection: database.Collection("audit_logs")}, nil
}

// GetLogs returns a paginated list of audit log entries.
func (r *MongoDBReader) GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error) {
	limit, offset := clampLimitOffset(params.Limit, params.Offset)

	matchFilters := bson.D{}

	if tsFilter := mongoDateRangeFilter(params.QueryParams); tsFilter != nil {
		matchFilters = append(matchFilters, bson.E{Key: "timestamp", Value: tsFilter})
	}
	if params.Model != "" {
		matchFilters = append(matchFilters, bson.E{Key: "model", Value: params.Model})
	}
	if params.Provider != "" {
		matchFilters = append(matchFilters, bson.E{Key: "provider", Value: params.Provider})
	}
	if params.Method != "" {
		matchFilters = append(matchFilters, bson.E{Key: "method", Value: params.Method})
	}
	if params.Path != "" {
		matchFilters = append(matchFilters, bson.E{Key: "path", Value: params.Path})
	}
	if params.ErrorType != "" {
		matchFilters = append(matchFilters, bson.E{Key: "error_type", Value: params.ErrorType})
	}
	if params.StatusCode != nil {
		matchFilters = append(matchFilters, bson.E{Key: "status_code", Value: *params.StatusCode})
	}
	if params.Stream != nil {
		matchFilters = append(matchFilters, bson.E{Key: "stream", Value: *params.Stream})
	}
	if params.Search != "" {
		pattern := regexp.QuoteMeta(params.Search)
		regex := bson.D{{Key: "$regex", Value: pattern}, {Key: "$options", Value: "i"}}
		matchFilters = append(matchFilters, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "request_id", Value: regex}},
			bson.D{{Key: "model", Value: regex}},
			bson.D{{Key: "provider", Value: regex}},
			bson.D{{Key: "method", Value: regex}},
			bson.D{{Key: "path", Value: regex}},
			bson.D{{Key: "error_type", Value: regex}},
			bson.D{{Key: "data.error_message", Value: regex}},
		}})
	}

	pipeline := bson.A{}
	if len(matchFilters) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: matchFilters}})
	}

	pipeline = append(pipeline, bson.D{{Key: "$facet", Value: bson.D{
		{Key: "data", Value: bson.A{
			bson.D{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: -1}}}},
			bson.D{{Key: "$skip", Value: offset}},
			bson.D{{Key: "$limit", Value: limit}},
		}},
		{Key: "total", Value: bson.A{
			bson.D{{Key: "$count", Value: "count"}},
		}},
	}}})

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate audit logs: %w", err)
	}
	defer cursor.Close(ctx)

	var facetResult struct {
		Data []struct {
			ID         string    `bson:"_id"`
			Timestamp  time.Time `bson:"timestamp"`
			DurationNs int64     `bson:"duration_ns"`
			Model      string    `bson:"model"`
			Provider   string    `bson:"provider"`
			StatusCode int       `bson:"status_code"`
			RequestID  string    `bson:"request_id"`
			ClientIP   string    `bson:"client_ip"`
			Method     string    `bson:"method"`
			Path       string    `bson:"path"`
			Stream     bool      `bson:"stream"`
			ErrorType  string    `bson:"error_type"`
			Data       *LogData  `bson:"data"`
		} `bson:"data"`
		Total []struct {
			Count int `bson:"count"`
		} `bson:"total"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&facetResult); err != nil {
			return nil, fmt.Errorf("failed to decode audit log facet result: %w", err)
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log cursor: %w", err)
	}

	total := 0
	if len(facetResult.Total) > 0 {
		total = facetResult.Total[0].Count
	}

	entries := make([]LogEntry, 0, len(facetResult.Data))
	for _, row := range facetResult.Data {
		entries = append(entries, LogEntry{
			ID:         row.ID,
			Timestamp:  row.Timestamp,
			DurationNs: row.DurationNs,
			Model:      row.Model,
			Provider:   row.Provider,
			StatusCode: row.StatusCode,
			RequestID:  row.RequestID,
			ClientIP:   row.ClientIP,
			Method:     row.Method,
			Path:       row.Path,
			Stream:     row.Stream,
			ErrorType:  row.ErrorType,
			Data:       row.Data,
		})
	}

	return &LogListResult{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

func mongoDateRangeFilter(params QueryParams) bson.D {
	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		return bson.D{{Key: "$gte", Value: params.StartDate.UTC()}, {Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()}}
	}
	if !startZero {
		return bson.D{{Key: "$gte", Value: params.StartDate.UTC()}}
	}
	if !endZero {
		return bson.D{{Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()}}
	}
	return nil
}
