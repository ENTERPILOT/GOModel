package usage

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoDBReader implements UsageReader for MongoDB.
type MongoDBReader struct {
	collection *mongo.Collection
}

// NewMongoDBReader creates a new MongoDB usage reader.
func NewMongoDBReader(database *mongo.Database) (*MongoDBReader, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &MongoDBReader{collection: database.Collection("usage")}, nil
}

func (r *MongoDBReader) GetSummary(ctx context.Context, params UsageQueryParams) (*UsageSummary, error) {
	pipeline := bson.A{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "timestamp", Value: bson.D{
				{Key: "$gte", Value: params.StartDate.UTC()},
				{Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()},
			}},
		}}})
	} else if !startZero {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: params.StartDate.UTC()}}},
		}}})
	} else if !endZero {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "timestamp", Value: bson.D{{Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()}}},
		}}})
	}

	pipeline = append(pipeline, bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: nil},
		{Key: "total_requests", Value: bson.D{{Key: "$sum", Value: 1}}},
		{Key: "total_input", Value: bson.D{{Key: "$sum", Value: "$input_tokens"}}},
		{Key: "total_output", Value: bson.D{{Key: "$sum", Value: "$output_tokens"}}},
		{Key: "total_tokens", Value: bson.D{{Key: "$sum", Value: "$total_tokens"}}},
	}}})

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage summary: %w", err)
	}
	defer cursor.Close(ctx)

	summary := &UsageSummary{}
	if cursor.Next(ctx) {
		var result struct {
			TotalRequests int   `bson:"total_requests"`
			TotalInput    int64 `bson:"total_input"`
			TotalOutput   int64 `bson:"total_output"`
			TotalTokens   int64 `bson:"total_tokens"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode usage summary: %w", err)
		}
		summary.TotalRequests = result.TotalRequests
		summary.TotalInput = result.TotalInput
		summary.TotalOutput = result.TotalOutput
		summary.TotalTokens = result.TotalTokens
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage summary cursor: %w", err)
	}

	return summary, nil
}

func mongoDateFormat(interval string) string {
	switch interval {
	case "weekly":
		return "%G-W%V"
	case "monthly":
		return "%Y-%m"
	case "yearly":
		return "%Y"
	default:
		return "%Y-%m-%d"
	}
}

func (r *MongoDBReader) GetDailyUsage(ctx context.Context, params UsageQueryParams) ([]DailyUsage, error) {
	interval := params.Interval
	if interval == "" {
		interval = "daily"
	}

	pipeline := bson.A{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "timestamp", Value: bson.D{
				{Key: "$gte", Value: params.StartDate.UTC()},
				{Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()},
			}},
		}}})
	} else if !startZero {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: params.StartDate.UTC()}}},
		}}})
	} else if !endZero {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "timestamp", Value: bson.D{{Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()}}},
		}}})
	}

	dateFormat := mongoDateFormat(interval)

	pipeline = append(pipeline,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "$dateToString", Value: bson.D{
				{Key: "format", Value: dateFormat},
				{Key: "date", Value: "$timestamp"},
			}}}},
			{Key: "requests", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "input_tokens", Value: bson.D{{Key: "$sum", Value: "$input_tokens"}}},
			{Key: "output_tokens", Value: bson.D{{Key: "$sum", Value: "$output_tokens"}}},
			{Key: "total_tokens", Value: bson.D{{Key: "$sum", Value: "$total_tokens"}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	)

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate daily usage: %w", err)
	}
	defer cursor.Close(ctx)

	result := make([]DailyUsage, 0)
	for cursor.Next(ctx) {
		var row struct {
			Date         string `bson:"_id"`
			Requests     int    `bson:"requests"`
			InputTokens  int64  `bson:"input_tokens"`
			OutputTokens int64  `bson:"output_tokens"`
			TotalTokens  int64  `bson:"total_tokens"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("failed to decode daily usage row: %w", err)
		}
		result = append(result, DailyUsage{
			Date:         row.Date,
			Requests:     row.Requests,
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalTokens:  row.TotalTokens,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily usage cursor: %w", err)
	}

	return result, nil
}
