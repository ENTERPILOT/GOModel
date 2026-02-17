package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLReader implements UsageReader for PostgreSQL databases.
type PostgreSQLReader struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLReader creates a new PostgreSQL usage reader.
func NewPostgreSQLReader(pool *pgxpool.Pool) (*PostgreSQLReader, error) {
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}
	return &PostgreSQLReader{pool: pool}, nil
}

func (r *PostgreSQLReader) GetSummary(ctx context.Context, days int) (*UsageSummary, error) {
	var query string
	var args []interface{}

	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days).UTC()
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM "usage" WHERE timestamp >= $1`
		args = append(args, cutoff)
	} else {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM "usage"`
	}

	summary := &UsageSummary{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&summary.TotalRequests, &summary.TotalInput, &summary.TotalOutput, &summary.TotalTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return summary, nil
}

func (r *PostgreSQLReader) GetDailyUsage(ctx context.Context, days int) ([]DailyUsage, error) {
	var query string
	var args []interface{}

	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days).UTC()
		query = `SELECT DATE(timestamp AT TIME ZONE 'UTC') as day, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM "usage" WHERE timestamp >= $1
			GROUP BY DATE(timestamp AT TIME ZONE 'UTC') ORDER BY day`
		args = append(args, cutoff)
	} else {
		query = `SELECT DATE(timestamp AT TIME ZONE 'UTC') as day, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
			FROM "usage" GROUP BY DATE(timestamp AT TIME ZONE 'UTC') ORDER BY day`
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily usage: %w", err)
	}
	defer rows.Close()

	var result []DailyUsage
	for rows.Next() {
		var d DailyUsage
		var day time.Time
		if err := rows.Scan(&day, &d.Requests, &d.InputTokens, &d.OutputTokens, &d.TotalTokens); err != nil {
			return nil, fmt.Errorf("failed to scan daily usage row: %w", err)
		}
		d.Date = day.Format("2006-01-02")
		result = append(result, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily usage rows: %w", err)
	}

	return result, nil
}
