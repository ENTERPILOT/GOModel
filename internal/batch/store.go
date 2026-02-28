// Package batch provides persistence for OpenAI-compatible batch lifecycle endpoints.
package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gomodel/internal/core"
)

// ErrNotFound indicates a requested batch was not found.
var ErrNotFound = errors.New("batch not found")

// Store defines persistence operations for batch lifecycle APIs.
type Store interface {
	Create(ctx context.Context, batch *core.BatchResponse) error
	Get(ctx context.Context, id string) (*core.BatchResponse, error)
	List(ctx context.Context, limit int, after string) ([]*core.BatchResponse, error)
	Update(ctx context.Context, batch *core.BatchResponse) error
	Close() error
}

func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 101:
		return 101
	default:
		return limit
	}
}

func cloneBatch(src *core.BatchResponse) (*core.BatchResponse, error) {
	if src == nil {
		return nil, fmt.Errorf("batch is nil")
	}
	b, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}
	var dst core.BatchResponse
	if err := json.Unmarshal(b, &dst); err != nil {
		return nil, fmt.Errorf("unmarshal batch: %w", err)
	}
	return &dst, nil
}

func serializeBatch(batch *core.BatchResponse) ([]byte, error) {
	if batch == nil {
		return nil, fmt.Errorf("batch is nil")
	}
	b, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}
	return b, nil
}

func deserializeBatch(raw []byte) (*core.BatchResponse, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty batch payload")
	}
	var batch core.BatchResponse
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, fmt.Errorf("unmarshal batch: %w", err)
	}
	return &batch, nil
}
