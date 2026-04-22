package language

import (
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// unresolvedPlaceholder matches any remaining {placeholder} in output.
var unresolvedPlaceholder = regexp.MustCompile(`\{[a-z_]+\}`)

// TestPropertyP1TemplateFormattingProducesValidPrompts verifies that for any
// source language (known code, arbitrary name, Unicode), BuildPrompt produces
// output containing the resolved language name, token, target language name,
// Core Rule Block text, Decision Rubric text, and no unresolved placeholders.
//
// Validates: Requirements 1.10, 2.9, 4.5
func TestPropertyP1TemplateFormattingProducesValidPrompts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate source language: mix of known codes and arbitrary strings
		sourceLang := rapid.OneOf(
			rapid.SampledFrom([]string{"nl", "hu", "it", "ru", "en", "de", "fr", "es", "pt", "pl", "tr"}),
			rapid.StringMatching(`[A-Za-z\x{00C0}-\x{024F}]{1,20}`),
			rapid.StringMatching(`[\x{3040}-\x{309F}]{1,10}`), // Hiragana
		).Draw(t, "sourceLang")

		mode := rapid.SampledFrom([]string{"words", "expressions", "sentences"}).Draw(t, "mode")
		token := rapid.StringMatching(`.{1,50}`).Draw(t, "token")
		context := rapid.StringMatching(`.{0,100}`).Draw(t, "context")
		targetLang := rapid.OneOf(
			rapid.SampledFrom([]string{"hu", "en", "de", "fr"}),
			rapid.StringMatching(`[a-z]{2,15}`),
		).Draw(t, "targetLang")

		result, err := BuildPrompt(sourceLang, mode, token, context, targetLang)
		if err != nil {
			t.Fatalf("BuildPrompt returned error: %v", err)
		}

		resolvedSource := ResolveLanguageName(sourceLang)
		resolvedTarget := ResolveLanguageName(targetLang)

		if !strings.Contains(result, resolvedSource) {
			t.Errorf("output missing resolved source language %q", resolvedSource)
		}
		if !strings.Contains(result, token) {
			t.Errorf("output missing token %q", token)
		}
		if !strings.Contains(result, resolvedTarget) {
			t.Errorf("output missing resolved target language %q", resolvedTarget)
		}
		// CORE RULES and DECISION RUBRIC are only in words/expressions templates.
		if mode != "sentences" {
			if !strings.Contains(result, "CORE RULES:") {
				t.Error("output missing Core Rule Block")
			}
			if !strings.Contains(result, "DECISION RUBRIC") {
				t.Error("output missing Decision Rubric")
			}
		}
		if matches := unresolvedPlaceholder.FindAllString(result, -1); len(matches) > 0 {
			t.Errorf("output contains unresolved placeholders: %v", matches)
		}
	})
}

// TestPropertyP5BuildPromptInjectsAllParameters verifies that BuildPrompt
// injects the resolved source language, token, context (when non-empty),
// and resolved target language into the output for any random inputs.
//
// Validates: Requirements 6.1–6.7, 42.1–42.4
func TestPropertyP5BuildPromptInjectsAllParameters(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sourceLang := rapid.OneOf(
			rapid.SampledFrom([]string{"nl", "hu", "it", "ru", "en", "de"}),
			rapid.StringMatching(`[A-Za-z]{2,20}`),
		).Draw(t, "sourceLang")

		mode := rapid.SampledFrom([]string{"words", "expressions", "sentences"}).Draw(t, "mode")
		token := rapid.StringMatching(`[A-Za-z\x{00C0}-\x{024F}]{1,30}`).Draw(t, "token")
		context := rapid.StringMatching(`[A-Za-z ]{0,60}`).Draw(t, "context")
		targetLang := rapid.OneOf(
			rapid.SampledFrom([]string{"hu", "en", "de", "fr"}),
			rapid.StringMatching(`[a-z]{2,10}`),
		).Draw(t, "targetLang")

		result, err := BuildPrompt(sourceLang, mode, token, context, targetLang)
		if err != nil {
			t.Fatalf("BuildPrompt returned error: %v", err)
		}

		resolvedSource := ResolveLanguageName(sourceLang)
		resolvedTarget := ResolveLanguageName(targetLang)

		if !strings.Contains(result, resolvedSource) {
			t.Errorf("output missing resolved source language %q", resolvedSource)
		}
		if !strings.Contains(result, token) {
			t.Errorf("output missing token %q", token)
		}
		// Sentence template doesn't use a {context} placeholder.
		if mode != "sentences" && context != "" && !strings.Contains(result, context) {
			t.Errorf("output missing context %q", context)
		}
		if !strings.Contains(result, resolvedTarget) {
			t.Errorf("output missing resolved target language %q", resolvedTarget)
		}
	})
}

// TestWordsTemplateContainsAllFieldNames verifies that WordsTemplate mentions
// all 14 English field names from the words schema.
//
// Validates: Requirement 1.2
func TestWordsTemplateContainsAllFieldNames(t *testing.T) {
	fields := []string{
		"word", "type", "article", "definition", "english_definition",
		"example", "english", "target_translation", "notes", "connotation",
		"register", "collocations", "contrastive_notes", "secondary_meanings",
	}
	for _, f := range fields {
		if !strings.Contains(WordsTemplate, `"`+f+`"`) {
			t.Errorf("WordsTemplate missing field name %q", f)
		}
	}
}

