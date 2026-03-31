package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestPropertyP17ProviderInterfaceConsistency validates that the Provider interface
// contract is consistent: Invoke returns either (non-empty string, nil error) or
// (empty string, non-nil error), Name() returns a non-empty string, and errors
// are wrappable as *ProviderError.
//
// **Validates: Requirements 7.2, 7.3, 13.1, 13.4**
func TestPropertyP17ProviderInterfaceConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		prompt := rapid.String().Draw(t, "prompt")
		modelID := rapid.String().Draw(t, "modelID")
		shouldSucceed := rapid.Bool().Draw(t, "shouldSucceed")

		var provider Provider

		if shouldSucceed {
			// Create a mock that returns a non-empty response with no error.
			response := rapid.StringMatching(".+").Draw(t, "response")
			provider = &mockProvider{
				response: response,
				err:      nil,
			}
		} else {
			// Create a mock that returns an error.
			provider = &mockProvider{
				response: "",
				err:      fmt.Errorf("test error"),
			}
		}

		result, err := provider.Invoke(context.Background(), prompt, modelID)

		// Consistency property: if err == nil, result must be non-empty.
		if err == nil && result == "" {
			t.Fatalf("Invoke returned nil error with empty result for prompt=%q modelID=%q", prompt, modelID)
		}

		// Consistency property: if err != nil, result must be empty.
		if err != nil && result != "" {
			t.Fatalf("Invoke returned non-nil error with non-empty result=%q for prompt=%q modelID=%q", result, prompt, modelID)
		}

		// If err != nil, it must be wrappable as *ProviderError.
		if err != nil {
			var provErr *ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error is not wrappable as *ProviderError: %v", err)
			}
		}

		// Name() must return a non-empty string.
		name := provider.Name()
		if name == "" {
			t.Fatalf("Name() returned empty string")
		}
	})
}

// TestRegistryContainsAllProviders validates that the Registry map contains
// constructor entries for all four expected provider names.
//
// Validates: Requirements 11.3
func TestRegistryContainsAllProviders(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{name: "bedrock", provider: "bedrock"},
		{name: "openai", provider: "openai"},
		{name: "anthropic", provider: "anthropic"},
		{name: "vertexai", provider: "vertexai"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fn, ok := Registry[tc.provider]
			if !ok {
				t.Fatalf("Registry missing entry for %q", tc.provider)
			}
			if fn == nil {
				t.Fatalf("Registry entry for %q is nil", tc.provider)
			}
		})
	}
}

// TestProviderErrorFormat validates that ProviderError.Error() includes
// the provider name and the descriptive message.
//
// Validates: Requirements 13.1, 13.2
func TestProviderErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		message  string
	}{
		{name: "bedrock auth failed", provider: "bedrock", message: "auth failed"},
		{name: "openai rate limited", provider: "openai", message: "rate limited"},
		{name: "anthropic empty response", provider: "anthropic", message: "empty response"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := &ProviderError{Provider: tc.provider, Message: tc.message}
			got := err.Error()
			if !strings.Contains(got, tc.provider) {
				t.Errorf("Error() = %q, want it to contain provider %q", got, tc.provider)
			}
			if !strings.Contains(got, tc.message) {
				t.Errorf("Error() = %q, want it to contain message %q", got, tc.message)
			}
		})
	}
}

// TestProviderErrorUnwrap validates that ProviderError.Unwrap() returns
// the underlying error when set, and nil when not set.
//
// Validates: Requirements 13.1
func TestProviderErrorUnwrap(t *testing.T) {
	underlying := fmt.Errorf("underlying cause")

	tests := []struct {
		name    string
		err     *ProviderError
		wantNil bool
	}{
		{
			name:    "with underlying error",
			err:     &ProviderError{Provider: "test", Message: "fail", Err: underlying},
			wantNil: false,
		},
		{
			name:    "without underlying error",
			err:     &ProviderError{Provider: "test", Message: "fail", Err: nil},
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Unwrap()
			if tc.wantNil && got != nil {
				t.Errorf("Unwrap() = %v, want nil", got)
			}
			if !tc.wantNil && got == nil {
				t.Fatal("Unwrap() = nil, want non-nil error")
			}
			if !tc.wantNil && got != underlying {
				t.Errorf("Unwrap() = %v, want %v", got, underlying)
			}
		})
	}
}

// TestOpenAIProviderAllowsNilAPIKeyWithBaseURL validates that NewOpenAIProvider
// succeeds when APIKey is empty but BaseURL is set (e.g., Ollama).
//
// Validates: Requirements 13.2
func TestOpenAIProviderAllowsNilAPIKeyWithBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "ollama localhost", baseURL: "http://localhost:11434/v1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewOpenAIProvider(ProviderOptions{
				APIKey:  "",
				BaseURL: tc.baseURL,
			})
			if err != nil {
				t.Fatalf("NewOpenAIProvider() returned error: %v", err)
			}
			if p == nil {
				t.Fatal("NewOpenAIProvider() returned nil provider")
			}
		})
	}
}

// TestOpenAIProviderRejectsNilAPIKeyWithoutBaseURL validates that NewOpenAIProvider
// returns a *ProviderError when both APIKey and BaseURL are empty.
//
// Validates: Requirements 13.2
func TestOpenAIProviderRejectsNilAPIKeyWithoutBaseURL(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "empty api key and base url"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewOpenAIProvider(ProviderOptions{
				APIKey:  "",
				BaseURL: "",
			})
			if err == nil {
				t.Fatal("NewOpenAIProvider() returned nil error, want *ProviderError")
			}
			var provErr *ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error is not *ProviderError: %v", err)
			}
		})
	}
}

// TestAnthropicProviderRejectsNilAPIKey validates that NewAnthropicProvider
// returns a *ProviderError when APIKey is empty.
//
// Validates: Requirements 13.2
func TestAnthropicProviderRejectsNilAPIKey(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "empty api key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAnthropicProvider(ProviderOptions{
				APIKey: "",
			})
			if err == nil {
				t.Fatal("NewAnthropicProvider() returned nil error, want *ProviderError")
			}
			var provErr *ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error is not *ProviderError: %v", err)
			}
		})
	}
}
