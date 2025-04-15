package logger

import (
	"context"
	"log/slog"
	"sync"

	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/xcontext"
)

var ProgramLevel = new(slog.LevelVar)

var (
	startLogSenderOnce sync.Once
	logQueue           = make(chan func(), 100) // big enough for a large transient burst
)

func Log(ctx context.Context, msg string, mt lsp.MessageType) {
	client := lsp.GetClient(ctx)
	if client == nil {
		return
	}
	logMsg := &lsp.LogMessageParams{
		Message:     msg,
		MessageType: mt,
	}

	startLogSenderOnce.Do(func() {
		go func() {
			for fn := range logQueue {
				fn()
			}
		}()
	})

	ctx2 := xcontext.Detach(ctx)
	logQueue <- func() { client.LogMessage(ctx2, logMsg) }
}

func convertLevel(level slog.Level) lsp.MessageType {
	switch level {
	case slog.LevelDebug:
		return lsp.MessageType(5)
	case slog.LevelInfo:
		return lsp.MessageType(3)
	case slog.LevelWarn:
		return lsp.MessageType(2)
	case slog.LevelError:
		return lsp.MessageType(1)
	default:
		return lsp.MessageType(4)
	}
}
