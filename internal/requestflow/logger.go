package requestflow

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const batchFlushThreshold = 100

// LoggerConfig controls async execution logging.
type LoggerConfig struct {
	Enabled       bool
	BufferSize    int
	FlushInterval time.Duration
}

// Logger queues execution entries for batched persistence.
type Logger struct {
	store         Store
	config        LoggerConfig
	buffer        chan *Execution
	done          chan struct{}
	wg            sync.WaitGroup
	writes        sync.WaitGroup
	flushInterval time.Duration
	closed        atomic.Bool
}

// NewLogger creates a new async execution logger.
func NewLogger(store Store, cfg LoggerConfig) *Logger {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	l := &Logger{
		store:         store,
		config:        cfg,
		buffer:        make(chan *Execution, cfg.BufferSize),
		done:          make(chan struct{}),
		flushInterval: cfg.FlushInterval,
	}
	l.wg.Add(1)
	go l.flushLoop()
	return l
}

// Write queues an execution for persistence.
func (l *Logger) Write(entry *Execution) {
	if entry == nil || l == nil || !l.config.Enabled {
		return
	}
	if l.closed.Load() {
		return
	}
	l.writes.Add(1)
	defer l.writes.Done()
	if l.closed.Load() {
		return
	}
	select {
	case l.buffer <- entry:
	default:
		slog.Warn("request flow buffer full, dropping execution", "request_id", entry.RequestID, "model", entry.Model)
	}
}

// Close flushes buffered entries.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	if l.closed.Swap(true) {
		return nil
	}
	l.writes.Wait()
	close(l.done)
	l.wg.Wait()
	return l.store.Close()
}

func (l *Logger) flushLoop() {
	defer l.wg.Done()
	batch := make([]*Execution, 0, batchFlushThreshold)
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case entry := <-l.buffer:
			batch = append(batch, entry)
			if len(batch) >= batchFlushThreshold {
				l.flushBatch(batch)
				batch = make([]*Execution, 0, batchFlushThreshold)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = make([]*Execution, 0, batchFlushThreshold)
			}
		case <-l.done:
			for {
				select {
				case entry := <-l.buffer:
					batch = append(batch, entry)
				default:
					goto drainComplete
				}
			}
		drainComplete:
			if len(batch) > 0 {
				l.flushBatch(batch)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := l.store.Flush(ctx); err != nil {
				slog.Error("failed to flush request flow store", "error", err)
			}
			cancel()
			return
		}
	}
}

func (l *Logger) flushBatch(batch []*Execution) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := l.store.WriteExecutionBatch(ctx, batch); err != nil {
		slog.Error("failed to write request flow batch", "error", err, "count", len(batch))
	}
}

// NoopLogger ignores executions.
type NoopLogger struct{}

// Write does nothing.
func (l *NoopLogger) Write(_ *Execution) {}

// Close does nothing.
func (l *NoopLogger) Close() error { return nil }

// LoggerInterface abstracts execution logging.
type LoggerInterface interface {
	Write(entry *Execution)
	Close() error
}
