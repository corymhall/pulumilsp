package ai

import "encoding/json"

// Requests

type CopilotRequest struct {
	Query string       `json:"query"`
	State CopilotState `json:"state"`
}

type CopilotCodeFixRequest struct {
	CopilotRequest
	DirectSkillCall CopilotDirectSkillCall `json:"directSkillCall"`
}

type CopilotState struct {
	Client CopilotClientState `json:"client"`
}

type CopilotClientState struct {
	CloudContext CopilotCloudContext `json:"cloudContext"`
}

type CopilotCloudContext struct {
	OrgID string `json:"orgId"` // The organization ID.
	URL   string `json:"url"`   // The URL the user is viewing. Mock value often used.
}

type CopilotDirectSkillCall struct {
	Skill  string             `json:"skill"` // The skill to call. e.g. "summarizeUpdate"
	Params CopilotSkillParams `json:"params"`
}

type CopilotSkillParams struct {
	ValidateResult              bool     `json:"validateResult"`              // Whether to validate the result.
	ValidationStopPhase         string   `json:"validationStopPhase"`         // The phase to stop validation. e.g. "preview"
	SelfDebugMaxIterations      int      `json:"selfDebugMaxIterations"`      // The maximum number of iterations for self-debugging.
	SelfDebugMaxNumberOfErrors  int      `json:"selfDebugMaxNumberOfErrors"`  // The maximum number of errors for self-debugging.
	SelfDebugSupportedLanguages []string `json:"selfDebugSupportedLanguages"` // The supported languages for self-debugging.
	GetProjectFiles             bool     `json:"getProjectFiles"`             // Whether to get project files.
	CustomInstructions          string   `json:"customInstructions"`          // Custom instructions for the assistant.
}

// Responses

type CopilotCodeFixResponse struct {
	ThreadMessages []CopilotThreadMessage `json:"messages"`
	Error          string                 `json:"error"`
	Details        string                 `json:"details"` // The details of the error.
}

type CopilotThreadMessage struct {
	Role    string          `json:"role"`    // The role of the message. e.g. "assistant" / "user"
	Kind    string          `json:"kind"`    // Depends on the tool called, e.g. "response" / "program"
	Content json.RawMessage `json:"content"` // The content of the message. String or JSON object.
}
