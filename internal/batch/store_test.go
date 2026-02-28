package batch

import (
	"context"
	"strings"
	"testing"

	"gomodel/internal/core"
)

func TestSerializeBatchValidatesID(t *testing.T) {
	t.Run("nil batch", func(t *testing.T) {
		_, err := serializeBatch(nil)
		if err == nil {
			t.Fatal("expected error for nil batch")
		}
	})

	t.Run("empty batch id", func(t *testing.T) {
		_, err := serializeBatch(&core.BatchResponse{})
		if err == nil {
			t.Fatal("expected error for empty batch ID")
		}
		if !strings.Contains(err.Error(), "batch ID is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNewRequiresConfig(t *testing.T) {
	_, err := New(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
