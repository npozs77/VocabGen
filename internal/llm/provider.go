package llm

import (
	"context"
	"fmt"
)

// Provider defines the contract for LLM API backends.
// Any struct with these two methods automatically satisfies this interface.
type Provider interface {
	// Invoke sends a prompt to the LLM and returns the raw text response.
	// ctx carries cancellation/timeout; callers can cancel long-running requests.
	Invoke(ctx context.Context, prompt string, modelID string) (string, error)

	// Name returns the provider identifier (e.g., "bedrock", "openai").
	Name() string
}

// ProviderError wraps provider-specific errors with the provider name.
// All provider errors can be checked with errors.As(&ProviderError{}).
type ProviderError struct {
	Provider string
	Message  string
	Err      error // underlying error, if any
}

// Error returns a formatted error string including the provider name.
func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *ProviderError) Unwrap() error { return e.Err }

// NewProviderFunc is the signature for provider constructor functions.
// The registry maps provider names to these constructors.
type NewProviderFunc func(opts ProviderOptions) (Provider, error)

// ProviderOptions holds configuration passed to provider constructors.
type ProviderOptions struct {
	APIKey     string // for OpenAI/Anthropic
	BaseURL    string // for OpenAI-compatible servers (Azure, Ollama, LM Studio)
	Region     string // for Bedrock (AWS) or Vertex AI (GCP)
	Profile    string // for Bedrock AWS profile
	GCPProject string // for Vertex AI
}

// Registry maps provider name strings to constructor functions.
var Registry = map[string]NewProviderFunc{
	"bedrock":   NewBedrockProvider,
	"openai":    NewOpenAIProvider,
	"anthropic": NewAnthropicProvider,
	"vertexai":  NewVertexAIProvider,
}
