package lsp

type WorkspaceDiagnosticParams struct {
	WorkDoneProgressOptions
	Identifier        *string            `json:"identifier,omitempty"`
	PreviousResultIDs []PreviousResultID `json:"previousResultIds"`
}

type PreviousResultID struct {
	URI string `json:"uri"`
	// The value of the previous result ID.
	Value string `json:"value"`
}

type FullWorkspaceDiagnosticReport struct {
	Items []RelatedFullDocumentDiagnosticReport `json:"items"`
}

type UnchangedWorkspaceDiagnosticReport struct {
	Items []RelatedUnchangedDocumentDiagnosticReport `json:"items"`
}

type WorkspaceDocumentDiagnosticReport struct {
	Kind    DocumentDiagnosticReportKind `json:"kind"`
	URI     string                       `json:"uri"`
	Version int                          `json:"version"`
}

type RelatedFullDocumentDiagnosticReport struct {
	WorkspaceDocumentDiagnosticReport
	FullDocumentDiagnosticReport
	RelatedDocuments map[string]FullOrUnchangedDocumentDiagnosticReport `json:"relatedDocuments"`
}

type RelatedUnchangedDocumentDiagnosticReport struct {
	WorkspaceDocumentDiagnosticReport
	UnchangedDocumentDiagnosticReport
	RelatedDocuments map[string]FullOrUnchangedDocumentDiagnosticReport `json:"relatedDocuments"`
}

type FullOrUnchangedDocumentDiagnosticReport struct {
	ResultID *string       `json:"resultId,omitempty"`
	Items    *[]Diagnostic `json:"items,omitempty"`
}
