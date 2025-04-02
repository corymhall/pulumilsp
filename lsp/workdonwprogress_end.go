package lsp

type WorkDoneProgressEndParams struct {
	Token ProgressToken             `json:"token"`
	Value *WorkDoneProgressEndValue `json:"value"`
}

type WorkDoneProgressEndValue struct {
	Kind    Kind   `json:"kind"`
	Message string `json:"message"`
}
