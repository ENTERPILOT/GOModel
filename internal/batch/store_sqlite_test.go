package batch

import (
	"context"
	"path/filepath"
	"testing"

	"gomodel/internal/core"
	"gomodel/internal/storage"
)

func TestSQLiteStoreLifecycle(t *testing.T) {
	st, err := storage.NewSQLite(storage.SQLiteConfig{Path: filepath.Join(t.TempDir(), "batches.db")})
	if err != nil {
		t.Fatalf("new sqlite storage: %v", err)
	}
	defer st.Close()

	store, err := NewSQLiteStore(st.SQLiteDB())
	if err != nil {
		t.Fatalf("new sqlite batch store: %v", err)
	}

	ctx := context.Background()
	b := &core.BatchResponse{
		ID:        "batch-sql-1",
		Object:    "batch",
		Status:    "completed",
		CreatedAt: 123,
		RequestCounts: core.BatchRequestCounts{
			Total:     1,
			Completed: 1,
		},
		Results: []core.BatchResultItem{
			{Index: 0, StatusCode: 200, URL: "/v1/chat/completions"},
		},
	}

	if err := store.Create(ctx, b); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.Get(ctx, b.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != b.ID {
		t.Fatalf("id = %q, want %q", got.ID, b.ID)
	}
	if got.RequestCounts.Total != 1 {
		t.Fatalf("request_counts.total = %d, want 1", got.RequestCounts.Total)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(got.Results))
	}

	got.Status = "cancelled"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	got2, err := store.Get(ctx, b.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got2.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", got2.Status)
	}
}
