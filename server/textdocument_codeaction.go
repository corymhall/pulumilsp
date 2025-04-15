package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/corymhall/pulumilsp/ai"
	"github.com/corymhall/pulumilsp/debug"
	"github.com/corymhall/pulumilsp/lsp"
)

func (s *server) CodeAction(ctx context.Context, params *lsp.CodeActionParams) ([]lsp.CodeAction, error) {
	_, done := debug.Start(ctx, "CodeAction")
	defer done()
	actions := []lsp.CodeAction{}
	for _, diag := range params.Context.Diagnostics {
		actions = append(actions, lsp.CodeAction{
			Title:       fmt.Sprintf("Fix with Copilot (%s)", diag.Source),
			Kind:        lsp.CodeActionKindQuickFix,
			Data:        diag.Data,
			Diagnostics: []lsp.Diagnostic{diag},
		})
	}

	return actions, nil
}

func (s *server) ResolveCodeAction(ctx context.Context, params *lsp.CodeAction) (*lsp.CodeAction, error) {
	ctx, done := debug.Start(ctx, "ResolveCodeAction")
	defer done()
	res, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	debug.Debug.Log(ctx, "Received resolveCodeAction request", "request", string(res))
	var data lsp.CodeActionResolveData
	err = json.Unmarshal(*params.Data, &data)
	if err != nil {
		debug.LogError(ctx, "error unmarshalling capture info", err)
		return nil, fmt.Errorf("error unmarshalling capture info: %w", err)
	}

	if len(params.Diagnostics) != 1 {
		debug.LogError(ctx, "error processing diagnostics", errors.New(fmt.Sprintf("expected 1 diagnostic, got %d", len(params.Diagnostics))))
		return nil, fmt.Errorf("expected 1 diagnostic, got %d", len(params.Diagnostics))
	}
	diagnostic := params.Diagnostics[0]

	work := s.progress.Start(ctx, "Pulumi", "Fixing with Copilot...", nil, nil)
	var fix string
	if strings.Contains(params.Title, "Replication") {
		fix, err = s.aiClient.FixWithCopilot(ctx, "pulumi", data.Text, diagnostic.Message)
	} else {
		fix, err = ai.FixWithOpenAI(ctx, data.Text, diagnostic.Message)
	}

	work.End(ctx, "Done.")
	if err != nil || fix == "" {
		debug.LogError(ctx, "error getting fix with copilot", err)
		return nil, fmt.Errorf("error getting fix with copilot: %w", err)
	}
	debug.Debug.Log(ctx, "fix found with copilot", "fix", fix)
	return &lsp.CodeAction{
		Title: params.Title,
		Kind:  params.Kind,
		Edit: &lsp.WorkspaceEdit{
			Changes: map[lsp.DocumentURI][]lsp.TextEdit{
				data.URI: []lsp.TextEdit{
					{
						NewText: fix,
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      int32(data.CaptureInfo.StartPoint.Row),
								Character: int32(data.CaptureInfo.StartPoint.Column),
							},
							End: lsp.Position{
								Line:      int32(data.CaptureInfo.EndPoint.Row),
								Character: int32(data.CaptureInfo.EndPoint.Column),
							},
						},
					},
				},
			},
		},
	}, nil
}
