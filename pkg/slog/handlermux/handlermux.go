// Package handlermux defines the handler for slog to manage multiple handler
package handlermux

import (
	"context"
	"log/slog"
)

type HandlerMux struct {
	handlers []slog.Handler
}

var _ = (slog.Handler)((*HandlerMux)(nil))

func New(handlers ...slog.Handler) *HandlerMux {
	return &HandlerMux{handlers: handlers}
}

func (m *HandlerMux) Add(handler slog.Handler) {
	m.handlers = append(m.handlers, handler)
}

func (m *HandlerMux) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *HandlerMux) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		r := record.Clone()
		err := h.Handle(ctx, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *HandlerMux) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		handlers = append(handlers, h.WithAttrs(attrs))
	}
	return &HandlerMux{handlers: handlers}
}

func (m *HandlerMux) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		handlers = append(handlers, h.WithGroup(name))
	}
	return &HandlerMux{handlers: handlers}
}
