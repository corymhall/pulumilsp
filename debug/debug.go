package debug

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Level int

const (
	_ Level = iota
	Error
	Warning
	Info
	Debug
	Trace
)

type loggerCtx int

const (
	loggerCtxKey = loggerCtx(iota)
)

func withLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}

func getLoggger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(*slog.Logger)
	if !ok {
		return slog.Default()
	}

	return logger
}

func convertLevel(level Level) slog.Level {
	switch level {
	case Error:
		return slog.LevelError
	case Warning:
		return slog.LevelWarn
	case Info:
		return slog.LevelInfo
	case Debug:
		return slog.LevelDebug
	default:
		return slog.LevelDebug
	}
}

func (l Level) Log(ctx context.Context, msg string, args ...any) {
	logger := getLoggger(ctx)
	logger.Log(ctx, convertLevel(l), msg, args...)
}

func Detach(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, loggerCtxKey, nil)
	return ctx
}

func LogError(ctx context.Context, msg string, err error) {
	logger := getLoggger(ctx)
	logger.Log(ctx, slog.LevelError, msg, slog.Any("error", err))
}

func WithGroup(ctx context.Context, name string) (context.Context, *slog.Logger) {
	logger := getLoggger(ctx).WithGroup(name)
	ctx = withLogger(ctx, logger)
	return ctx, logger
}

func With(ctx context.Context, args ...any) (context.Context, *slog.Logger) {
	logger := getLoggger(ctx).With(args...)
	ctx = withLogger(ctx, logger)
	return ctx, logger
}

func Start(ctx context.Context, name string, args ...any) (context.Context, func()) {
	logger := getLoggger(ctx).WithGroup(name)
	ctx = withLogger(ctx, logger)
	logger.Log(ctx, slog.LevelDebug, fmt.Sprintf("%s Starting...", name), args...)
	start := time.Now()

	return ctx, func() {
		elapsed := time.Since(start)
		if elapsed < time.Second {
			elapsed = 0
		}
		args = append(args, slog.Duration("elapsed", elapsed))
		logger.Log(ctx, slog.LevelDebug, fmt.Sprintf("%s Done", name), args...)
	}
}
