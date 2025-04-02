package lsp

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/corymhall/pulumilsp/rpc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type DocumentURI string

type LanguageKind string

func (uri DocumentURI) Path() string {
	contract.Assertf(strings.HasPrefix(string(uri), "file://"), "URI must start with file://")
	return filepath.FromSlash(string(uri)[7:])
}

func URIFromPath(path string) DocumentURI {
	if path == "" {
		return ""
	}
	return DocumentURI("file://" + filepath.ToSlash(path))
}

type Server interface {
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#progress
	// Progress(context.Context, *ProgressParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#codeAction_resolve
	ResolveCodeAction(context.Context, *CodeAction) (*CodeAction, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#codeLens_resolve
	// ResolveCodeLens(context.Context, *CodeLens) (*CodeLens, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#exit
	Exit(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#initialize
	Initialize(context.Context, *InitializeRequestParams) (*InitializeResult, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#initialized
	Initialized(context.Context, *InitializedParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#shutdown
	Shutdown(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_codeAction
	CodeAction(context.Context, *CodeActionParams) ([]CodeAction, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_codeLens
	// CodeLens(context.Context, *CodeLensParams) ([]CodeLens, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_diagnostic
	// Diagnostic(context.Context, *DocumentDiagnosticParams) (*DocumentDiagnosticReport, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_didChange
	// DidChange(context.Context, *DidChangeTextDocumentParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_didClose
	// DidClose(context.Context, *DidCloseTextDocumentParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_didOpen
	DidOpen(context.Context, *DidOpenTextDocumentParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_didSave
	DidSave(context.Context, *DidSaveTextDocumentParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_workDoneProgress_cancel
	// WorkDoneProgressCancel(context.Context, *WorkDoneProgressCancelParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_diagnostic
	// DiagnosticWorkspace(context.Context, *WorkspaceDiagnosticParams) (*WorkspaceDiagnosticReport, error)
	Logger() *log.Logger
}

func serverDispatch(ctx context.Context, server Server, reply rpc.Replier, r rpc.Request) (bool, error) {
	switch r.Method() {
	case "exit":
		err := server.Exit(ctx)
		return true, reply(ctx, nil, err)
	case "shutdown":
		err := server.Shutdown(ctx)
		return true, reply(ctx, nil, err)
	// case "$/progress":
	// 	var params ProgressParams
	// 	if err := UnmarshalJSON(r.Params(), &params); err != nil {
	// 		return true, sendParseError(ctx, reply, err)
	// 	}
	// 	err := server.Progress(ctx, &params)
	// 	return true, reply(ctx, nil, err)
	case "initialize":
		server.Logger().Printf("Received initialize request: %s", string(r.Params()))
		var params InitializeRequestParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := server.Initialize(ctx, &params)
		if err != nil {
			server.Logger().Printf("Error initialize: %s", err)
		}
		return true, reply(ctx, resp, err)
	case "initialized":
		server.Logger().Printf("Received initialized notification: %s", string(r.Params()))
		var params InitializedParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := server.Initialized(ctx, &params)
		if err != nil {
			server.Logger().Printf("Error initialized: %s", err)
		}
		return true, reply(ctx, nil, err)
	case "textDocument/didOpen":
		server.Logger().Printf("Received didOpen notification: %s", string(r.Params()))
		var params DidOpenTextDocumentParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := server.DidOpen(ctx, &params)
		if err != nil {
			server.Logger().Printf("Error didOpen: %s", err)
		}
		return true, reply(ctx, nil, err)
	case "textDocument/didSave":
		server.Logger().Printf("Received didSave notification: %s", string(r.Params()))
		var params DidSaveTextDocumentParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := server.DidSave(ctx, &params)
		if err != nil {
			server.Logger().Printf("Error didSave: %s", err)
		}
		return true, reply(ctx, nil, err)
	case "textDocument/codeAction":
		var params CodeActionParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := server.CodeAction(ctx, &params)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)
	case "codeAction/resolve":
		var params CodeAction
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := server.ResolveCodeAction(ctx, &params)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)
	default:
		return true, nil
	}
}

func sendParseError(ctx context.Context, reply rpc.Replier, err error) error {
	return reply(ctx, nil, fmt.Errorf("%w: %s", rpc.ErrParse, err))
}
