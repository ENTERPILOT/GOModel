package usage

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Logger provides async buffered logging with batch writes.
// It collects usage entries in a channel and flushes them to storage
// either when the buffer is full or at regular intervals.
type Logger struct {
	store         UsageStore
	config        Config
	buffer        chan *UsageEntry
	done          chan struct{}
	wg            sync.WaitGroup
	writes        sync.WaitGroup // tracks in-flight Write calls
	flushInterval time.Duration
	closed        atomic.Bool
}

// NewLogger creates a new async buffered Logger.
// The logger starts a background goroutine for flushing entries.
func NewLogger(store UsageStore, cfg Config) *Logger {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	l := &Logger{
		store:         store,
		config:        cfg,
		buffer:        make(chan *UsageEntry, cfg.BufferSize),
		done:          make(chan struct{}),
		flushInterval: cfg.FlushInterval,
	}

	l.wg.Add(1)
	go l.flushLoop()

	return l
}

// Write queues a usage entry for async writing.
// This method is non-blocking. If the buffer is full or the logger is closed,
// the entry is dropped and a warning is logged.
func (l *Logger) Write(entry *UsageEntry) {
	if entry == nil {
		return
	}

	// Check if logger is shut down to avoid sending on closed channel
	if l.closed.Load() {
		return
	}

	// Track this write to prevent Close from closing buffer while we're sending
	l.writes.Add(1)
	defer l.writes.Done()

	// Double-check after registering - Close() may have set closed between first check and Add(1)
	if l.closed.Load() {
		return
	}

	select {
	case l.buffer <- entry:
		// Entry queued successfully
	default:
		// Buffer full - drop entry and log warning
		requestID := entry.RequestID
		if requestID == "" {
			requestID = "unknown"
		}
		slog.Warn("usage log buffer full, dropping entry",
			"request_id", requestID,
			"model", entry.Model,
		)
	}
}

// Config returns the logger configuration
func (l *Logger) Config() Config {
	return l.config
}

// Close stops the logger and flushes remaining entries.
// This should be called during graceful shutdown.
// Close is idempotent - calling it multiple times is safe.
func (l *Logger) Close() error {
	// Make Close idempotent - if already closed, return immediately
	if l.closed.Swap(true) {
		return nil
	}

	// Wait for any in-flight Write calls to complete
	l.writes.Wait()

	// Signal the flush loop to stop
	close(l.done)

	// Wait for the flush loop to finish
	l.wg.Wait()

	// Close the store
	return l.store.Close()
}

// flushLoop runs in the background and periodically flushes the buffer.
func (l *Logger) flushLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	batch := make([]*UsageEntry, 0, BatchFlushThreshold)

	for {
		select {
		case entry := <-l.buffer:
			batch = append(batch, entry)
			// Flush when batch reaches threshold
			if len(batch) >= BatchFlushThreshold {
				l.flushBatch(batch)
				batch = make([]*UsageEntry, 0, BatchFlushThreshold)
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = make([]*UsageEntry, 0, BatchFlushThreshold)
			}

		case <-l.done:
			// Shutdown: close buffer to stop accepting entries
			// Note: l.closed is already set by Close() before sending on l.done
			close(l.buffer)
			// Drain remaining entries from buffer
			for entry := range l.buffer {
				batch = append(batch, entry)
			}
			// Final flush
			if len(batch) > 0 {
				l.flushBatch(batch)
			}
			// Flush the store
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := l.store.Flush(ctx); err != nil {
				slog.Error("failed to flush usage store", "error", err)
			}
			cancel()
			return
		}
	}
}

// flushBatch writes a batch of entries to the store.
func (l *Logger) flushBatch(batch []*UsageEntry) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := l.store.WriteBatch(ctx, batch); err != nil {
		slog.Error("failed to write usage batch",
			"error", err,
			"count", len(batch),
		)
	}
}

// NoopLogger is a logger that does nothing (used when usage tracking is disabled)
type NoopLogger struct{}

// Write does nothing
func (l *NoopLogger) Write(_ *UsageEntry) {}

// Config returns an empty config
func (l *NoopLogger) Config() Config {
	return Config{Enabled: false}
}

// Close does nothing
func (l *NoopLogger) Close() error {
	return nil
}

// LoggerInterface defines the interface for loggers (both real and noop)
type LoggerInterface interface {
	Write(entry *UsageEntry)
	Config() Config
	Close() error
}
