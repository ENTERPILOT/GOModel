package responsestore

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore keeps response snapshots in process memory.
// Data survives across requests but not process restarts.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]*StoredResponse
}

// NewMemoryStore creates an empty in-memory response store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]*StoredResponse),
	}
}

// Create stores a new response snapshot.
func (s *MemoryStore) Create(_ context.Context, response *StoredResponse) error {
	if response == nil || response.Response == nil || response.Response.ID == "" {
		return fmt.Errorf("response id is required")
	}

	c, err := cloneResponse(response)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[c.Response.ID]; exists {
		return fmt.Errorf("response already exists: %s", c.Response.ID)
	}
	s.items[c.Response.ID] = c
	return nil
}

// Get retrieves one response snapshot by id.
func (s *MemoryStore) Get(_ context.Context, id string) (*StoredResponse, error) {
	s.mu.RLock()
	response, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return cloneResponse(response)
}

// Update replaces an existing response snapshot.
func (s *MemoryStore) Update(_ context.Context, response *StoredResponse) error {
	if response == nil || response.Response == nil || response.Response.ID == "" {
		return fmt.Errorf("response id is required")
	}
	c, err := cloneResponse(response)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[c.Response.ID]; !exists {
		return ErrNotFound
	}
	s.items[c.Response.ID] = c
	return nil
}

// Delete removes one response snapshot by id.
func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[id]; !exists {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

// Close releases resources (no-op for memory store).
func (s *MemoryStore) Close() error {
	return nil
}
