package model

import "time"

// DeployAppReq asks for an AI application to be placed on an AI semiconductor node.
type DeployAppReq struct {
	// AppID selects the registered application to deploy.
	AppID string `json:"appId" validate:"required" example:"vllm-llama31-8b"`
	// InfraName becomes the CB-Tumblebug Infra ID and is the idempotency key.
	InfraName string `json:"infraName" validate:"required" example:"vllm-lab-01"`
	// SpecID pins the node spec. Empty asks for a recommendation first.
	SpecID string `json:"specId,omitempty" example:"aws+us-west-2+g6.xlarge"`
	// ImageID pins the node image. Empty asks CB-Tumblebug to select one.
	ImageID string `json:"imageId,omitempty" example:"ubuntu22.04"`
	// NodeCount is how many serving nodes to provision.
	NodeCount int `json:"nodeCount,omitempty" example:"1"`
	// SGTemplateID pins the security group template provisioning starts from.
	// Empty leaves the cloud manager default, which opens every port and is
	// documented upstream as being for development and testing only.
	SGTemplateID string `json:"sgTemplateId,omitempty" example:"sg-usecase-web"`
}

// Validate reports why a deployment request cannot be planned.
func (r *DeployAppReq) Validate() error {
	switch {
	case r.AppID == "":
		return errRequired("appId")
	case r.InfraName == "":
		return errRequired("infraName")
	case r.NodeCount < 0:
		return errInvalid("nodeCount", "cannot be negative")
	}
	return nil
}

// DeploymentPlan is what the service would send to CB-Tumblebug.
//
// It is produced whether or not the call is actually made, so that a dry run and
// a real run archive the same shape and stay comparable.
type DeploymentPlan struct {
	AppID        string `json:"appId"`
	AppName      string `json:"appName"`
	Namespace    string `json:"nsId"`
	InfraName    string `json:"infraName"`
	SpecID       string `json:"specId"`
	ImageID      string `json:"imageId"`
	SGTemplateID string `json:"sgTemplateId,omitempty"`
	NodeCount    int    `json:"nodeCount"`
	// SpecSource records how SpecID was chosen: "requested" or "recommended".
	SpecSource string `json:"specSource"`
	// ImageSource records how ImageID was chosen: "requested" or "resolved".
	ImageSource string `json:"imageSource"`
	// ImageReason explains an image that was resolved rather than requested.
	ImageReason string `json:"imageReason,omitempty"`
	// AcceleratorSummary describes the accelerator the chosen spec provides.
	AcceleratorSummary string `json:"acceleratorSummary,omitempty"`
	// EstimatedCost is the CB-Tumblebug review estimate, when a review ran.
	EstimatedCost string `json:"estimatedCost,omitempty"`
	// Request is the dynamic Infra request body, including the install commands.
	Request any `json:"request"`
}

// ServingAccess reports whether the application's serving port can be reached.
//
// Provisioning opens SSH only, so this is what turns a running node into a
// usable endpoint. It is part of the result rather than a side effect so that a
// caller learns immediately when the port stayed closed.
type ServingAccess struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	// Opened is false when the rule could not be added; Message says why.
	Opened bool `json:"opened"`
	// SourceIP is the range the port was opened to.
	SourceIP         string   `json:"sourceIp,omitempty"`
	SecurityGroupIDs []string `json:"securityGroupIds,omitempty"`
	// Endpoints are the reachable addresses, one per node with a public address.
	Endpoints []string `json:"endpoints,omitempty"`
	Message   string   `json:"message"`
}

// DeploymentResult reports the outcome of a deployment attempt.
type DeploymentResult struct {
	Plan *DeploymentPlan `json:"plan"`
	// DryRun is true when the plan was archived but never sent.
	DryRun bool `json:"dryRun"`
	// Review is the CB-Tumblebug pre-flight verdict, when a review ran.
	Review *ReviewSummary `json:"review,omitempty"`
	// Infra is the provisioned Infra, present only on a real run.
	Infra any `json:"infra,omitempty"`
	// Serving reports whether the application's port was opened, on a real run.
	Serving *ServingAccess `json:"serving,omitempty"`
	// Message explains the outcome to the caller.
	Message string `json:"message"`
}

// ReviewSummary is the caller-facing part of a CB-Tumblebug provisioning review.
type ReviewSummary struct {
	CreationViable  bool     `json:"creationViable"`
	OverallStatus   string   `json:"overallStatus"`
	OverallMessage  string   `json:"overallMessage"`
	EstimatedCost   string   `json:"estimatedCost,omitempty"`
	TotalNodeCount  int      `json:"totalNodeCount"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// ControlAppReq applies a lifecycle action to a deployed application.
type ControlAppReq struct {
	// Action is one of suspend, resume, reboot, terminate.
	Action string `json:"action" validate:"required" example:"suspend" enums:"suspend,resume,reboot,terminate"`
}

// ControlResult reports a lifecycle action outcome.
type ControlResult struct {
	InfraID  string   `json:"infraId"`
	Action   string   `json:"action"`
	DryRun   bool     `json:"dryRun"`
	Affected []string `json:"affected,omitempty"`
	Message  string   `json:"message"`
}

// SpecCandidate is one node spec that satisfies an accelerator requirement.
type SpecCandidate struct {
	ID                  string  `json:"id"`
	ProviderName        string  `json:"providerName"`
	RegionName          string  `json:"regionName"`
	AcceleratorType     string  `json:"acceleratorType"`
	AcceleratorModel    string  `json:"acceleratorModel"`
	AcceleratorCount    int     `json:"acceleratorCount"`
	AcceleratorMemoryGB float64 `json:"acceleratorMemoryGB"`
	VCPU                int     `json:"vCPU"`
	MemoryGiB           float64 `json:"memoryGiB"`
	CostPerHour         float64 `json:"costPerHour"`
}

// ArchiveRecord is one archived agent run or direct deployment action.
//
// The record holds the operator instruction, every tool the agent chose, what it
// answered, and what the guard did. Keeping the intermediate steps is what makes
// the archive reusable as feedback for later planning, not just an audit log.
type ArchiveRecord struct {
	RunID     string      `json:"runId"`
	StartedAt time.Time   `json:"startedAt"`
	EndedAt   time.Time   `json:"endedAt"`
	Namespace string      `json:"nsId"`
	Kind      string      `json:"kind" example:"intent"`
	Intent    string      `json:"intent,omitempty"`
	Steps     []ToolStep  `json:"steps"`
	Answer    string      `json:"answer,omitempty"`
	Error     string      `json:"error,omitempty"`
	Usage     *TokenUsage `json:"usage,omitempty"`
}

// ToolStep is one tool the agent called during a run.
type ToolStep struct {
	Index int    `json:"index"`
	Tool  string `json:"tool"`
	// Grade is the tool risk grade that decided whether the call was executed.
	Grade string `json:"grade"`
	Input any    `json:"input"`
	// Executed is false when the dry-run guard blocked an infrastructure change.
	Executed bool      `json:"executed"`
	Output   any       `json:"output,omitempty"`
	Error    string    `json:"error,omitempty"`
	CalledAt time.Time `json:"calledAt"`
	Duration string    `json:"duration"`
}

// TokenUsage is the planning cost of one agent run.
type TokenUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// ArchiveListResp is the archive listing body.
type ArchiveListResp struct {
	Records []ArchiveRecord `json:"records"`
}
