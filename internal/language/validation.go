package language

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Translation holds a normalized translation with primary and alternatives.
type Translation struct {
	Primary      string
	Alternatives string
}

// ValidatedEntry holds the validated and normalized data from an LLM response.
// Translation fields are always normalized to {Primary, Alternatives} form.
type ValidatedEntry struct {
	// Words fields
	Word string
	// Expressions fields
	Expression string

	Type              string
	Article           string
	Definition        string
	EnglishDefinition string
	Example           string
	English           Translation
	TargetTranslation Translation
	Notes             string
	Connotation       string
	Register          string
	Collocations      string // words only
	ContrastiveNotes  string
	SecondaryMeanings string // words only

	// Sentence fields (ephemeral, not stored in DB)
	Sentence          string
	CorrectedSentence string
	IsCorrect         bool
	GrammarErrors     []GrammarError
	KeyVocabulary     []VocabItem
}

// GrammarError represents a single grammar error found in a sentence.
type GrammarError struct {
	Error       string `json:"error"`
	Correction  string `json:"correction"`
	Explanation string `json:"explanation"`
}

// VocabItem represents a key vocabulary item extracted from a sentence.
type VocabItem struct {
	Word       string `json:"word"`
	Definition string `json:"definition"`
	English    string `json:"english"`
}

// ValidationError is returned when LLM JSON doesn't match the expected schema.
type ValidationError struct {
	Message string
	Fields  []string
}

// Error returns the validation error message.
func (e *ValidationError) Error() string { return e.Message }

// wordsRequired lists required fields for words mode.
var wordsRequired = []string{"word", "type", "article", "definition", "example", "english", "target_translation"}

// expressionsRequired lists required fields for expressions mode.
var expressionsRequired = []string{"expression", "definition", "example", "english", "target_translation"}

// wordsOptional lists optional fields for words mode.
var wordsOptional = []string{"english_definition", "notes", "connotation", "register", "collocations", "contrastive_notes", "secondary_meanings"}

// expressionsOptional lists optional fields for expressions mode.
var expressionsOptional = []string{"english_definition", "notes", "connotation", "register", "contrastive_notes"}

// sentenceRequired lists required fields for sentences mode.
var sentenceRequired = []string{"sentence", "corrected_sentence", "is_correct", "grammar_errors", "translation", "target_translation", "key_vocabulary"}

// sentenceOptional lists optional fields for sentences mode.
var sentenceOptional = []string{"notes"}

// translationFields are fields that accept string or {primary, alternatives} object.
var translationFields = map[string]bool{"english": true, "target_translation": true}

// normalizeTranslation converts a raw JSON value into a Translation.
// Accepts plain strings or objects with "primary" and optional "alternatives".
func normalizeTranslation(field string, raw any) (Translation, error) {
	switch v := raw.(type) {
	case string:
		return Translation{Primary: v, Alternatives: ""}, nil
	case map[string]any:
		primary, ok := v["primary"]
		if !ok {
			return Translation{}, fmt.Errorf("translation field %q missing \"primary\" key", field)
		}
		ps, ok := primary.(string)
		if !ok {
			return Translation{}, fmt.Errorf("translation field %q: \"primary\" must be a string", field)
		}
		alt := ""
		if a, exists := v["alternatives"]; exists {
			as, ok := a.(string)
			if !ok {
				return Translation{}, fmt.Errorf("translation field %q: \"alternatives\" must be a string", field)
			}
			alt = as
		}
		return Translation{Primary: ps, Alternatives: alt}, nil
	default:
		return Translation{}, fmt.Errorf("translation field %q must be a string or object with \"primary\", got %T", field, raw)
	}
}

