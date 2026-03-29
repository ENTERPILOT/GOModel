package usage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteReaderSummary_IncludesFractionalStartBoundaryAndExcludesFractionalEndBoundary(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}

	ctx := context.Background()
	err = store.WriteBatch(ctx, []*UsageEntry{
		{
			ID:           "start-boundary",
			RequestID:    "req-start",
			ProviderID:   "provider-start",
			Timestamp:    time.Date(2026, 1, 15, 23, 0, 0, 123_000_000, time.UTC),
			Model:        "gpt-5",
			Provider:     "openai",
			Endpoint:     "/v1/chat/completions",
			TotalTokens:  10,
			OutputTokens: 10,
		},
		{
			ID:           "inside-range",
			RequestID:    "req-inside",
			ProviderID:   "provider-inside",
			Timestamp:    time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC),
			Model:        "gpt-5",
			Provider:     "openai",
			Endpoint:     "/v1/chat/completions",
			TotalTokens:  20,
			OutputTokens: 20,
		},
		{
			ID:           "after-end-boundary",
			RequestID:    "req-after",
			ProviderID:   "provider-after",
			Timestamp:    time.Date(2026, 1, 16, 23, 0, 0, 123_000_000, time.UTC),
			Model:        "gpt-5",
			Provider:     "openai",
			Endpoint:     "/v1/chat/completions",
			TotalTokens:  999,
			OutputTokens: 999,
		},
	})
	if err != nil {
		t.Fatalf("failed to seed usage entries: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create sqlite reader: %v", err)
	}

	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	summary, err := reader.GetSummary(ctx, UsageQueryParams{
		StartDate: time.Date(2026, 1, 16, 0, 0, 0, 0, location),
		EndDate:   time.Date(2026, 1, 16, 0, 0, 0, 0, location),
		TimeZone:  "Europe/Warsaw",
	})
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}

	if summary.TotalRequests != 2 {
		t.Fatalf("expected 2 requests in range, got %d", summary.TotalRequests)
	}
	if summary.TotalTokens != 30 {
		t.Fatalf("expected 30 total tokens in range, got %d", summary.TotalTokens)
	}
}

func TestSQLiteReaderGetDailyUsage_GroupsAcrossDSTTransitionInConfiguredTimeZone(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}

	ctx := context.Background()
	err = store.WriteBatch(ctx, []*UsageEntry{
		{
			ID:           "before-dst-switch",
			RequestID:    "req-before",
			ProviderID:   "provider-before",
			Timestamp:    time.Date(2026, 3, 28, 23, 30, 0, 0, time.UTC),
			Model:        "gpt-5",
			Provider:     "openai",
			Endpoint:     "/v1/chat/completions",
			TotalTokens:  10,
			OutputTokens: 10,
		},
		{
			ID:           "after-dst-switch",
			RequestID:    "req-after",
			ProviderID:   "provider-after",
			Timestamp:    time.Date(2026, 3, 29, 1, 30, 0, 0, time.UTC),
			Model:        "gpt-5",
			Provider:     "openai",
			Endpoint:     "/v1/chat/completions",
			TotalTokens:  20,
			OutputTokens: 20,
		},
	})
	if err != nil {
		t.Fatalf("failed to seed usage entries: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create sqlite reader: %v", err)
	}

	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	daily, err := reader.GetDailyUsage(ctx, UsageQueryParams{
		StartDate: time.Date(2026, 3, 29, 0, 0, 0, 0, location),
		EndDate:   time.Date(2026, 3, 29, 0, 0, 0, 0, location),
		Interval:  "daily",
		TimeZone:  "Europe/Warsaw",
	})
	if err != nil {
		t.Fatalf("GetDailyUsage returned error: %v", err)
	}

	if len(daily) != 1 {
		t.Fatalf("expected 1 grouped period, got %d", len(daily))
	}
	if daily[0].Date != "2026-03-29" {
		t.Fatalf("expected grouped date %q, got %q", "2026-03-29", daily[0].Date)
	}
	if daily[0].Requests != 2 {
		t.Fatalf("expected 2 requests in grouped period, got %d", daily[0].Requests)
	}
	if daily[0].TotalTokens != 30 {
		t.Fatalf("expected 30 total tokens in grouped period, got %d", daily[0].TotalTokens)
	}
}

func TestSQLiteReaderSummary_IncludesSpaceSeparatedBoundaryTimestamp(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	if _, err := NewSQLiteStore(db, 0); err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		INSERT INTO usage (
			id, request_id, provider_id, timestamp, model, provider, endpoint,
			input_tokens, output_tokens, total_tokens,
			input_cost, output_cost, total_cost, costs_calculation_caveat
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"space-boundary",
		"req-space",
		"provider-space",
		"2026-01-15 23:00:00+00:00",
		"gpt-5",
		"openai",
		"/v1/chat/completions",
		0,
		10,
		10,
		0.0,
		0.0,
		0.0,
		"",
	)
	if err != nil {
		t.Fatalf("failed to seed mixed-format usage entry: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create sqlite reader: %v", err)
	}

	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	summary, err := reader.GetSummary(ctx, UsageQueryParams{
		StartDate: time.Date(2026, 1, 16, 0, 0, 0, 0, location),
		EndDate:   time.Date(2026, 1, 16, 0, 0, 0, 0, location),
		TimeZone:  "Europe/Warsaw",
	})
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}

	if summary.TotalRequests != 1 {
		t.Fatalf("expected 1 request in range, got %d", summary.TotalRequests)
	}
	if summary.TotalTokens != 10 {
		t.Fatalf("expected 10 total tokens in range, got %d", summary.TotalTokens)
	}
}

func TestSQLiteReaderGroupingRange_UsesNormalizedTimestampExtrema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	if _, err := NewSQLiteStore(db, 0); err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		INSERT INTO usage (
			id, request_id, provider_id, timestamp, model, provider, endpoint,
			input_tokens, output_tokens, total_tokens,
			input_cost, output_cost, total_cost, costs_calculation_caveat
		) VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"earliest-rfc3339",
		"req-early",
		"provider-early",
		"2026-03-29T00:30:00Z",
		"gpt-5",
		"openai",
		"/v1/chat/completions",
		0,
		10,
		10,
		0.0,
		0.0,
		0.0,
		"",
		"latest-space",
		"req-late",
		"provider-late",
		"2026-03-29 23:30:00+00:00",
		"gpt-5",
		"openai",
		"/v1/chat/completions",
		0,
		20,
		20,
		0.0,
		0.0,
		0.0,
		"",
	)
	if err != nil {
		t.Fatalf("failed to seed mixed-format usage entries: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create sqlite reader: %v", err)
	}

	start, end, ok, err := reader.sqliteGroupingRange(ctx, UsageQueryParams{
		TimeZone: "Europe/Warsaw",
	})
	if err != nil {
		t.Fatalf("sqliteGroupingRange returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected sqliteGroupingRange to detect a usage range")
	}

	expectedStart := time.Date(2026, 3, 29, 0, 30, 0, 0, time.UTC)
	expectedEnd := time.Date(2026, 3, 29, 23, 30, 1, 0, time.UTC)
	if !start.Equal(expectedStart) {
		t.Fatalf("expected range start %s, got %s", expectedStart, start)
	}
	if !end.Equal(expectedEnd) {
		t.Fatalf("expected range end %s, got %s", expectedEnd, end)
	}
}
