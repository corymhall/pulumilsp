package server

import (
	"context"
	"os"

	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/xcontext"
)

func mustReadFile(ctx context.Context, uri lsp.DocumentURI) file.Handle {
	ctx = xcontext.Detach(ctx)
	fh, err := ReadFile(ctx, uri)
	if err != nil {
		// ReadFile cannot fail with an uncancellable context.
		return brokenFile{uri, err}
	}
	return fh
}

// A brokenFile represents an unexpected failure to read a file.
type brokenFile struct {
	uri lsp.DocumentURI
	err error
}

func (b brokenFile) URI() lsp.DocumentURI     { return b.uri }
func (b brokenFile) Version() int32           { return 0 }
func (b brokenFile) Content() ([]byte, error) { return nil, b.err }

// A diskFile is a file in the filesystem, or a failure to read one.
// It implements the file.Source interface.
type diskFile struct {
	uri     lsp.DocumentURI
	content []byte
	hash    file.Hash
	err     error
}

func (h *diskFile) URI() lsp.DocumentURI { return h.uri }

func (h *diskFile) Version() int32           { return 0 }
func (h *diskFile) Content() ([]byte, error) { return h.content, h.err }

// ReadFile stats and (maybe) reads the file, updates the cache, and returns it.
func ReadFile(ctx context.Context, uri lsp.DocumentURI) (file.Handle, error) {
	fh, err := readFile(ctx, uri) // ~25us
	if err != nil {
		return nil, err // e.g. cancelled (not: read failed)
	}

	return fh, nil
}

// ioLimit limits the number of parallel file reads per process.
var ioLimit = make(chan struct{}, 128)

func readFile(ctx context.Context, uri lsp.DocumentURI) (*diskFile, error) {
	select {
	case ioLimit <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-ioLimit }()

	// It is possible that a race causes us to read a file with different file
	// ID, or whose mtime differs from the given mtime. However, in these cases
	// we expect the client to notify of a subsequent file change, and the file
	// content should be eventually consistent.
	content, err := os.ReadFile(uri.Path()) // ~20us
	if err != nil {
		content = nil // just in case
	}
	return &diskFile{
		uri:     uri,
		content: content,
		hash:    file.HashOf(content),
		err:     err,
	}, nil
}
