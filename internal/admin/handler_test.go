package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/usage"
)

// mockUsageReader implements usage.UsageReader for testing.
type mockUsageReader struct {
	summary    *usage.UsageSummary
	daily      []usage.DailyUsage
	summaryErr error
	dailyErr   error
}

func (m *mockUsageReader) GetSummary(_ context.Context, _ usage.UsageQueryParams) (*usage.UsageSummary, error) {
	if m.summaryErr != nil {
		return nil, m.summaryErr
	}
	return m.summary, nil
}

func (m *mockUsageReader) GetDailyUsage(_ context.Context, _ usage.UsageQueryParams) ([]usage.DailyUsage, error) {
	if m.dailyErr != nil {
		return nil, m.dailyErr
	}
	return m.daily, nil
}

// handlerMockProvider implements core.Provider for ListModels registry testing.
type handlerMockProvider struct {
	models *core.ModelsResponse
}

func (m *handlerMockProvider) ChatCompletion(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
	return nil, nil
}
func (m *handlerMockProvider) StreamChatCompletion(_ context.Context, _ *core.ChatRequest) (io.ReadCloser, error) {
	return nil, nil
}
func (m *handlerMockProvider) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	return m.models, nil
}
func (m *handlerMockProvider) Responses(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return nil, nil
}
func (m *handlerMockProvider) StreamResponses(_ context.Context, _ *core.ResponsesRequest) (io.ReadCloser, error) {
	return nil, nil
}

