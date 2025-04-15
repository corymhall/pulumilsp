package lsp

type LogMessageParams struct {
	Message     string      `json:"message"`
	MessageType MessageType `json:"messageType"`
}

// The message type
type MessageType uint32
type ShowMessageParams struct {
	// The message type. See {@link MessageType}
	Type MessageType `json:"type"`
	// The actual message.
	Message string `json:"message"`
}
