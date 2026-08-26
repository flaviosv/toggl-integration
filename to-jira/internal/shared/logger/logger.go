package logger

import (
	"context"
	"log/slog"
	"os"
)

func Initialize(env string) *slog.Logger {
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	return slog.New(handler)
}

type contextKey string

const loggerContextKey contextKey = "logger"

// WithLogger attaches l to ctx so a later FromContext call can retrieve it.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(loggerContextKey).(*slog.Logger)
	if !ok || l == nil {
		return slog.Default()
	}
	return l
}
