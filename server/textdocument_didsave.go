package server

import (
	"context"

	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
)

func (s *server) DidSave(ctx context.Context, params *lsp.DidSaveTextDocumentParams) error {
	c := file.Modification{
		URI:    params.TextDocument.URI,
		Action: file.Save,
	}
	if params.Text != nil {
		c.Text = []byte(*params.Text)
	}

	return s.didModifyFiles(ctx, []file.Modification{c}, FromDidSave)
}
