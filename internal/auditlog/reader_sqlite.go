package auditlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// SQLiteReader implements Reader for SQLite databases.
type SQLiteReader struct {
	db *sql.DB
}

// NewSQLiteReader creates a new SQLite audit log reader.
func NewSQLiteReader(db *sql.DB) (*SQLiteReader, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	return &SQLiteReader{db: db}, nil
}

// GetLogs returns a paginated list of audit log entries.
func (r *SQLiteReader) GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error) {
	limit, offset := clampLimitOffset(params.Limit, params.Offset)

	conditions, args := sqliteDateRangeConditions(params.QueryParams)

	if params.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, params.Model)
	}
	if params.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, params.Provider)
	}
	if params.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, params.Method)
	}
	if params.Path != "" {
		conditions = append(conditions, "path = ?")
		args = append(args, params.Path)
	}
	if params.ErrorType != "" {
		conditions = append(conditions, "error_type = ?")
		args = append(args, params.ErrorType)
	}
	if params.StatusCode != nil {
		conditions = append(conditions, "status_code = ?")
		args = append(args, *params.StatusCode)
	}
	if params.Stream != nil {
		conditions = append(conditions, "stream = ?")
		if *params.Stream {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if params.Search != "" {
		s := "%" + escapeLikeWildcards(params.Search) + "%"
		conditions = append(conditions, `(request_id LIKE ? ESCAPE '\' OR model LIKE ? ESCAPE '\' OR provider LIKE ? ESCAPE '\' OR method LIKE ? ESCAPE '\' OR path LIKE ? ESCAPE '\' OR error_type LIKE ? ESCAPE '\')`)
		args = append(args, s, s, s, s, s, s)
	}

	where := buildWhereClause(conditions)

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count audit log entries: %w", err)
	}

	dataQuery := `SELECT id, timestamp, duration_ns, model, provider, status_code, request_id,
		client_ip, method, path, stream, error_type, data
		FROM audit_logs` + where + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	dataArgs := append(append([]interface{}(nil), args...), limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		var ts string
		var streamInt int
		var dataJSON *string

		if err := rows.Scan(&e.ID, &ts, &e.DurationNs, &e.Model, &e.Provider, &e.StatusCode,
			&e.RequestID, &e.ClientIP, &e.Method, &e.Path, &streamInt, &e.ErrorType, &dataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}

		e.Stream = streamInt == 1
		e.Timestamp = parseSQLTimestamp(ts, e.ID)

		if dataJSON != nil && *dataJSON != "" {
			var data LogData
			if err := json.Unmarshal([]byte(*dataJSON), &data); err != nil {
				slog.Warn("failed to unmarshal audit data JSON", "id", e.ID, "error", err)
			} else {
				e.Data = &data
			}
		}

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log rows: %w", err)
	}

	return &LogListResult{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

func sqliteDateRangeConditions(params QueryParams) (conditions []string, args []interface{}) {
	if !params.StartDate.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, params.StartDate.UTC().Format("2006-01-02"))
	}
	if !params.EndDate.IsZero() {
		conditions = append(conditions, "timestamp < ?")
		args = append(args, params.EndDate.AddDate(0, 0, 1).UTC().Format("2006-01-02"))
	}
	return conditions, args
}

func parseSQLTimestamp(ts string, entryID string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", ts); err == nil {
		return t
	}

	slog.Warn("failed to parse audit timestamp", "id", entryID, "raw_timestamp", ts)
	return time.Time{}
}
