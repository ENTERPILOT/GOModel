package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"gomodel/internal/usage"
)

func newContext(query string) echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test?"+query, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec)
}

func TestParseUsageParams_DaysDefault(t *testing.T) {
	c := newContext("")
	params := parseUsageParams(c)

	if params.Interval != "daily" {
		t.Errorf("expected interval 'daily', got %q", params.Interval)
	}

	today := time.Now().UTC()
	expectedEnd := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	expectedStart := expectedEnd.AddDate(0, 0, -29)

	if !params.EndDate.Equal(expectedEnd) {
		t.Errorf("expected end date %v, got %v", expectedEnd, params.EndDate)
	}
	if !params.StartDate.Equal(expectedStart) {
		t.Errorf("expected start date %v, got %v", expectedStart, params.StartDate)
	}
}

func TestParseUsageParams_DaysExplicit(t *testing.T) {
	c := newContext("days=7")
	params := parseUsageParams(c)

	today := time.Now().UTC()
	expectedEnd := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	expectedStart := expectedEnd.AddDate(0, 0, -6)

	if !params.StartDate.Equal(expectedStart) {
		t.Errorf("expected start date %v, got %v", expectedStart, params.StartDate)
	}
	if !params.EndDate.Equal(expectedEnd) {
		t.Errorf("expected end date %v, got %v", expectedEnd, params.EndDate)
	}
}

func TestParseUsageParams_StartAndEndDate(t *testing.T) {
	c := newContext("start_date=2026-01-01&end_date=2026-01-31")
	params := parseUsageParams(c)

	expectedStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedEnd := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	if !params.StartDate.Equal(expectedStart) {
		t.Errorf("expected start date %v, got %v", expectedStart, params.StartDate)
	}
	if !params.EndDate.Equal(expectedEnd) {
		t.Errorf("expected end date %v, got %v", expectedEnd, params.EndDate)
	}
}

func TestParseUsageParams_OnlyStartDate(t *testing.T) {
	c := newContext("start_date=2026-01-15")
	params := parseUsageParams(c)

	expectedStart := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	today := time.Now().UTC()
	expectedEnd := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	if !params.StartDate.Equal(expectedStart) {
		t.Errorf("expected start date %v, got %v", expectedStart, params.StartDate)
	}
	if !params.EndDate.Equal(expectedEnd) {
		t.Errorf("expected end date %v, got %v", expectedEnd, params.EndDate)
	}
}

func TestParseUsageParams_OnlyEndDate(t *testing.T) {
	c := newContext("end_date=2026-02-10")
	params := parseUsageParams(c)

	expectedEnd := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	expectedStart := expectedEnd.AddDate(0, 0, -29)

	if !params.StartDate.Equal(expectedStart) {
		t.Errorf("expected start date %v, got %v", expectedStart, params.StartDate)
	}
	if !params.EndDate.Equal(expectedEnd) {
		t.Errorf("expected end date %v, got %v", expectedEnd, params.EndDate)
	}
}

func TestParseUsageParams_InvalidDates(t *testing.T) {
	c := newContext("start_date=invalid&end_date=also-invalid")
	params := parseUsageParams(c)

	// Should fall back to days=30 default
	today := time.Now().UTC()
	expectedEnd := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	expectedStart := expectedEnd.AddDate(0, 0, -29)

	if !params.StartDate.Equal(expectedStart) {
		t.Errorf("expected start date %v, got %v", expectedStart, params.StartDate)
	}
	if !params.EndDate.Equal(expectedEnd) {
		t.Errorf("expected end date %v, got %v", expectedEnd, params.EndDate)
	}
}

func TestParseUsageParams_IntervalWeekly(t *testing.T) {
	c := newContext("interval=weekly")
	params := parseUsageParams(c)

	if params.Interval != "weekly" {
		t.Errorf("expected interval 'weekly', got %q", params.Interval)
	}
}

func TestParseUsageParams_IntervalMonthly(t *testing.T) {
	c := newContext("interval=monthly")
	params := parseUsageParams(c)

	if params.Interval != "monthly" {
		t.Errorf("expected interval 'monthly', got %q", params.Interval)
	}
}

func TestParseUsageParams_IntervalInvalid(t *testing.T) {
	c := newContext("interval=hourly")
	params := parseUsageParams(c)

	if params.Interval != "daily" {
		t.Errorf("expected default interval 'daily', got %q", params.Interval)
	}
}

func TestParseUsageParams_IntervalEmpty(t *testing.T) {
	c := newContext("")
	params := parseUsageParams(c)

	if params.Interval != "daily" {
		t.Errorf("expected default interval 'daily', got %q", params.Interval)
	}
}

// Ensure usage.UsageQueryParams is the type used (compile check)
var _ = func() usage.UsageQueryParams {
	return usage.UsageQueryParams{
		StartDate: time.Time{},
		EndDate:   time.Time{},
		Interval:  "daily",
	}
}