func newHandlerContext(path string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// --- UsageSummary handler tests ---

func TestUsageSummary_NilReader(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/summary")

	if err := h.UsageSummary(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var summary usage.UsageSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if summary.TotalRequests != 0 || summary.TotalInput != 0 || summary.TotalOutput != 0 || summary.TotalTokens != 0 {
		t.Errorf("expected zeroed summary, got %+v", summary)
	}
}

func TestUsageSummary_Success(t *testing.T) {
	reader := &mockUsageReader{
		summary: &usage.UsageSummary{
			TotalRequests: 42,
			TotalInput:    1000,
			TotalOutput:   500,
			TotalTokens:   1500,
		},
	}
	h := NewHandler(reader, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/summary?days=30")

	if err := h.UsageSummary(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var summary usage.UsageSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if summary.TotalRequests != 42 {
		t.Errorf("expected 42 requests, got %d", summary.TotalRequests)
	}
	if summary.TotalTokens != 1500 {
		t.Errorf("expected 1500 total tokens, got %d", summary.TotalTokens)
	}
}

func TestUsageSummary_GatewayError(t *testing.T) {
	reader := &mockUsageReader{
		summaryErr: core.NewProviderError("test", http.StatusBadGateway, "upstream failed", nil),
	}
	h := NewHandler(reader, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/summary")

	if err := h.UsageSummary(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsString(body, "provider_error") {
		t.Errorf("expected provider_error in body, got: %s", body)
	}
}

func TestUsageSummary_GenericError(t *testing.T) {
	reader := &mockUsageReader{
		summaryErr: errors.New("database connection lost"),
	}
	h := NewHandler(reader, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/summary")

	if err := h.UsageSummary(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsString(body, "internal_error") {
		t.Errorf("expected internal_error in body, got: %s", body)
	}
	if containsString(body, "database connection lost") {
		t.Errorf("original error message should be hidden, got: %s", body)
	}
	if !containsString(body, "an unexpected error occurred") {
		t.Errorf("expected generic message, got: %s", body)
	}
}

// --- DailyUsage handler tests ---

func TestDailyUsage_NilReader(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/daily")

	if err := h.DailyUsage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	// Should be [] not null
	if rec.Body.String() != "[]\n" {
		t.Errorf("expected empty JSON array, got: %q", rec.Body.String())
	}
}

func TestDailyUsage_Success(t *testing.T) {
	reader := &mockUsageReader{
		daily: []usage.DailyUsage{
			{Date: "2026-02-01", Requests: 10, InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
			{Date: "2026-02-02", Requests: 20, InputTokens: 200, OutputTokens: 100, TotalTokens: 300},
		},
	}
	h := NewHandler(reader, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/daily?days=7")

	if err := h.DailyUsage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var daily []usage.DailyUsage
	if err := json.Unmarshal(rec.Body.Bytes(), &daily); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(daily) != 2 {
		t.Errorf("expected 2 entries, got %d", len(daily))
	}
}

func TestDailyUsage_NilResult(t *testing.T) {
	reader := &mockUsageReader{
		daily: nil, // reader returns nil slice
	}
	h := NewHandler(reader, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/daily")

	if err := h.DailyUsage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	// Should be [] not null
	if rec.Body.String() != "[]\n" {
		t.Errorf("expected empty JSON array, got: %q", rec.Body.String())
	}
}

func TestDailyUsage_Error(t *testing.T) {
	reader := &mockUsageReader{
		dailyErr: core.NewRateLimitError("test", "too many requests"),
	}
	h := NewHandler(reader, nil)
	c, rec := newHandlerContext("/admin/api/v1/usage/daily")

	if err := h.DailyUsage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsString(body, "rate_limit_error") {
		t.Errorf("expected rate_limit_error in body, got: %s", body)
	}
}

// --- ListModels handler tests ---

func TestListModels_NilRegistry(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := newHandlerContext("/admin/api/v1/models")

	if err := h.ListModels(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "[]\n" {
		t.Errorf("expected empty JSON array, got: %q", rec.Body.String())
	}
}

func TestListModels_WithModels(t *testing.T) {
	registry := providers.NewModelRegistry()
	mock := &handlerMockProvider{
		models: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4", Object: "model", OwnedBy: "openai"},
				{ID: "claude-3", Object: "model", OwnedBy: "anthropic"},
			},
		},
	}
	registry.RegisterProviderWithType(mock, "test")
	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to initialize registry: %v", err)
	}

	h := NewHandler(nil, registry)
	c, rec := newHandlerContext("/admin/api/v1/models")

	if err := h.ListModels(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var models []providers.ModelWithProvider
	if err := json.Unmarshal(rec.Body.Bytes(), &models); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	// Should be sorted by model ID
	if models[0].Model.ID != "claude-3" {
		t.Errorf("expected first model to be claude-3, got %s", models[0].Model.ID)
	}
	if models[1].Model.ID != "gpt-4" {
		t.Errorf("expected second model to be gpt-4, got %s", models[1].Model.ID)
	}
	if models[0].ProviderType != "test" {
		t.Errorf("expected provider type 'test', got %s", models[0].ProviderType)
	}
}

func TestListModels_EmptyRegistry(t *testing.T) {
	// A registry with no providers initialized — ListModelsWithProvider returns nil
	registry := providers.NewModelRegistry()

	h := NewHandler(nil, registry)
	c, rec := newHandlerContext("/admin/api/v1/models")

	if err := h.ListModels(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "[]\n" {
		t.Errorf("expected empty JSON array, got: %q", rec.Body.String())
	}
}

// --- handleError tests ---

func TestHandleError_GatewayErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "provider_error → 502",
			err:            core.NewProviderError("test", http.StatusBadGateway, "upstream error", nil),
			expectedStatus: http.StatusBadGateway,
			expectedType:   "provider_error",
		},
		{
			name:           "rate_limit_error → 429",
			err:            core.NewRateLimitError("test", "rate limited"),
			expectedStatus: http.StatusTooManyRequests,
			expectedType:   "rate_limit_error",
		},
		{
			name:           "invalid_request_error → 400",
			err:            core.NewInvalidRequestError("bad input", nil),
			expectedStatus: http.StatusBadRequest,
			expectedType:   "invalid_request_error",
		},
		{
			name:           "authentication_error → 401",
			err:            core.NewAuthenticationError("test", "invalid key"),
			expectedStatus: http.StatusUnauthorized,
			expectedType:   "authentication_error",
		},
		{
			name:           "not_found_error → 404",
			err:            core.NewNotFoundError("model not found"),
			expectedStatus: http.StatusNotFound,
			expectedType:   "not_found_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newHandlerContext("/test")

			if err := handleError(c, tt.err); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			body := rec.Body.String()
			if !containsString(body, tt.expectedType) {
				t.Errorf("expected %s in body, got: %s", tt.expectedType, body)
			}
		})
	}
}

func TestHandleError_UnexpectedError(t *testing.T) {
	c, rec := newHandlerContext("/test")

	if err := handleError(c, errors.New("something broke")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsString(body, "an unexpected error occurred") {
		t.Errorf("expected generic message, got: %s", body)
	}
	if containsString(body, "something broke") {
		t.Errorf("original error should be hidden, got: %s", body)
	}
}

// containsString is a small helper to check substring presence.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

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
