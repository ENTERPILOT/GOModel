package batch

import (
	"context"
	"testing"

	"gomodel/internal/core"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	b := &core.BatchResponse{
		ID:        "batch-1",
		Object:    "batch",
		Status:    "completed",
		CreatedAt: 100,
		Results: []core.BatchResultItem{
			{Index: 0, StatusCode: 200},
		},
	}

	if err := store.Create(ctx, b); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.Get(ctx, "batch-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != b.ID {
		t.Fatalf("id = %q, want %q", got.ID, b.ID)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(got.Results))
	}

	got.Status = "cancelled"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	got2, err := store.Get(ctx, "batch-1")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got2.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", got2.Status)
	}
}

func TestMemoryStoreListAfter(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	inputs := []*core.BatchResponse{
		{ID: "batch-c", CreatedAt: 3, Status: "completed"},
		{ID: "batch-b", CreatedAt: 2, Status: "completed"},
		{ID: "batch-a", CreatedAt: 1, Status: "completed"},
	}
	for _, b := range inputs {
		b.Object = "batch"
		if err := store.Create(ctx, b); err != nil {
			t.Fatalf("create %s: %v", b.ID, err)
		}
	}

	list, err := store.List(ctx, 2, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].ID != "batch-c" || list[1].ID != "batch-b" {
		t.Fatalf("unexpected order: %s, %s", list[0].ID, list[1].ID)
	}

	next, err := store.List(ctx, 2, "batch-b")
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if len(next) != 1 || next[0].ID != "batch-a" {
		t.Fatalf("unexpected after result: %+v", next)
	}
}