// ValidateResponse parses raw JSON and validates against the English schema
// for the given mode ("words" or "expressions").
// Normalizes translation fields and defaults optional fields to "".
func ValidateResponse(mode, rawJSON string) (*ValidatedEntry, error) {
	// Strip markdown code fences if present (LLMs sometimes wrap JSON in ```json ... ```)
	cleaned := strings.TrimSpace(rawJSON)
	if strings.HasPrefix(cleaned, "```") {
		// Remove opening fence (```json or ```)
		if idx := strings.Index(cleaned, "\n"); idx != -1 {
			cleaned = cleaned[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(cleaned), &data); err != nil {
		return nil, &ValidationError{Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	var required, optional []string
	switch mode {
	case "words":
		required = wordsRequired
		optional = wordsOptional
	case "expressions":
		required = expressionsRequired
		optional = expressionsOptional
	case "sentences":
		return validateSentenceResponse(data)
	default:
		return nil, fmt.Errorf("invalid mode: %q", mode)
	}

	// Check required fields
	var missing []string
	for _, f := range required {
		if _, exists := data[f]; !exists {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return nil, &ValidationError{
			Message: fmt.Sprintf("missing required fields: %s", strings.Join(missing, ", ")),
			Fields:  missing,
		}
	}

	// Validate required string fields (non-translation)
	for _, f := range required {
		if translationFields[f] {
			continue
		}
		if _, ok := data[f].(string); !ok {
			return nil, &ValidationError{
				Message: fmt.Sprintf("field %q: expected string, got %T", f, data[f]),
				Fields:  []string{f},
			}
		}
	}

	// Validate and normalize translation fields
	english, err := normalizeTranslation("english", data["english"])
	if err != nil {
		return nil, &ValidationError{Message: err.Error(), Fields: []string{"english"}}
	}
	target, err := normalizeTranslation("target_translation", data["target_translation"])
	if err != nil {
		return nil, &ValidationError{Message: err.Error(), Fields: []string{"target_translation"}}
	}

	// Validate optional fields: default to "" if absent, coerce arrays to
	// comma-separated strings (LLMs sometimes return lists), error on other types.
	for _, f := range optional {
		v, exists := data[f]
		if !exists || v == nil {
			data[f] = ""
			continue
		}
		switch val := v.(type) {
		case string:
			// already fine
		case []any:
			// Coerce JSON array to comma-separated string.
			parts := make([]string, 0, len(val))
			for _, item := range val {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			data[f] = strings.Join(parts, ", ")
		default:
			return nil, &ValidationError{
				Message: fmt.Sprintf("field %q: expected string, got %T", f, v),
				Fields:  []string{f},
			}
		}
	}

	getString := func(key string) string {
		if s, ok := data[key].(string); ok {
			return s
		}
		return ""
	}

	entry := &ValidatedEntry{
		English:           english,
		TargetTranslation: target,
		Definition:        getString("definition"),
		EnglishDefinition: getString("english_definition"),
		Example:           getString("example"),
		Notes:             getString("notes"),
		Connotation:       getString("connotation"),
		Register:          getString("register"),
		ContrastiveNotes:  getString("contrastive_notes"),
	}

	if mode == "words" {
		entry.Word = getString("word")
		entry.Type = getString("type")
		entry.Article = getString("article")
		entry.Collocations = getString("collocations")
		entry.SecondaryMeanings = getString("secondary_meanings")
	} else {
		entry.Expression = getString("expression")
	}

	return entry, nil
}

// validateSentenceResponse validates and extracts sentence-specific fields from
// the parsed JSON data. Sentence responses have a different structure than
// words/expressions: they include grammar_errors (array), key_vocabulary (array),
// is_correct (bool), and translation fields.
func validateSentenceResponse(data map[string]any) (*ValidatedEntry, error) {
	// Check required fields
	var missing []string
	for _, f := range sentenceRequired {
		if _, exists := data[f]; !exists {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return nil, &ValidationError{
			Message: fmt.Sprintf("missing required fields: %s", strings.Join(missing, ", ")),
			Fields:  missing,
		}
	}

	getString := func(key string) string {
		if s, ok := data[key].(string); ok {
			return s
		}
		return ""
	}

	// Validate sentence (string)
	if _, ok := data["sentence"].(string); !ok {
		return nil, &ValidationError{
			Message: fmt.Sprintf("field %q: expected string, got %T", "sentence", data["sentence"]),
			Fields:  []string{"sentence"},
		}
	}

	// Validate corrected_sentence (string)
	if _, ok := data["corrected_sentence"].(string); !ok {
		return nil, &ValidationError{
			Message: fmt.Sprintf("field %q: expected string, got %T", "corrected_sentence", data["corrected_sentence"]),
			Fields:  []string{"corrected_sentence"},
		}
	}

	// Validate is_correct (bool)
	isCorrect, ok := data["is_correct"].(bool)
	if !ok {
		return nil, &ValidationError{
			Message: fmt.Sprintf("field %q: expected boolean, got %T", "is_correct", data["is_correct"]),
			Fields:  []string{"is_correct"},
		}
	}

	// Validate and parse grammar_errors (array of objects)
	grammarRaw, ok := data["grammar_errors"].([]any)
	if !ok {
		return nil, &ValidationError{
			Message: fmt.Sprintf("field %q: expected array, got %T", "grammar_errors", data["grammar_errors"]),
			Fields:  []string{"grammar_errors"},
		}
	}
	grammarErrors := make([]GrammarError, 0, len(grammarRaw))
	for i, item := range grammarRaw {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, &ValidationError{
				Message: fmt.Sprintf("grammar_errors[%d]: expected object, got %T", i, item),
				Fields:  []string{"grammar_errors"},
			}
		}
		ge := GrammarError{}
		if v, ok := obj["error"].(string); ok {
			ge.Error = v
		}
		if v, ok := obj["correction"].(string); ok {
			ge.Correction = v
		}
		if v, ok := obj["explanation"].(string); ok {
			ge.Explanation = v
		}
		grammarErrors = append(grammarErrors, ge)
	}

	// Validate and normalize translation fields
	translation, err := normalizeTranslation("translation", data["translation"])
	if err != nil {
		return nil, &ValidationError{Message: err.Error(), Fields: []string{"translation"}}
	}
	targetTranslation, err := normalizeTranslation("target_translation", data["target_translation"])
	if err != nil {
		return nil, &ValidationError{Message: err.Error(), Fields: []string{"target_translation"}}
	}

	// Validate and parse key_vocabulary (array of objects)
	vocabRaw, ok := data["key_vocabulary"].([]any)
	if !ok {
		return nil, &ValidationError{
			Message: fmt.Sprintf("field %q: expected array, got %T", "key_vocabulary", data["key_vocabulary"]),
			Fields:  []string{"key_vocabulary"},
		}
	}
	keyVocab := make([]VocabItem, 0, len(vocabRaw))
	for i, item := range vocabRaw {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, &ValidationError{
				Message: fmt.Sprintf("key_vocabulary[%d]: expected object, got %T", i, item),
				Fields:  []string{"key_vocabulary"},
			}
		}
		vi := VocabItem{}
		if v, ok := obj["word"].(string); ok {
			vi.Word = v
		}
		if v, ok := obj["definition"].(string); ok {
			vi.Definition = v
		}
		if v, ok := obj["english"].(string); ok {
			vi.English = v
		}
		keyVocab = append(keyVocab, vi)
	}

	// Default optional fields to "" if absent.
	for _, f := range sentenceOptional {
		if _, exists := data[f]; !exists {
			data[f] = ""
		}
	}

	entry := &ValidatedEntry{
		Sentence:          getString("sentence"),
		CorrectedSentence: getString("corrected_sentence"),
		IsCorrect:         isCorrect,
		GrammarErrors:     grammarErrors,
		English:           translation,
		TargetTranslation: targetTranslation,
		KeyVocabulary:     keyVocab,
		Notes:             getString("notes"),
	}

	return entry, nil
}
