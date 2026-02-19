// Package logging provides a pretty-printing slog.Handler for local development.
package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

// PrettyHandler is a zero-dependency slog.Handler that writes colorized,
// human-readable log lines in the format:
//
//	HH:MM:SS LEVEL msg  key=value key=value
type PrettyHandler struct {
	mu  sync.Mutex
	out io.Writer
}

func NewPrettyHandler(out io.Writer) *PrettyHandler {
	return &PrettyHandler{out: out}
}

func (h *PrettyHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	buf.WriteString(colorGray)
	buf.WriteString(r.Time.Format(time.TimeOnly))
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	buf.WriteString(levelColor(r.Level))
	buf.WriteString(colorBold)
	fmt.Fprintf(&buf, "%-5s", r.Level.String())
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	buf.WriteString(colorGray)
	buf.WriteString(r.Message)
	buf.WriteString(colorReset)

	r.Attrs(func(a slog.Attr) bool {
		buf.WriteByte(' ')
		buf.WriteString(colorCyan)
		buf.WriteString(a.Key)
		buf.WriteString(colorReset)
		buf.WriteByte('=')
		buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

func (h *PrettyHandler) WithAttrs(_ []slog.Attr) slog.Handler  { return h }
func (h *PrettyHandler) WithGroup(_ string) slog.Handler        { return h }

func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return colorRed
	case l >= slog.LevelWarn:
		return colorYellow
	case l >= slog.LevelInfo:
		return colorGreen
	default:
		return colorGray
	}
}
