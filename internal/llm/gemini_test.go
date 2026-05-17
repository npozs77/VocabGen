package llm

import (
	"errors"
	"testing"
)

// TestGeminiProviderRejectsEmptyAPIKey validates that NewGeminiProvider
// returns a *ProviderError when APIKey is empty.
//
// Validates: Requirements 69.1, 12.8
func TestGeminiProviderRejectsEmptyAPIKey(t *testing.T) {
	tests := []struct {
		name string
		opts ProviderOptions
	}{
		{
			name: "empty api key",
			opts: ProviderOptions{APIKey: ""},
		},
		{
			name: "all fields empty",
			opts: ProviderOptions{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewGeminiProvider(tc.opts)
			if err == nil {
				t.Fatal("NewGeminiProvider() returned nil error, want *ProviderError")
			}
			var provErr *ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error is not *ProviderError: %v", err)
			}
			if provErr.Provider != "gemini" {
				t.Errorf("ProviderError.Provider = %q, want %q", provErr.Provider, "gemini")
			}
		})
	}
}

// TestGeminiProviderConstructionWithAPIKey validates that NewGeminiProvider
// succeeds when a valid API key is provided.
//
// Validates: Requirements 69.1
func TestGeminiProviderConstructionWithAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{name: "valid api key", apiKey: "AIzaSyTest1234567890"},
		{name: "short api key", apiKey: "key123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewGeminiProvider(ProviderOptions{APIKey: tc.apiKey})
			if err != nil {
				t.Fatalf("NewGeminiProvider() returned error: %v", err)
			}
			if p == nil {
				t.Fatal("NewGeminiProvider() returned nil provider")
			}
		})
	}
}

// TestGeminiProviderName validates that Name() returns "gemini".
//
// Validates: Requirements 69.2
func TestGeminiProviderName(t *testing.T) {
	p, err := NewGeminiProvider(ProviderOptions{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewGeminiProvider() returned error: %v", err)
	}

	got := p.Name()
	if got != "gemini" {
		t.Errorf("Name() = %q, want %q", got, "gemini")
	}
}

// TestGeminiRegistryEntry validates that the Registry contains a "gemini" entry.
//
// Validates: Requirements 11.4
func TestGeminiRegistryEntry(t *testing.T) {
	fn, ok := Registry["gemini"]
	if !ok {
		t.Fatal("Registry missing entry for \"gemini\"")
	}
	if fn == nil {
		t.Fatal("Registry entry for \"gemini\" is nil")
	}
}
