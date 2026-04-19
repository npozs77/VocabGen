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
