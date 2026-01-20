package usage

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockStore implements UsageStore for testing
type mockStore struct {
	entries []*UsageEntry
	mu      sync.Mutex
	closed  bool
}

func (m *mockStore) WriteBatch(ctx context.Context, entries []*UsageEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entries...)
	return nil
}

func (m *mockStore) Flush(ctx context.Context) error {
	return nil
}

func (m *mockStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockStore) getEntries() []*UsageEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*UsageEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

func TestLogger(t *testing.T) {
	store := &mockStore{}
	cfg := Config{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: 100 * time.Millisecond,
	}

	logger := NewLogger(store, cfg)

	// Write some entries
	for i := 0; i < 5; i++ {
		logger.Write(&UsageEntry{
			ID:           "test-" + string(rune('0'+i)),
			RequestID:    "req-" + string(rune('0'+i)),
			Model:        "gpt-4",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		})
	}

	// Wait for flush interval
	time.Sleep(200 * time.Millisecond)

	// Check entries were written
	entries := store.getEntries()
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}

	// Close logger
	if err := logger.Close(); err != nil {
		t.Errorf("logger close error: %v", err)
	}

	// Verify store was closed
	if !store.closed {
		t.Error("store should be closed")
	}
}

func TestLoggerClose(t *testing.T) {
	store := &mockStore{}
	cfg := Config{
		Enabled:       true,
		BufferSize:    1000,
		FlushInterval: 1 * time.Hour, // Long interval so flush is triggered by close
	}

	logger := NewLogger(store, cfg)

	// Write entries
	for i := 0; i < 10; i++ {
		logger.Write(&UsageEntry{
			ID:        "test-" + string(rune('0'+i)),
			RequestID: "req-" + string(rune('0'+i)),
		})
	}

	// Close immediately - should flush pending entries
	if err := logger.Close(); err != nil {
		t.Errorf("logger close error: %v", err)
	}

	// Verify all entries were flushed
	entries := store.getEntries()
	if len(entries) != 10 {
		t.Errorf("expected 10 entries after close, got %d", len(entries))
	}
}

func TestNoopLogger(t *testing.T) {
	logger := &NoopLogger{}

	// Write should not panic
	logger.Write(&UsageEntry{ID: "test"})

	// Config should show disabled
	cfg := logger.Config()
	if cfg.Enabled {
		t.Error("NoopLogger should report disabled")
	}

	// Close should not error
	if err := logger.Close(); err != nil {
		t.Errorf("NoopLogger close error: %v", err)
	}
}

func TestLoggerBufferFull(t *testing.T) {
	store := &mockStore{}
	cfg := Config{
		Enabled:       true,
		BufferSize:    2, // Very small buffer
		FlushInterval: 1 * time.Hour,
	}

	logger := NewLogger(store, cfg)
	defer logger.Close()

	// Track dropped entries via atomic counter
	var written atomic.Int32

	// Try to write more than buffer size
	for i := 0; i < 10; i++ {
		logger.Write(&UsageEntry{ID: "test-" + string(rune('0'+i))})
		written.Add(1)
	}

	// Some entries may be dropped
	// Just verify it doesn't panic/deadlock
}
