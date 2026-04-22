package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/user/vocabgen/internal/llm"
)

// mockProvider returns a fixed valid JSON response and counts invocations.
type mockProvider struct {
	invocations atomic.Int64
}

func (m *mockProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	m.invocations.Add(1)
	// Extract the word/expression from the prompt to make responses unique per token.
	// The prompt contains the token, so we use a generic response.
	resp := map[string]any{
		"word":               "testword",
		"type":               "znw",
		"article":            "het",
		"definition":         "een definitie",
		"english_definition": "a definition",
		"example":            "een voorbeeld",
		"english":            map[string]any{"primary": "test", "alternatives": ""},
		"target_translation": map[string]any{"primary": "teszt", "alternatives": ""},
		"notes":              "",
		"connotation":        "",
		"register":           "",
		"collocations":       "",
		"contrastive_notes":  "",
		"secondary_meanings": "",
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}

func (m *mockProvider) Name() string { return "mock" }

// mockExprProvider returns valid expression JSON.
type mockExprProvider struct {
	invocations atomic.Int64
}

func (m *mockExprProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	m.invocations.Add(1)
	resp := map[string]any{
		"expression":         "test expression",
		"definition":         "een definitie",
		"english_definition": "a definition",
		"example":            "een voorbeeld",
		"english":            map[string]any{"primary": "test", "alternatives": ""},
		"target_translation": map[string]any{"primary": "teszt", "alternatives": ""},
		"notes":              "",
		"connotation":        "",
		"register":           "",
		"contrastive_notes":  "",
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}

func (m *mockExprProvider) Name() string { return "mock-expr" }

// panicProvider panics if Invoke is called — used for dry-run tests.
type panicProvider struct{}

func (p *panicProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	panic("provider should not be invoked in dry-run mode")
}

func (p *panicProvider) Name() string { return "panic" }

// countingMockProvider counts invocations and returns valid word JSON.
type countingMockProvider struct {
	invocations atomic.Int64
}

func (m *countingMockProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	m.invocations.Add(1)
	resp := map[string]any{
		"word":               "testword",
		"type":               "znw",
		"article":            "het",
		"definition":         "een definitie",
		"english_definition": "a definition",
		"example":            "een voorbeeld",
		"english":            map[string]any{"primary": "test", "alternatives": ""},
		"target_translation": map[string]any{"primary": "teszt", "alternatives": ""},
		"notes":              "",
		"connotation":        "",
		"register":           "",
		"collocations":       "",
		"contrastive_notes":  "",
		"secondary_meanings": "",
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}

func (m *countingMockProvider) Name() string { return "counting-mock" }

// failingMockProvider fails when the prompt contains any token from failTokens.
type failingMockProvider struct {
	response    string
	failTokens  map[string]bool
	invocations atomic.Int64
}

func newFailingMockProvider(failTokens map[string]bool) *failingMockProvider {
	resp := map[string]any{
		"word":               "testword",
		"type":               "znw",
		"article":            "het",
		"definition":         "een definitie",
		"english_definition": "a definition",
		"example":            "een voorbeeld",
		"english":            map[string]any{"primary": "test", "alternatives": ""},
		"target_translation": map[string]any{"primary": "teszt", "alternatives": ""},
		"notes":              "",
		"connotation":        "",
		"register":           "",
		"collocations":       "",
		"contrastive_notes":  "",
		"secondary_meanings": "",
	}
	b, _ := json.Marshal(resp)
	return &failingMockProvider{
		response:   string(b),
		failTokens: failTokens,
	}
}

func (m *failingMockProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	m.invocations.Add(1)
	for token := range m.failTokens {
		if strings.Contains(prompt, token) {
			return "", &llm.ProviderError{Provider: "failing-mock", Message: fmt.Sprintf("simulated failure for token: %s", token)}
		}
	}
	return m.response, nil
}

func (m *failingMockProvider) Name() string { return "failing-mock" }

// mockSentenceProvider returns valid sentence JSON.
type mockSentenceProvider struct {
	invocations atomic.Int64
}

func (m *mockSentenceProvider) Invoke(_ context.Context, _, _ string) (string, error) {
	m.invocations.Add(1)
	resp := map[string]any{
		"sentence":           "Ik ga morgen naar de markt",
		"corrected_sentence": "Ik ga morgen naar de markt.",
		"is_correct":         false,
		"grammar_errors": []map[string]any{
			{
				"error":       "markt",
				"correction":  "markt.",
				"explanation": "Missing period at end of sentence",
			},
		},
		"translation":        map[string]any{"primary": "I am going to the market tomorrow", "alternatives": ""},
		"target_translation": map[string]any{"primary": "Holnap a piacra megyek", "alternatives": ""},
		"key_vocabulary": []map[string]any{
			{"word": "morgen", "definition": "de volgende dag", "english": "tomorrow"},
			{"word": "markt", "definition": "plaats waar handel plaatsvindt", "english": "market"},
		},
		"notes": "informal register",
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}

func (m *mockSentenceProvider) Name() string { return "mock-sentence" }
