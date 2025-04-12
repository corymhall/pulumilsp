package lsp

type InitializeRequestParams struct {
	WorkDoneProgressCreateParams
	ClientInfo   *ClientInfo        `json:"clientInfo"`
	RootURI      DocumentURI        `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
	// ... there's tons more that goes here
}

type ClientCapabilities struct {
	Window ClientWindowCapabilities `json:"window"`
}

type ClientWindowCapabilities struct {
	WorkDoneProgress bool `json:"workDoneProgress"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type WorkDoneProgressOptions struct {
	WorkDoneProgress bool `json:"workDoneProgress"`
}

type DiagnosticOptions struct {
	WorkDoneProgressOptions
	Identifier            *string `json:"identifier,omitempty"`
	InterFileDependencies bool    `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool    `json:"workspaceDiagnostics"`
}

type CodeActionProviderOptions struct {
	CodeActionKinds []CodeActionKind `json:"codeActionKinds"`
	ResolveProvider bool             `json:"resolveProvider"`
}

type ServerCapabilities struct {
	TextDocumentSync   int                       `json:"textDocumentSync"`
	CodeActionProvider CodeActionProviderOptions `json:"codeActionProvider"`
	// Not enabled because it sends requests to the server a lot
	DiagnosticProvider DiagnosticOptions `json:"diagnosticProvider"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
