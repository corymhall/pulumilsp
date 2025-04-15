package server

import (
	"context"
	"log/slog"

	"github.com/corymhall/pulumilsp/debug"
	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
)

func (s *server) DidSave(ctx context.Context, params *lsp.DidSaveTextDocumentParams) error {
	ctx, done := debug.Start(ctx, "DidSave", slog.String("uri", string(params.TextDocument.URI)))
	defer done()
	c := file.Modification{
		URI:    params.TextDocument.URI,
		Action: file.Save,
	}
	if params.Text != nil {
		c.Text = []byte(*params.Text)
	}

	return s.didModifyFiles(ctx, []file.Modification{c}, FromDidSave)
}
