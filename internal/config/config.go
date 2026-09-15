// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const envPrefix = "AIAPP"

// DefaultTumblebugBaseURL is where the service looks for CB-Tumblebug when
// nothing says otherwise.
//
// It is a default rather than a required value so that the service starts with
// no configuration at all. Everything that does not touch a cloud - the
// application catalog, the matcher, the tool listing, the archive, the API
// documentation - works without CB-Tumblebug, and refusing to start took all of
// that away to enforce a setting the deployment path would have reported anyway.
// The startup log says which URL is in use and whether it came from the
// environment, so a service pointed at the wrong place says so on the first line
// instead of at the first deployment.
const DefaultTumblebugBaseURL = "http://localhost:1323/tumblebug"

// Config holds all runtime configuration for the service.
type Config struct {
	Server    ServerConfig
	Tumblebug TumblebugConfig
	LLM       LLMConfig
	Agent     AgentConfig
	Archive   ArchiveConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int
}

// TumblebugConfig holds CB-Tumblebug connection settings.
//
// Creating an Infra blocks until provisioning finishes, so Timeout must stay
// generous: cancelling mid-flight aborts provisioning and can leave billable
// orphan resources behind.
type TumblebugConfig struct {
	BaseURL          string
	Username         string
	Password         string
	Timeout          time.Duration
	CredentialHolder string
	// ImageNamespace holds the shared node image catalog, which is not the
	// namespace a deployment is created in.
	ImageNamespace string
	// SpecNamespace holds the shared node spec catalog. It is separate from
	// ImageNamespace because the two catalogs are separate resource types, even
	// though a default install keeps both in the same namespace.
	SpecNamespace string
	// DefaultOSType is the image family used when an application does not name one.
	DefaultOSType string
	// DryRun keeps every infrastructure-changing tool from reaching CB-Tumblebug.
	// It defaults to true so that an agent run can never provision by accident.
	DryRun bool
	// BaseURLFromDefault records that nothing configured BaseURL. The caller
	// reports it at startup: a service quietly pointed at a built-in address is
	// the one failure here that would otherwise only surface at deployment time.
	BaseURLFromDefault bool
}

// LLMConfig holds the Anthropic Messages API settings used for planning.
type LLMConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
	Timeout   time.Duration
}

// AgentConfig bounds a single agent run.
type AgentConfig struct {
	// MaxSteps caps tool-use iterations so a looping model cannot run forever.
	MaxSteps int
}

// ArchiveConfig holds the deployment/control archive location.
type ArchiveConfig struct {
	Dir string
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("server.port", 8090)
	v.SetDefault("tumblebug.base_url", DefaultTumblebugBaseURL)
	v.SetDefault("tumblebug.timeout", 20*time.Minute)
	v.SetDefault("tumblebug.credential_holder", "")
	v.SetDefault("tumblebug.image_namespace", "system")
	v.SetDefault("tumblebug.spec_namespace", "system")
	v.SetDefault("tumblebug.default_os_type", "ubuntu 22.04")
	v.SetDefault("tumblebug.dry_run", true)
	v.SetDefault("llm.base_url", "https://api.anthropic.com")
	v.SetDefault("llm.model", "claude-opus-5")
	v.SetDefault("llm.max_tokens", 16000)
	v.SetDefault("llm.timeout", 5*time.Minute)
	v.SetDefault("agent.max_steps", 12)
	v.SetDefault("archive.dir", "./data/archive")

	// AutomaticEnv only resolves keys that viper already knows about.
	for _, key := range []string{
		"server.port",
		"tumblebug.base_url",
		"tumblebug.username",
		"tumblebug.password",
		"tumblebug.timeout",
		"tumblebug.credential_holder",
		"tumblebug.image_namespace",
		"tumblebug.spec_namespace",
		"tumblebug.default_os_type",
		"tumblebug.dry_run",
		"llm.base_url",
		"llm.api_key",
		"llm.model",
		"llm.max_tokens",
		"llm.timeout",
		"agent.max_steps",
		"archive.dir",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("failed to bind env for %q: %w", key, err)
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: v.GetInt("server.port"),
		},
		Tumblebug: TumblebugConfig{
			BaseURL:          strings.TrimRight(v.GetString("tumblebug.base_url"), "/"),
			Username:         v.GetString("tumblebug.username"),
			Password:         v.GetString("tumblebug.password"),
			Timeout:          v.GetDuration("tumblebug.timeout"),
			CredentialHolder: v.GetString("tumblebug.credential_holder"),
			ImageNamespace:   v.GetString("tumblebug.image_namespace"),
			SpecNamespace:    v.GetString("tumblebug.spec_namespace"),
			DefaultOSType:    v.GetString("tumblebug.default_os_type"),
			DryRun:           v.GetBool("tumblebug.dry_run"),
			// AutomaticEnv resolves the key whether or not the variable is set,
			// so the environment is the thing to ask.
			BaseURLFromDefault: os.Getenv(envPrefix+"_TUMBLEBUG_BASE_URL") == "",
		},
		LLM: LLMConfig{
			BaseURL:   strings.TrimRight(v.GetString("llm.base_url"), "/"),
			APIKey:    v.GetString("llm.api_key"),
			Model:     v.GetString("llm.model"),
			MaxTokens: v.GetInt("llm.max_tokens"),
			Timeout:   v.GetDuration("llm.timeout"),
		},
		Agent: AgentConfig{
			MaxSteps: v.GetInt("agent.max_steps"),
		},
		Archive: ArchiveConfig{
			Dir: v.GetString("archive.dir"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Tumblebug.BaseURL == "" {
		return fmt.Errorf("%s_TUMBLEBUG_BASE_URL must not be set to an empty value", envPrefix)
	}
	if c.Tumblebug.Timeout <= 0 {
		return fmt.Errorf("invalid tumblebug timeout: %s", c.Tumblebug.Timeout)
	}
	if c.Tumblebug.ImageNamespace == "" {
		return fmt.Errorf("%s_TUMBLEBUG_IMAGE_NAMESPACE is required", envPrefix)
	}
	if c.Tumblebug.SpecNamespace == "" {
		return fmt.Errorf("%s_TUMBLEBUG_SPEC_NAMESPACE is required", envPrefix)
	}
	if c.Tumblebug.DefaultOSType == "" {
		return fmt.Errorf("%s_TUMBLEBUG_DEFAULT_OS_TYPE is required", envPrefix)
	}
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("%s_LLM_BASE_URL is required", envPrefix)
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("%s_LLM_MODEL is required", envPrefix)
	}
	if c.LLM.MaxTokens <= 0 {
		return fmt.Errorf("invalid llm max tokens: %d", c.LLM.MaxTokens)
	}
	if c.LLM.Timeout <= 0 {
		return fmt.Errorf("invalid llm timeout: %s", c.LLM.Timeout)
	}
	if c.Agent.MaxSteps <= 0 {
		return fmt.Errorf("invalid agent max steps: %d", c.Agent.MaxSteps)
	}
	if c.Archive.Dir == "" {
		return fmt.Errorf("%s_ARCHIVE_DIR is required", envPrefix)
	}
	return nil
}
