package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/rpc"
)

func (s *server) Initialize(ctx context.Context, params *lsp.InitializeRequestParams) (*lsp.InitializeResult, error) {
	s.stateMu.Lock()
	if s.state >= serverInitializing {
		defer s.stateMu.Unlock()
		return nil, fmt.Errorf("%w: initialize called while server in %v state", rpc.ErrInvalidRequest, s.state)
	}
	s.progress.SetSupportsWorkDoneProgress(params.Capabilities.Window.WorkDoneProgress)
	s.state = serverInitializing
	s.stateMu.Unlock()
	s.rootURI = params.RootURI
	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: 1,
			CodeActionProvider: lsp.CodeActionProviderOptions{
				ResolveProvider: true,
				CodeActionKinds: []lsp.CodeActionKind{
					lsp.CodeActionKindQuickFix,
					lsp.CodeActionKindRefactor,
					lsp.CodeActionKindRefactorRewrite,
				},
			},
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
	s.initializeView(ctx)
	return nil
}
