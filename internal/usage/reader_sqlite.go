package usage

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *SQLiteReader) GetSummary(ctx context.Context, params UsageQueryParams) (*UsageSummary, error) {
	var query string
	var args []interface{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	costCols := `, SUM(input_cost), SUM(output_cost), SUM(total_cost)`

	if !startZero && !endZero {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)` + costCols + `
			FROM usage WHERE timestamp >= ? AND timestamp < ?`
		args = append(args, params.StartDate.UTC().Format("2006-01-02"), params.EndDate.AddDate(0, 0, 1).UTC().Format("2006-01-02"))
	} else if !startZero {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)` + costCols + `
			FROM usage WHERE timestamp >= ?`
		args = append(args, params.StartDate.UTC().Format("2006-01-02"))
	} else {
		query = `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)` + costCols + `
			FROM usage`
	}

	summary := &UsageSummary{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.TotalRequests, &summary.TotalInput, &summary.TotalOutput, &summary.TotalTokens,
		&summary.TotalInputCost, &summary.TotalOutputCost, &summary.TotalCost,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return summary, nil
}

func (r *SQLiteReader) GetUsageByModel(ctx context.Context, params UsageQueryParams) ([]ModelUsage, error) {
	var query string
	var args []interface{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	costCols := `, SUM(input_cost), SUM(output_cost), SUM(total_cost)`

	if !startZero && !endZero {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)` + costCols + `
			FROM usage WHERE timestamp >= ? AND timestamp < ? GROUP BY model, provider`
		args = append(args, params.StartDate.UTC().Format("2006-01-02"), params.EndDate.AddDate(0, 0, 1).UTC().Format("2006-01-02"))
	} else if !startZero {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)` + costCols + `
			FROM usage WHERE timestamp >= ? GROUP BY model, provider`
		args = append(args, params.StartDate.UTC().Format("2006-01-02"))
	} else {
		query = `SELECT model, provider, COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)` + costCols + `
			FROM usage GROUP BY model, provider`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *SQLiteReader) GetUsageLog(ctx context.Context, params UsageLogParams) (*UsageLogResult, error) {
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

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		conditions = append(conditions, "timestamp >= ?", "timestamp < ?")
		args = append(args, params.StartDate.UTC().Format("2006-01-02"), params.EndDate.AddDate(0, 0, 1).UTC().Format("2006-01-02"))
	} else if !startZero {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, params.StartDate.UTC().Format("2006-01-02"))
	}

	if params.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, params.Model)
	}
	if params.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, params.Provider)
	}
	if params.Search != "" {
		conditions = append(conditions, "(model LIKE ? OR provider LIKE ? OR request_id LIKE ? OR provider_id LIKE ?)")
		s := "%" + params.Search + "%"
		args = append(args, s, s, s, s)
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
	countQuery := "SELECT COUNT(*) FROM usage" + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count usage log entries: %w", err)
	}

	// Fetch page
	dataQuery := `SELECT id, request_id, provider_id, timestamp, model, provider, endpoint,
		input_tokens, output_tokens, total_tokens, input_cost, output_cost, total_cost, raw_data, costs_calculation_caveat
		FROM usage` + where + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	dataArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage log: %w", err)
	}
	defer rows.Close()

	entries := make([]UsageLogEntry, 0)
	for rows.Next() {
		var e UsageLogEntry
		var ts string
		var caveat *string
		var rawDataJSON *string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.ProviderID, &ts, &e.Model, &e.Provider, &e.Endpoint,
			&e.InputTokens, &e.OutputTokens, &e.TotalTokens, &e.InputCost, &e.OutputCost, &e.TotalCost, &rawDataJSON, &caveat); err != nil {
			return nil, fmt.Errorf("failed to scan usage log row: %w", err)
		}
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			e.Timestamp = t
		} else if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", ts); err == nil {
			e.Timestamp = t
		} else if t, err := time.Parse("2006-01-02T15:04:05Z", ts); err == nil {
			e.Timestamp = t
		}
		if rawDataJSON != nil && *rawDataJSON != "" {
			_ = json.Unmarshal([]byte(*rawDataJSON), &e.RawData)
		}
		if caveat != nil {
			e.CostsCalculationCaveat = *caveat
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

func sqliteGroupExpr(interval string) string {
	switch interval {
	case "weekly":
		return `strftime('%G-W%V', timestamp)`
	case "monthly":
		return `strftime('%Y-%m', timestamp)`
	case "yearly":
		return `strftime('%Y', timestamp)`
	default:
		return `DATE(timestamp)`
	}
}

func (r *SQLiteReader) GetDailyUsage(ctx context.Context, params UsageQueryParams) ([]DailyUsage, error) {
	interval := params.Interval
	if interval == "" {
		interval = "daily"
	}
	groupExpr := sqliteGroupExpr(interval)

	var where string
	var args []interface{}

	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		where = ` WHERE timestamp >= ? AND timestamp < ?`
		args = append(args, params.StartDate.UTC().Format("2006-01-02"), params.EndDate.AddDate(0, 0, 1).UTC().Format("2006-01-02"))
	} else if !startZero {
		where = ` WHERE timestamp >= ?`
		args = append(args, params.StartDate.UTC().Format("2006-01-02"))
	}

	query := fmt.Sprintf(`SELECT %s as period, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM usage%s GROUP BY %s ORDER BY period`, groupExpr, where, groupExpr)

	rows, err := r.db.QueryContext(ctx, query, args...)
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
