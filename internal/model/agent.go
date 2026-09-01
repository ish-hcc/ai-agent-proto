package model

// IntentReq is a natural language instruction for the deployment agent.
type IntentReq struct {
	// Intent is what the operator wants, in their own words.
	Intent string `json:"intent" validate:"required" example:"Llama 3.1 8B 추론 서버를 GPU 한 장짜리 노드에 올려줘"`
}

// Validate reports why an intent cannot be run.
func (r *IntentReq) Validate() error {
	if r.Intent == "" {
		return errRequired("intent")
	}
	return nil
}

// IntentResp is the outcome of one agent run.
type IntentResp struct {
	RunID string `json:"runId"`
	// Answer is what the agent reports back to the operator.
	Answer string `json:"answer"`
	// Steps is every tool the agent chose, in order, with what the guard did.
	Steps []ToolStep `json:"steps"`
	// DryRun reflects the guard setting the run executed under.
	DryRun bool        `json:"dryRun"`
	Usage  *TokenUsage `json:"usage,omitempty"`
}

// ToolDescriptor describes one tool the agent may call.
//
// The list is exposed over REST so that another framework can adopt the same
// tool surface without reading this service's source.
type ToolDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Grade is one of read, review, write, destructive.
	Grade string `json:"grade"`
	// GuardedByDryRun is true when the dry-run setting can block this tool.
	GuardedByDryRun bool `json:"guardedByDryRun"`
	InputSchema     any  `json:"inputSchema"`
}

// ToolListResp is the tool catalog body.
type ToolListResp struct {
	DryRun bool             `json:"dryRun"`
	Tools  []ToolDescriptor `json:"tools"`
}
