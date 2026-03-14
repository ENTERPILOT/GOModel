package aliases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound indicates a requested alias was not found.
var ErrNotFound = errors.New("alias not found")

// Store defines persistence operations for aliases.
type Store interface {
	List(ctx context.Context) ([]Alias, error)
	Get(ctx context.Context, name string) (*Alias, error)
	Upsert(ctx context.Context, alias Alias) error
	Delete(ctx context.Context, name string) error
	Close() error
}

type aliasScanner interface {
	Scan(dest ...any) error
}

type aliasRows interface {
	aliasScanner
	Next() bool
	Err() error
}

func normalizeName(name string) string {
	return strings.TrimSpace(name)
}

func normalizeAlias(alias Alias) (Alias, error) {
	alias.Name = normalizeName(alias.Name)
	alias.TargetModel = strings.TrimSpace(alias.TargetModel)
	alias.TargetProvider = strings.TrimSpace(alias.TargetProvider)
	alias.Description = strings.TrimSpace(alias.Description)

	if alias.Name == "" {
		return Alias{}, fmt.Errorf("alias name is required")
	}
	if strings.Contains(alias.Name, "/") {
		return Alias{}, fmt.Errorf("alias name %q must be unqualified", alias.Name)
	}
	if alias.TargetModel == "" {
		return Alias{}, fmt.Errorf("target_model is required")
	}
	if _, err := alias.TargetSelector(); err != nil {
		return Alias{}, fmt.Errorf("invalid target selector: %w", err)
	}
	return alias, nil
}

func collectAliases(rows aliasRows, scan func(aliasScanner) (Alias, error)) ([]Alias, error) {
	result := make([]Alias, 0)
	for rows.Next() {
		alias, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
