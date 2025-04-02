package file

import (
	"fmt"

	"github.com/corymhall/pulumilsp/lsp"
)

// Kind describes the kind of the file in question.
// It can be one of Go,mod, Sum, or Tmpl.
type Kind int

const (
	// UnknownKind is a file type we don't know about.
	UnknownKind = Kind(iota)

	// TypeScript is a TypeScript source file.
	TypeScript
)

func (k Kind) String() string {
	switch k {
	case TypeScript:
		return "typescript"
	default:
		return fmt.Sprintf("internal error: unknown file kind %d", k)
	}
}

// KindForLang returns the gopls file [Kind] associated with the given LSP
// LanguageKind string from the LanguageID field of [protocol.TextDocumentItem],
// or UnknownKind if the language is not one recognized by gopls.
func KindForLang(langID lsp.LanguageKind) Kind {
	switch langID {
	case "typescript":
		return TypeScript
	default:
		return UnknownKind
	}
}
