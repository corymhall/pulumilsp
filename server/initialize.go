package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/corymhall/pulumilsp/debug"
	"github.com/corymhall/pulumilsp/logger"
	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/rpc"
)

type InitOptions struct {
	LogLevel *string `json:"logLevel,omitempty"`
}

func (s *server) Initialize(ctx context.Context, params *lsp.InitializeRequestParams) (*lsp.InitializeResult, error) {
	s.stateMu.Lock()
	if s.state >= serverInitializing {
		defer s.stateMu.Unlock()
		return nil, fmt.Errorf("%w: initialize called while server in %v state", errors.New(rpc.ErrInvalidRequest), s.state)
	}
	if params.InitializationOptions != nil {
		debug.Info.Log(ctx, "Received initialization options", "options", string(*params.InitializationOptions))
		var options InitOptions
		if err := json.Unmarshal(*params.InitializationOptions, &options); err != nil {
			return nil, fmt.Errorf("error unmarshalling initialization options: %w", err)
		}
		var level slog.Level
		if options.LogLevel != nil {
			switch strings.ToLower(*options.LogLevel) {
			case "debug":
				level = slog.LevelDebug
			case "info":
				level = slog.LevelInfo
			case "warn":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			}
			debug.Info.Log(ctx, "Setting log level", "level", level)
			logger.ProgramLevel.Set(level)
		}
	}
	s.progress.SetSupportsWorkDoneProgress(params.Capabilities.Window.WorkDoneProgress)
	s.state = serverInitializing
	s.stateMu.Unlock()
	s.rootURI = params.RootURI
	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: 1,
			// Don't enable this yet, just a hackathon idea
			// CodeActionProvider: lsp.CodeActionProviderOptions{
			// 	ResolveProvider: true,
			// 	CodeActionKinds: []lsp.CodeActionKind{
			// 		lsp.CodeActionKindQuickFix,
			// 		lsp.CodeActionKindRefactor,
			// 		lsp.CodeActionKindRefactorRewrite,
			// 	},
			// },
		},
		ServerInfo: lsp.ServerInfo{
			Name:    "pulumilsp",
			Version: "0.0.1",
		},
	}, nil
}

func (s *server) Initialized(ctx context.Context, params *lsp.InitializedParams) error {
	s.stateMu.Lock()
	if s.state >= serverInitialized {
		defer s.stateMu.Unlock()
		return fmt.Errorf("%w: initialized called while server in %v state", errors.New(rpc.ErrInvalidRequest), s.state)
	}
	s.state = serverInitialized
	s.stateMu.Unlock()

	// when we've initialized create the view
	// do this in a separate goroutine, otherwise it will
	// block from receiving more requests
	go func() {
		s.initializeView(ctx)
	}()
	return nil
}