// TestExpressionsTemplateContainsAllFieldNames verifies that ExpressionsTemplate
// mentions all 10 English field names from the expressions schema.
//
// Validates: Requirement 2.2
func TestExpressionsTemplateContainsAllFieldNames(t *testing.T) {
	fields := []string{
		"expression", "definition", "english_definition", "example",
		"english", "target_translation", "notes", "connotation",
		"register", "contrastive_notes",
	}
	for _, f := range fields {
		if !strings.Contains(ExpressionsTemplate, `"`+f+`"`) {
			t.Errorf("ExpressionsTemplate missing field name %q", f)
		}
	}
}

// TestBuildPromptModeSelection verifies that mode "words" uses WordsTemplate
// and mode "expressions" uses ExpressionsTemplate.
//
// Validates: Requirements 6.4, 6.7
func TestBuildPromptModeSelection(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		wantContain string
		wantErr     bool
	}{
		{"words mode uses word placeholder", "words", `word or phrase: "test"`, false},
		{"expressions mode uses expression placeholder", "expressions", `expression: "test"`, false},
		{"sentences mode uses sentence placeholder", "sentences", `sentence: "test"`, false},
		{"invalid mode returns error", "invalid", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := BuildPrompt("nl", tc.mode, "test", "", "hu")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error for invalid mode, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result, tc.wantContain) {
				t.Errorf("output missing expected content %q", tc.wantContain)
			}
		})
	}
}

// TestBuildPromptPerLanguage verifies that BuildPrompt for each supported
// language produces a prompt containing the correct full language name.
//
// Validates: Requirements 43.4, 43.6
func TestBuildPromptPerLanguage(t *testing.T) {
	for code, name := range SupportedLanguages {
		t.Run(code+"→"+name, func(t *testing.T) {
			result, err := BuildPrompt(code, "words", "test", "", "en")
			if err != nil {
				t.Fatalf("BuildPrompt error: %v", err)
			}
			if !strings.Contains(result, name) {
				t.Errorf("prompt for %q missing language name %q", code, name)
			}
		})
	}
}

// TestSentenceTemplateContainsAllFieldNames verifies that SentenceTemplate
// mentions all required field names from the sentence schema.
//
// Validates: Issue #26
func TestSentenceTemplateContainsAllFieldNames(t *testing.T) {
	fields := []string{
		"sentence", "corrected_sentence", "is_correct", "grammar_errors",
		"translation", "target_translation", "key_vocabulary", "notes",
	}
	for _, f := range fields {
		if !strings.Contains(SentenceTemplate, `"`+f+`"`) {
			t.Errorf("SentenceTemplate missing field name %q", f)
		}
	}
}

// TestBuildPromptSentenceMode verifies that mode "sentences" uses SentenceTemplate.
//
// Validates: Issue #26
func TestBuildPromptSentenceMode(t *testing.T) {
	result, err := BuildPrompt("nl", "sentences", "Ik ga morgen naar de markt", "", "hu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "analyze a sentence") {
		t.Error("sentences mode should use SentenceTemplate")
	}
	if !strings.Contains(result, "Ik ga morgen naar de markt") {
		t.Error("output missing the sentence token")
	}
	if !strings.Contains(result, "Dutch") {
		t.Error("output missing resolved source language")
	}
	if !strings.Contains(result, "Hungarian") {
		t.Error("output missing resolved target language")
	}
}

// TestPropertyP20_SentenceTemplateProducesValidPrompts verifies that for any
// source language and sentence, BuildPrompt with mode "sentences" produces
// output containing the resolved language name, sentence, target language,
// and no unresolved placeholders.
//
// Validates: Issue #26
func TestPropertyP20_SentenceTemplateProducesValidPrompts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sourceLang := rapid.OneOf(
			rapid.SampledFrom([]string{"nl", "hu", "it", "ru", "en", "de", "fr", "es"}),
			rapid.StringMatching(`[A-Za-z]{2,15}`),
		).Draw(t, "sourceLang")

		sentence := rapid.StringMatching(`.{1,80}`).Draw(t, "sentence")
		targetLang := rapid.OneOf(
			rapid.SampledFrom([]string{"hu", "en", "de", "fr"}),
			rapid.StringMatching(`[a-z]{2,10}`),
		).Draw(t, "targetLang")

		result, err := BuildPrompt(sourceLang, "sentences", sentence, "", targetLang)
		if err != nil {
			t.Fatalf("BuildPrompt returned error: %v", err)
		}

		resolvedSource := ResolveLanguageName(sourceLang)
		resolvedTarget := ResolveLanguageName(targetLang)

		if !strings.Contains(result, resolvedSource) {
			t.Errorf("output missing resolved source language %q", resolvedSource)
		}
		if !strings.Contains(result, sentence) {
			t.Errorf("output missing sentence %q", sentence)
		}
		if !strings.Contains(result, resolvedTarget) {
			t.Errorf("output missing resolved target language %q", resolvedTarget)
		}
		if matches := unresolvedPlaceholder.FindAllString(result, -1); len(matches) > 0 {
			t.Errorf("output contains unresolved placeholders: %v", matches)
		}
	})
}
