package server

import (
	"context"

	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
)

func (s *server) DidOpen(ctx context.Context, params *lsp.DidOpenTextDocumentParams) error {
	return s.didModifyFiles(ctx, []file.Modification{{
		URI:        params.TextDocument.URI,
		Action:     file.Open,
		Version:    params.TextDocument.Version,
		Text:       []byte(params.TextDocument.Text),
		LanguageID: params.TextDocument.LanguageID,
	}}, FromDidOpen)
}
