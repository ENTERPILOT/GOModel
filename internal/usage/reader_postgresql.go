package usage

import (
	"context"
	"fmt"

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

func (r *PostgreSQLReader) GetSummary(ctx context.Context, params UsageQueryParams) (*UsageSummary, error) {
	var query string
	var args []interface{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	costCols := `, SUM(input_cost), SUM(output_cost), SUM(total_cost)`

	if !startZero && !endZero {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)` + costCols + `
			FROM "usage" WHERE timestamp >= $1 AND timestamp < $2`
		args = append(args, params.StartDate.UTC(), params.EndDate.AddDate(0, 0, 1).UTC())
	} else if !startZero {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)` + costCols + `
			FROM "usage" WHERE timestamp >= $1`
		args = append(args, params.StartDate.UTC())
	} else {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)` + costCols + `
			FROM "usage"`
	}

	summary := &UsageSummary{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&summary.TotalRequests, &summary.TotalInput, &summary.TotalOutput, &summary.TotalTokens,
		&summary.TotalInputCost, &summary.TotalOutputCost, &summary.TotalCost,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return summary, nil
}

func (r *PostgreSQLReader) GetUsageByModel(ctx context.Context, params UsageQueryParams) ([]ModelUsage, error) {
	var query string
	var args []interface{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
			FROM "usage" WHERE timestamp >= $1 AND timestamp < $2 GROUP BY model, provider`
		args = append(args, params.StartDate.UTC(), params.EndDate.AddDate(0, 0, 1).UTC())
	} else if !startZero {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
			FROM "usage" WHERE timestamp >= $1 GROUP BY model, provider`
		args = append(args, params.StartDate.UTC())
	} else {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
			FROM "usage" GROUP BY model, provider`
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage by model: %w", err)
	}
	defer rows.Close()

	result := make([]ModelUsage, 0)
	for rows.Next() {
		var m ModelUsage
		if err := rows.Scan(&m.Model, &m.Provider, &m.InputTokens, &m.OutputTokens); err != nil {
			return nil, fmt.Errorf("failed to scan usage by model row: %w", err)
		}
		result = append(result, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage by model rows: %w", err)
	}

	return result, nil
}

func pgGroupExpr(interval string) string {
	switch interval {
	case "weekly":
		return `to_char(DATE_TRUNC('week', timestamp AT TIME ZONE 'UTC'), 'IYYY-"W"IW')`
	case "monthly":
		return `to_char(DATE_TRUNC('month', timestamp AT TIME ZONE 'UTC'), 'YYYY-MM')`
	case "yearly":
		return `to_char(DATE_TRUNC('year', timestamp AT TIME ZONE 'UTC'), 'YYYY')`
	default:
		return `to_char(DATE(timestamp AT TIME ZONE 'UTC'), 'YYYY-MM-DD')`
	}
}

func (r *PostgreSQLReader) GetDailyUsage(ctx context.Context, params UsageQueryParams) ([]DailyUsage, error) {
	interval := params.Interval
	if interval == "" {
		interval = "daily"
	}
	groupExpr := pgGroupExpr(interval)

	var where string
	var args []interface{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		where = ` WHERE timestamp >= $1 AND timestamp < $2`
		args = append(args, params.StartDate.UTC(), params.EndDate.AddDate(0, 0, 1).UTC())
	} else if !startZero {
		where = ` WHERE timestamp >= $1`
		args = append(args, params.StartDate.UTC())
	}

	query := fmt.Sprintf(`SELECT %s as period, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM "usage"%s GROUP BY %s ORDER BY period`, groupExpr, where, groupExpr)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily usage: %w", err)
	}
	defer rows.Close()

	result := make([]DailyUsage, 0)
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
