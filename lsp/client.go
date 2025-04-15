package lsp

import "context"

type Client interface {
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_publishDiagnostics
	PublishDiagnostics(context.Context, *PublishDiagnosticsParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_workDoneProgress_create
	WorkDoneProgressCreate(context.Context, *WorkDoneProgressCreateParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#progress
	ProgressBegin(context.Context, *WorkDoneProgressBeginParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#progress
	ProgressEnd(context.Context, *WorkDoneProgressEndParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_showMessage
	ShowMessage(context.Context, *ShowMessageParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#logTrace
	// LogTrace(context.Context, *LogTraceParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_logMessage
	LogMessage(context.Context, *LogMessageParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_configuration
	Configuration(context.Context, *ParamConfiguration) ([]LSPAny, error)
}

func (s *clientDispatcher) PublishDiagnostics(ctx context.Context, params *PublishDiagnosticsParams) error {
	return s.sender.Notify(ctx, "textDocument/publishDiagnostics", params)
}

func (s *clientDispatcher) WorkDoneProgressCreate(ctx context.Context, params *WorkDoneProgressCreateParams) error {
	return s.sender.Call(ctx, "window/workDoneProgress/create", params, nil)
}

func (s *clientDispatcher) ProgressBegin(ctx context.Context, params *WorkDoneProgressBeginParams) error {
	return s.sender.Notify(ctx, "$/progress", params)
}

func (s *clientDispatcher) ProgressEnd(ctx context.Context, params *WorkDoneProgressEndParams) error {
	return s.sender.Notify(ctx, "$/progress", params)
}

func (s *clientDispatcher) ShowMessage(ctx context.Context, params *ShowMessageParams) error {
	return s.sender.Notify(ctx, "window/showMessage", params)
}

func (s *clientDispatcher) LogMessage(ctx context.Context, params *LogMessageParams) error {
	return s.sender.Notify(ctx, "window/logMessage", params)
}

func (s *clientDispatcher) Configuration(ctx context.Context, params *ParamConfiguration) ([]LSPAny, error) {
	var result []LSPAny
	if err := s.sender.Call(ctx, "workspace/configuration", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
