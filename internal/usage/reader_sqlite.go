package usage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLiteReader implements UsageReader for SQLite databases.
type SQLiteReader struct {
	db *sql.DB
}

// NewSQLiteReader creates a new SQLite usage reader.
func NewSQLiteReader(db *sql.DB) (*SQLiteReader, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	return &SQLiteReader{db: db}, nil
}

func (r *SQLiteReader) GetSummary(ctx context.Context, days int) (*UsageSummary, error) {
	var query string
	var args []interface{}

	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano)
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM usage WHERE timestamp >= ?`
		args = append(args, cutoff)
	} else {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM usage`
	}

	summary := &UsageSummary{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.TotalRequests, &summary.TotalInput, &summary.TotalOutput, &summary.TotalTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return summary, nil
}

func (r *SQLiteReader) GetDailyUsage(ctx context.Context, days int) ([]DailyUsage, error) {
	var query string
	var args []interface{}

	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano)
		query = `SELECT DATE(timestamp) as day, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM usage WHERE timestamp >= ?
			GROUP BY DATE(timestamp) ORDER BY day`
		args = append(args, cutoff)
	} else {
		query = `SELECT DATE(timestamp) as day, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM usage GROUP BY DATE(timestamp) ORDER BY day`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily usage: %w", err)
	}
	defer rows.Close()

	var result []DailyUsage
	for rows.Next() {
		var d DailyUsage
		if err := rows.Scan(&d.Date, &d.Requests, &d.InputTokens, &d.OutputTokens, &d.TotalTokens); err != nil {
			return nil, fmt.Errorf("failed to scan daily usage row: %w", err)
		}
		result = append(result, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily usage rows: %w", err)
	}

	return result, nil
}
