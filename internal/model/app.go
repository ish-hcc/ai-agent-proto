package model

// Runtime is the inference engine that serves an AI application.
//
// The engine and the accelerator are separate axes on purpose. A vendor NPU is
// reached through an engine plugin (for example the vLLM RBLN plugin), so the
// application stays "vLLM" while the accelerator underneath changes. Folding the
// two into one value ("vllm-gpu", "vllm-npu") breaks the spec every time an
// accelerator is added.
type Runtime string

// Runtimes this prototype can describe.
const (
	RuntimeVLLM   Runtime = "vllm"
	RuntimeTriton Runtime = "triton"
	RuntimeTGI    Runtime = "tgi"
	RuntimeOllama Runtime = "ollama"
)

// DeployTarget is where an application can be placed.
//
// Only DeployTargetVM is implemented in this prototype; DeployTargetContainer
// exists so that the metadata written now survives the container work.
type DeployTarget string

// Deploy targets this prototype can describe.
const (
	DeployTargetVM        DeployTarget = "vm"
	DeployTargetContainer DeployTarget = "container"
)

// AcceleratorRequirement is what an AI application needs from an AI semiconductor.
//
// The field names mirror the CB-Tumblebug spec attributes (acceleratorType,
// acceleratorModel, acceleratorCount, acceleratorMemoryGB) so that a requirement
// converts to a spec recommendation filter without a lookup table.
type AcceleratorRequirement struct {
	// Type is the accelerator class, matched against the spec acceleratorType.
	Type string `json:"type" validate:"required" example:"gpu"`
	// Model narrows the search to a device model. Empty means any model of Type.
	Model string `json:"model,omitempty" example:"NVIDIA L4"`
	// MinCount is the smallest acceptable number of devices per node.
	MinCount int `json:"minCount" validate:"required" example:"1"`
	// MinMemoryGiB is the smallest acceptable per-device memory. Zero means unset.
	MinMemoryGiB float64 `json:"minMemoryGiB,omitempty" example:"22"`
}

// ModelRef points at the AI model an application serves.
//
// This is a reference, not a copy: model registration itself belongs to the
// AI model management framework, and this field is the seam to it.
type ModelRef struct {
	// ID is the model identifier in the AI model registry.
	ID string `json:"id" validate:"required" example:"meta-llama/Llama-3.1-8B-Instruct"`
	// Format is the artifact format the runtime loads.
	Format string `json:"format,omitempty" example:"safetensors"`
	// SizeGiB is the on-disk footprint, used to size the root disk.
	SizeGiB float64 `json:"sizeGiB,omitempty" example:"16"`
}

// ResourceRequirement is the non-accelerator floor an application needs.
type ResourceRequirement struct {
	MinVCPU      int `json:"minVCPU,omitempty" example:"8"`
	MinMemoryGiB int `json:"minMemoryGiB,omitempty" example:"32"`
	RootDiskGiB  int `json:"rootDiskGiB,omitempty" example:"100"`
}

// ServingSpec is how the deployed application is reached and checked.
//
// It is also the seam to AI application monitoring: HealthPath and Port are what
// a monitoring collector needs to know an application is alive.
type ServingSpec struct {
	Port       int    `json:"port" validate:"required" example:"8000"`
	HealthPath string `json:"healthPath,omitempty" example:"/health"`
	Protocol   string `json:"protocol,omitempty" example:"http"`
}

// InstallSpec is what runs on a node once it is provisioned.
//
// The commands are handed to CB-Tumblebug as postCommands of the dynamic Infra
// request, so provisioning and application install are one call.
type InstallSpec struct {
	Commands       []string `json:"commands" validate:"required"`
	UserName       string   `json:"userName,omitempty" example:"cb-user"`
	TimeoutMinutes int      `json:"timeoutMinutes,omitempty" example:"30"`
	// OSType selects the node image family. Empty uses the service default.
	OSType string `json:"osType,omitempty" example:"ubuntu 22.04"`
}

// AppSpec is the AI application metadata record.
//
// This type is the prototype's answer to the first-year deliverable
// "AI semiconductor specific AI application metadata specification".
type AppSpec struct {
	ID           string                 `json:"id" validate:"required" example:"vllm-llama31-8b"`
	Name         string                 `json:"name" validate:"required" example:"vLLM Llama 3.1 8B Instruct"`
	Version      string                 `json:"version" validate:"required" example:"0.1.0"`
	Description  string                 `json:"description,omitempty" example:"OpenAI compatible inference endpoint"`
	Runtime      Runtime                `json:"runtime" validate:"required" example:"vllm"`
	RuntimeVer   string                 `json:"runtimeVersion,omitempty" example:"0.6.3"`
	Model        ModelRef               `json:"model" validate:"required"`
	Accelerator  AcceleratorRequirement `json:"accelerator" validate:"required"`
	Resources    ResourceRequirement    `json:"resources,omitempty"`
	Serving      ServingSpec            `json:"serving" validate:"required"`
	Install      InstallSpec            `json:"install" validate:"required"`
	DeployTarget []DeployTarget         `json:"deployTargets,omitempty"`
	Labels       map[string]string      `json:"labels,omitempty"`
}

// Validate reports why an AppSpec cannot be registered.
//
// Validation is explicit rather than tag-driven because adding a validator
// library is a dependency decision that has not been made yet; the validate tags
// above document the same contract for the generated Swagger.
func (a *AppSpec) Validate() error {
	switch {
	case a.ID == "":
		return errRequired("id")
	case a.Name == "":
		return errRequired("name")
	case a.Version == "":
		return errRequired("version")
	case a.Runtime == "":
		return errRequired("runtime")
	case a.Model.ID == "":
		return errRequired("model.id")
	case a.Accelerator.Type == "":
		return errRequired("accelerator.type")
	case a.Accelerator.MinCount <= 0:
		return errInvalid("accelerator.minCount", "must be at least 1")
	case a.Serving.Port <= 0 || a.Serving.Port > 65535:
		return errInvalid("serving.port", "must be between 1 and 65535")
	case len(a.Install.Commands) == 0:
		return errRequired("install.commands")
	}
	return nil
}

// AppSpecListResp is the catalog listing body.
type AppSpecListResp struct {
	Apps []AppSpec `json:"apps"`
}
