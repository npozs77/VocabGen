package llm

import (
	"context"
)

// mockProvider is a test double that returns a configured response or error.
type mockProvider struct {
	response    string
	err         error
	invocations int
}

func (m *mockProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	m.invocations++
	if m.err != nil {
		return "", &ProviderError{Provider: "mock", Message: m.err.Error(), Err: m.err}
	}
	return m.response, nil
}

func (m *mockProvider) Name() string { return "mock" }
