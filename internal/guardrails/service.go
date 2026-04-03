package guardrails

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type serviceSnapshot struct {
	definitions map[string]Definition
	order       []string
	registry    *Registry
}

// Service keeps reusable guardrails cached in memory and refreshes them from storage.
type Service struct {
	store Store

	mu       sync.RWMutex
	snapshot serviceSnapshot
}

// NewService creates a guardrail service backed by the provided store.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	return &Service{
		store: store,
		snapshot: serviceSnapshot{
			definitions: map[string]Definition{},
			order:       []string{},
			registry:    NewRegistry(),
		},
	}, nil
}

// Refresh reloads guardrails from storage and atomically swaps the in-memory snapshot.
func (s *Service) Refresh(ctx context.Context) error {
	definitions, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list guardrails: %w", err)
	}

	next := serviceSnapshot{
		definitions: make(map[string]Definition, len(definitions)),
		order:       make([]string, 0, len(definitions)),
		registry:    NewRegistry(),
	}
	for _, definition := range definitions {
		normalized, err := normalizeDefinition(definition)
		if err != nil {
			return fmt.Errorf("load guardrail %q: %w", definition.Name, err)
		}
		instance, descriptor, err := buildDefinition(normalized)
		if err != nil {
			return fmt.Errorf("load guardrail %q: %w", normalized.Name, err)
		}
		if err := next.registry.Register(instance, descriptor); err != nil {
			return fmt.Errorf("register guardrail %q: %w", normalized.Name, err)
		}
		next.definitions[normalized.Name] = normalized
		next.order = append(next.order, normalized.Name)
	}
	sort.Strings(next.order)

	s.mu.Lock()
	s.snapshot = next
	s.mu.Unlock()
	return nil
}

// UpsertDefinitions validates and upserts a definition set, then refreshes once.
func (s *Service) UpsertDefinitions(ctx context.Context, definitions []Definition) error {
	if s == nil || len(definitions) == 0 {
		return nil
	}

	for _, definition := range definitions {
		normalized, err := normalizeDefinition(definition)
		if err != nil {
			return err
		}
		if err := s.store.Upsert(ctx, normalized); err != nil {
			return fmt.Errorf("upsert guardrail %q: %w", normalized.Name, err)
		}
	}
	return s.Refresh(ctx)
}

// List returns all cached guardrail definitions sorted by name.
func (s *Service) List() []Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Definition, 0, len(s.snapshot.order))
	for _, name := range s.snapshot.order {
		result = append(result, cloneDefinition(s.snapshot.definitions[name]))
	}
	return result
}

// ListViews returns all cached guardrail definitions with lightweight summaries.
func (s *Service) ListViews() []View {
	definitions := s.List()
	views := make([]View, 0, len(definitions))
	for _, definition := range definitions {
		views = append(views, ViewFromDefinition(definition))
	}
	return views
}

// Get returns one cached guardrail by name.
func (s *Service) Get(name string) (*Definition, bool) {
	name = normalizeDefinitionName(name)
	if name == "" {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, ok := s.snapshot.definitions[name]
	if !ok {
		return nil, false
	}
	copy := cloneDefinition(definition)
	return &copy, true
}

// Upsert validates and stores a guardrail definition, then refreshes the snapshot.
func (s *Service) Upsert(ctx context.Context, definition Definition) error {
	normalized, err := normalizeDefinition(definition)
	if err != nil {
		return err
	}
	if err := s.store.Upsert(ctx, normalized); err != nil {
		return fmt.Errorf("upsert guardrail: %w", err)
	}
	if err := s.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh guardrails: %w", err)
	}
	return nil
}

// Delete removes a guardrail definition from storage and refreshes the snapshot.
func (s *Service) Delete(ctx context.Context, name string) error {
	name = normalizeDefinitionName(name)
	if name == "" {
		return newValidationError("guardrail name is required", nil)
	}
	if err := s.store.Delete(ctx, name); err != nil {
		return fmt.Errorf("delete guardrail: %w", err)
	}
	if err := s.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh guardrails: %w", err)
	}
	return nil
}

// TypeDefinitions returns the supported guardrail type schemas.
func (s *Service) TypeDefinitions() []TypeDefinition {
	return TypeDefinitions()
}

// Len returns the number of loaded guardrails.
func (s *Service) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshot.order)
}

// Names returns the loaded guardrail names in sorted order.
func (s *Service) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.snapshot.order...)
}

// BuildPipeline resolves named steps through the current in-memory guardrail registry.
func (s *Service) BuildPipeline(steps []StepReference) (*Pipeline, string, error) {
	if len(steps) == 0 {
		return nil, "", nil
	}

	s.mu.RLock()
	registry := s.snapshot.registry
	s.mu.RUnlock()
	if registry == nil {
		return nil, "", fmt.Errorf("guardrail catalog is not loaded")
	}
	return registry.BuildPipeline(steps)
}

// StartBackgroundRefresh periodically reloads guardrails from storage until stopped.
// Callers can observe background failures on the returned error channel.
func (s *Service) StartBackgroundRefresh(parent context.Context, interval time.Duration) (func(), <-chan error) {
	if parent == nil {
		errs := make(chan error)
		close(errs)
		return func() {}, errs
	}
	if interval <= 0 {
		interval = time.Minute
	}

	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	errs := make(chan error, 1)
	var once sync.Once

	go func() {
		defer close(done)
		defer close(errs)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshCtx, refreshCancel := context.WithTimeout(ctx, 30*time.Second)
				if err := s.Refresh(refreshCtx); err != nil {
					select {
					case errs <- fmt.Errorf("refresh guardrails: %w", err):
					default:
					}
				}
				refreshCancel()
			}
		}
	}()

	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}, errs
}
