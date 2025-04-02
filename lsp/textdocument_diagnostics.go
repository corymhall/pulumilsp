package lsp

import "encoding/json"

// The diagnostic's severity.
type DiagnosticSeverity uint32

type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     int32        `json:"version"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"` // TODO: Change to enum
	Source   string             `json:"source"`
	Message  string             `json:"message"`
	Data     *json.RawMessage   `json:"data,omitempty"`
}

type TextDocumentDiagnosticsParams struct {
	WorkDoneProgressOptions
	TextDocument     TextDocumentIdentifier `json:"textDocument"`
	Identifier       *string                `json:"identifier,omitempty"`
	PreviousResultID *string                `json:"previousResultId,omitempty"`
}

type DocumentDiagnosticReportKind string

const (
	DocumentDiagnosticReportKind_Unchanged DocumentDiagnosticReportKind = "unchanged"
	DocumentDiagnosticReportKind_Full      DocumentDiagnosticReportKind = "full"
)

type FullDocumentDiagnosticReport struct {
	// An optional result ID. If provided it will be sent on the next
	// diagnostic request for the same document.
	ResultID *string      `json:"resultId,omitempty"`
	Items    []Diagnostic `json:"items"`
}

type UnchangedDocumentDiagnosticReport struct {
	ResultID string `json:"resultId"`
}

type TextDocumentDiagnosticsReport interface {
	GetKind() DocumentDiagnosticReportKind
	GetResultID() *string
	GetItems() *[]Diagnostic
}

func (f FullDocumentDiagnosticReport) GetKind() DocumentDiagnosticReportKind {
	return DocumentDiagnosticReportKind_Full
}

func (f FullDocumentDiagnosticReport) GetResultID() *string {
	return f.ResultID
}

func (f FullDocumentDiagnosticReport) GetItems() *[]Diagnostic {
	return &f.Items
}

func (u UnchangedDocumentDiagnosticReport) GetKind() DocumentDiagnosticReportKind {
	return DocumentDiagnosticReportKind_Unchanged
}

func (u UnchangedDocumentDiagnosticReport) GetResultID() *string {
	return &u.ResultID
}

func (u UnchangedDocumentDiagnosticReport) GetItems() *[]Diagnostic {
	return nil
}
