package usage

import (
	"context"
	"encoding/json"
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

	costCols := `, SUM(input_cost), SUM(output_cost), SUM(total_cost)`

	if !startZero && !endZero {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)` + costCols + `
			FROM "usage" WHERE timestamp >= $1 AND timestamp < $2 GROUP BY model, provider`
		args = append(args, params.StartDate.UTC(), params.EndDate.AddDate(0, 0, 1).UTC())
	} else if !startZero {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)` + costCols + `
			FROM "usage" WHERE timestamp >= $1 GROUP BY model, provider`
		args = append(args, params.StartDate.UTC())
	} else {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)` + costCols + `
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
		if err := rows.Scan(&m.Model, &m.Provider, &m.InputTokens, &m.OutputTokens, &m.InputCost, &m.OutputCost, &m.TotalCost); err != nil {
			return nil, fmt.Errorf("failed to scan usage by model row: %w", err)
		}
		result = append(result, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage by model rows: %w", err)
	}

	return result, nil
}

func (r *PostgreSQLReader) GetUsageLog(ctx context.Context, params UsageLogParams) (*UsageLogResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIdx))
		args = append(args, params.StartDate.UTC())
		argIdx++
		conditions = append(conditions, fmt.Sprintf("timestamp < $%d", argIdx))
		args = append(args, params.EndDate.AddDate(0, 0, 1).UTC())
		argIdx++
	} else if !startZero {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIdx))
		args = append(args, params.StartDate.UTC())
		argIdx++
	}

	if params.Model != "" {
		conditions = append(conditions, fmt.Sprintf("model = $%d", argIdx))
		args = append(args, params.Model)
		argIdx++
	}
	if params.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
		args = append(args, params.Provider)
		argIdx++
	}
	if params.Search != "" {
		s := "%" + params.Search + "%"
		conditions = append(conditions, fmt.Sprintf("(model ILIKE $%d OR provider ILIKE $%d OR request_id ILIKE $%d OR provider_id ILIKE $%d)", argIdx, argIdx, argIdx, argIdx))
		args = append(args, s)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			where += " AND " + c
		}
	}

	// Count total
	var total int
	countQuery := `SELECT COUNT(*) FROM "usage"` + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count usage log entries: %w", err)
	}

	// Fetch page
	dataQuery := fmt.Sprintf(`SELECT id, request_id, provider_id, timestamp, model, provider, endpoint,
		input_tokens, output_tokens, total_tokens, input_cost, output_cost, total_cost, raw_data, COALESCE(costs_calculation_caveat, '')
		FROM "usage"%s ORDER BY timestamp DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	dataArgs := append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage log: %w", err)
	}
	defer rows.Close()

	entries := make([]UsageLogEntry, 0)
	for rows.Next() {
		var e UsageLogEntry
		var rawDataJSON *string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.ProviderID, &e.Timestamp, &e.Model, &e.Provider, &e.Endpoint,
			&e.InputTokens, &e.OutputTokens, &e.TotalTokens, &e.InputCost, &e.OutputCost, &e.TotalCost, &rawDataJSON, &e.CostsCalculationCaveat); err != nil {
			return nil, fmt.Errorf("failed to scan usage log row: %w", err)
		}
		if rawDataJSON != nil && *rawDataJSON != "" {
			_ = json.Unmarshal([]byte(*rawDataJSON), &e.RawData)
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage log rows: %w", err)
	}

	return &UsageLogResult{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
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
