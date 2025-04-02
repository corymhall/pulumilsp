package file

import (
	"context"
	"crypto/sha256"

	"github.com/corymhall/pulumilsp/lsp"
)

type Handle interface {
	URI() lsp.DocumentURI
	Version() int32
	Content() ([]byte, error)
}

type Source interface {
	ReadFile(ctx context.Context, uri lsp.DocumentURI) (Handle, error)
}

type Hash [sha256.Size]byte

func HashOf(data []byte) Hash {
	return Hash(sha256.Sum256(data))
}

func (h *Hash) XORWith(h2 Hash) {
	for i := range h {
		h[i] ^= h2[i]
	}
}

// Modification represents a modification to a file.
type Modification struct {
	URI    lsp.DocumentURI
	Action Action

	// OnDisk is true if a watched file is changed on disk.
	// If true, Version will be -1 and Text will be nil.
	OnDisk bool

	// Version will be -1 and Text will be nil when they are not supplied,
	// specifically on textDocument/didClose and for on-disk changes.
	Version int32
	Text    []byte

	// LanguageID is only sent from the language client on textDocument/didOpen.
	LanguageID lsp.LanguageKind
}

// An Action is a type of file state change.
type Action int

const (
	UnknownAction = Action(iota)
	Open
	Change
	Close
	Save
	Create
	Delete
)

func (a Action) String() string {
	switch a {
	case Open:
		return "Open"
	case Change:
		return "Change"
	case Close:
		return "Close"
	case Save:
		return "Save"
	case Create:
		return "Create"
	case Delete:
		return "Delete"
	default:
		return "Unknown"
	}
}
