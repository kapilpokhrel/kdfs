// Package multih defines the handler for slog to manage multiple handler
package multih

import (
	"context"
	"log/slog"
)

type MultiHandler struct {
	handlers []slog.Handler
}

var _ = (slog.Handler)((*MultiHandler)(nil))

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Add(handler slog.Handler) {
	m.handlers = append(m.handlers, handler)
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		r := record.Clone()
		err := h.Handle(ctx, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		handlers = append(handlers, h.WithAttrs(attrs))
	}
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		handlers = append(handlers, h.WithGroup(name))
	}
	return &MultiHandler{handlers: handlers}
}
