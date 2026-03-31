package llm

import (
	"context"
	"fmt"
)

// VertexAIProvider implements Provider for Google Vertex AI.
// Stub — full implementation in task 7.8.
type VertexAIProvider struct{}

// NewVertexAIProvider creates a VertexAIProvider. Stub — returns error until implemented.
func NewVertexAIProvider(opts ProviderOptions) (Provider, error) {
	return nil, &ProviderError{
		Provider: "vertexai",
		Message:  "Vertex AI provider not yet implemented",
	}
}

// Invoke is a stub that returns an error until the Vertex AI provider is fully implemented.
func (p *VertexAIProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	return "", fmt.Errorf("vertexai: not yet implemented")
}

// Name returns the provider identifier.
func (p *VertexAIProvider) Name() string { return "vertexai" }
