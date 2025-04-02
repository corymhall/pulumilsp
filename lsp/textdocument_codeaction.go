package lsp

import (
	"encoding/json"

	"github.com/corymhall/pulumilsp/parser"
)

type CodeActionParams struct {
	WorkDoneProgressOptions
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

type CodeActionKind string

const (
	CodeActionKindEmpty                 CodeActionKind = ""
	CodeActionKindQuickFix              CodeActionKind = "quickfix"
	CodeActionKindRefactor              CodeActionKind = "refactor"
	CodeActionKindRefactorExtract       CodeActionKind = "refactor.extract"
	CodeActionKindRefactorInline        CodeActionKind = "refactor.inline"
	CodeActionKindRefactorRewrite       CodeActionKind = "refactor.rewrite"
	CodeActionKindMoveToCodeActionKind  CodeActionKind = "refactor.move"
	CodeActionKindSource                CodeActionKind = "source"
	CodeActionKindSourceOrganizeImports CodeActionKind = "source.organizeImports"
	CodeActionKindSourceFixAll          CodeActionKind = "source.fixAll"
)

type TriggerKind int

const (
	TriggerKindInvoked TriggerKind = 1
	TriggerKindAuto    TriggerKind = 2
)

type CodeActionContext struct {
	Diagnostics []Diagnostic     `json:"diagnostics"`
	Only        []CodeActionKind `json:"only,omitempty"`
	TriggerKind TriggerKind      `json:"triggerKind,omitempty"`
}

type CodeAction struct {
	// A short, human-readable, title for this code action.
	Title string `json:"title"`
	// The kind of the code action. Used to filter code actions.
	Kind CodeActionKind `json:"kind"`
	// The workspace edit this code action performs.
	Edit *WorkspaceEdit `json:"edit,omitempty"`
	// The diagnostics that this code action resolves
	Diagnostics []Diagnostic     `json:"diagnostics,omitempty"`
	Command     *Command         `json:"command,omitempty"`
	Data        *json.RawMessage `json:"data,omitempty"`
}

type CodeActionResolveData struct {
	parser.CaptureInfo
	URI DocumentURI `json:"uri"`
}
type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}
