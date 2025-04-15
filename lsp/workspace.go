package lsp

type LSPAny = any

type ParamConfiguration struct {
	Items []ConfigurationItem `json:"items"`
}

type ConfigurationItem struct {
	ScopeURI *DocumentURI `json:"scopeUri,omitempty"`
	Section  *string      `json:"section,omitempty"`
}
