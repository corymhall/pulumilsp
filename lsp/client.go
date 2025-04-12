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
