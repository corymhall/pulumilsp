package lsp

type Kind string

const (
	Begin  Kind = "begin"
	Report Kind = "report"
	End    Kind = "end"
)

type WorkDoneProgressCreateParams struct {
	// The token to be used to report progress.
	Token ProgressToken `json:"token"`
}

type ProgressParams struct {
	Token ProgressToken `json:"token"`
	Value any           `json:"value"`
}
