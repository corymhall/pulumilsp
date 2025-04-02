package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/corymhall/pulumilsp/ai"
	"github.com/corymhall/pulumilsp/lsp"
)

func (s *server) CodeAction(ctx context.Context, params *lsp.CodeActionParams) ([]lsp.CodeAction, error) {
	res, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	actions := []lsp.CodeAction{}
	for _, diag := range params.Context.Diagnostics {
		actions = append(actions, lsp.CodeAction{
			Title:       fmt.Sprintf("Fix with Copilot (%s)", diag.Source),
			Kind:        lsp.CodeActionKindQuickFix,
			Data:        diag.Data,
			Diagnostics: []lsp.Diagnostic{diag},
		})
	}

	s.logger.Printf("Received codeAction request: %s", string(res))
	return actions, nil
}

func (s *server) ResolveCodeAction(ctx context.Context, params *lsp.CodeAction) (*lsp.CodeAction, error) {
	res, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	s.logger.Printf("Received resolveCodeAction request: %s", string(res))
	var data lsp.CodeActionResolveData
	err = json.Unmarshal(*params.Data, &data)
	if err != nil {
		s.logger.Printf("error unmarshalling capture info: %v", err)
		return nil, fmt.Errorf("error unmarshalling capture info: %w", err)
	}

	if len(params.Diagnostics) != 1 {
		s.logger.Printf("expected 1 diagnostic, got %d", len(params.Diagnostics))
		return nil, fmt.Errorf("expected 1 diagnostic, got %d", len(params.Diagnostics))
	}
	diagnostic := params.Diagnostics[0]

	work := s.progress.Start(ctx, "Pulumi", "Fixing with Copilot...", nil, nil)
	var fix string
	if strings.Contains(params.Title, "Replication") {
		fix, err = s.aiClient.FixWithCopilot(ctx, "pulumi", data.Text, diagnostic.Message)
	} else {
		fix, err = ai.FixWithOpenAI(ctx, s.logger, data.Text, diagnostic.Message)
	}

	work.End(ctx, "Done.")
	if err != nil || fix == "" {
		s.logger.Printf("error getting fix with copilot: %v", err)
		return nil, fmt.Errorf("error getting fix with copilot: %w", err)
	}
	s.logger.Printf("fix found with copilot: %s", fix)
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
