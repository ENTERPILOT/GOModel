package requestflow

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a stored flow resource does not exist.
var ErrNotFound = errors.New("request flow entry not found")

// Store persists editable definitions and execution history.
type Store interface {
	ListDefinitions(ctx context.Context) ([]*Definition, error)
	SaveDefinition(ctx context.Context, def *Definition) error
	DeleteDefinition(ctx context.Context, id string) error
	WriteExecutionBatch(ctx context.Context, entries []*Execution) error
	ListExecutions(ctx context.Context, params ExecutionQueryParams) (*ExecutionLogResult, error)
	Flush(ctx context.Context) error
	Close() error
}
