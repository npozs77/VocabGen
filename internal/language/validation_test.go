package language

import (
	"encoding/json"
	"errors"
	"testing"

	"pgregory.net/rapid"
)

// TestPropertyP2TranslationFieldNormalization verifies that normalizeTranslation
// produces {Primary: string, Alternatives: string} for both plain strings and
// objects with "primary" and optional "alternatives".
//
// Validates: Requirements 3.3, 3.4
func TestPropertyP2TranslationFieldNormalization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Choose between plain string and object form
		useObject := rapid.Bool().Draw(t, "useObject")
		primary := rapid.String().Draw(t, "primary")

		var rawJSON string
		var expectAlt string

		if useObject {
			includeAlt := rapid.Bool().Draw(t, "includeAlt")
			if includeAlt {
				alt := rapid.String().Draw(t, "alternatives")
				expectAlt = alt
				obj := map[string]string{"primary": primary, "alternatives": alt}
				b, _ := json.Marshal(obj)
				rawJSON = string(b)
			} else {
				expectAlt = ""
				obj := map[string]string{"primary": primary}
				b, _ := json.Marshal(obj)
				rawJSON = string(b)
			}
		} else {
			expectAlt = ""
			b, _ := json.Marshal(primary)
			rawJSON = string(b)
		}

		var raw any
		if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
			t.Fatalf("failed to unmarshal test input: %v", err)
		}

		tr, err := normalizeTranslation("test_field", raw)
		if err != nil {
			t.Fatalf("normalizeTranslation failed: %v", err)
		}

		if tr.Primary != primary {
			t.Errorf("Primary = %q, want %q", tr.Primary, primary)
		}
		if tr.Alternatives != expectAlt {
			t.Errorf("Alternatives = %q, want %q", tr.Alternatives, expectAlt)
		}
	})
}

// helper: builds a valid words JSON object with all required and optional fields.
func validWordsJSON(t *rapid.T) map[string]any {
	m := map[string]any{
		"word":       rapid.String().Draw(t, "word"),
		"type":       rapid.String().Draw(t, "type"),
		"article":    rapid.String().Draw(t, "article"),
		"definition": rapid.String().Draw(t, "definition"),
		"example":    rapid.String().Draw(t, "example"),
	}
	// Translation fields: randomly choose string or object form
	for _, f := range []string{"english", "target_translation"} {
		if rapid.Bool().Draw(t, f+"_object") {
			obj := map[string]any{"primary": rapid.String().Draw(t, f+"_primary")}
			if rapid.Bool().Draw(t, f+"_has_alt") {
				obj["alternatives"] = rapid.String().Draw(t, f+"_alt")
			}
			m[f] = obj
		} else {
			m[f] = rapid.String().Draw(t, f+"_str")
		}
	}
	// Optional fields
	for _, f := range []string{"english_definition", "notes", "connotation", "register", "collocations", "contrastive_notes", "secondary_meanings"} {
		if rapid.Bool().Draw(t, "include_"+f) {
			m[f] = rapid.String().Draw(t, f)
		}
	}
	return m
}

// helper: builds a valid expressions JSON object.
func validExpressionsJSON(t *rapid.T) map[string]any {
	m := map[string]any{
		"expression": rapid.String().Draw(t, "expression"),
		"definition": rapid.String().Draw(t, "definition"),
		"example":    rapid.String().Draw(t, "example"),
	}
	for _, f := range []string{"english", "target_translation"} {
		if rapid.Bool().Draw(t, f+"_object") {
			obj := map[string]any{"primary": rapid.String().Draw(t, f+"_primary")}
			if rapid.Bool().Draw(t, f+"_has_alt") {
				obj["alternatives"] = rapid.String().Draw(t, f+"_alt")
			}
			m[f] = obj
		} else {
			m[f] = rapid.String().Draw(t, f+"_str")
		}
	}
	for _, f := range []string{"english_definition", "notes", "connotation", "register", "contrastive_notes"} {
		if rapid.Bool().Draw(t, "include_"+f) {
			m[f] = rapid.String().Draw(t, f)
		}
	}
	return m
}

// TestPropertyP3OptionalFieldsDefaultToEmptyString verifies that when optional
// fields are absent from valid JSON, the validator succeeds and the missing
// optional fields are "" in the returned struct.
//
// Validates: Requirement 3.5
func TestPropertyP3OptionalFieldsDefaultToEmptyString(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		mode := rapid.SampledFrom([]string{"words", "expressions"}).Draw(t, "mode")

		var m map[string]any
		var optionalFields []string
		if mode == "words" {
			// Build with all required, randomly omit optionals
			m = map[string]any{
				"word":               rapid.String().Draw(t, "word"),
				"type":               rapid.String().Draw(t, "type"),
				"article":            rapid.String().Draw(t, "article"),
				"definition":         rapid.String().Draw(t, "definition"),
				"example":            rapid.String().Draw(t, "example"),
				"english":            rapid.String().Draw(t, "english"),
				"target_translation": rapid.String().Draw(t, "target_translation"),
			}
			optionalFields = []string{"english_definition", "notes", "connotation", "register", "collocations", "contrastive_notes", "secondary_meanings"}
		} else {
			m = map[string]any{
				"expression":         rapid.String().Draw(t, "expression"),
				"definition":         rapid.String().Draw(t, "definition"),
				"example":            rapid.String().Draw(t, "example"),
				"english":            rapid.String().Draw(t, "english"),
				"target_translation": rapid.String().Draw(t, "target_translation"),
			}
			optionalFields = []string{"english_definition", "notes", "connotation", "register", "contrastive_notes"}
		}

		// Randomly remove some optional fields
		var removed []string
		for _, f := range optionalFields {
			if rapid.Bool().Draw(t, "remove_"+f) {
				removed = append(removed, f)
			} else {
				m[f] = rapid.String().Draw(t, f)
			}
		}

		b, _ := json.Marshal(m)
		entry, err := ValidateResponse(mode, string(b))
		if err != nil {
			t.Fatalf("ValidateResponse failed: %v", err)
		}

		// Check removed optional fields are ""
		for _, f := range removed {
			val := getEntryField(entry, f)
			if val != "" {
				t.Errorf("optional field %q should be empty, got %q", f, val)
			}
		}
	})
}

// getEntryField returns the string value of a field by name from ValidatedEntry.
func getEntryField(e *ValidatedEntry, field string) string {
	switch field {
	case "english_definition":
		return e.EnglishDefinition
	case "notes":
		return e.Notes
	case "connotation":
		return e.Connotation
	case "register":
		return e.Register
	case "collocations":
		return e.Collocations
	case "contrastive_notes":
		return e.ContrastiveNotes
	case "secondary_meanings":
		return e.SecondaryMeanings
	default:
		return ""
	}
}

// TestPropertyP4MissingRequiredFieldsReturnValidationError verifies that
// removing a random non-empty subset of required fields causes ValidateResponse
// to return a ValidationError mentioning every removed field.
//
// Also tests non-string optional fields and malformed translation fields.
//
// Validates: Requirements 3.6, 3.7, 3.8, 3.9
func TestPropertyP4MissingRequiredFieldsReturnValidationError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			mode := rapid.SampledFrom([]string{"words", "expressions"}).Draw(t, "mode")

			var m map[string]any
			var required []string
			if mode == "words" {
				m = map[string]any{
					"word":               "test",
					"type":               "noun",
					"article":            "de",
					"definition":         "def",
					"example":            "ex",
					"english":            "eng",
					"target_translation": "tgt",
				}
				required = []string{"word", "type", "article", "definition", "example", "english", "target_translation"}
			} else {
				m = map[string]any{
					"expression":         "test expr",
					"definition":         "def",
					"example":            "ex",
					"english":            "eng",
					"target_translation": "tgt",
				}
				required = []string{"expression", "definition", "example", "english", "target_translation"}
			}

			// Remove a random non-empty subset
			var removed []string
			for _, f := range required {
				if rapid.Bool().Draw(t, "remove_"+f) {
					removed = append(removed, f)
					delete(m, f)
				}
			}
			if len(removed) == 0 {
				// Ensure at least one field is removed
				idx := rapid.IntRange(0, len(required)-1).Draw(t, "forceRemoveIdx")
				removed = append(removed, required[idx])
				delete(m, required[idx])
			}

			b, _ := json.Marshal(m)
			_, err := ValidateResponse(mode, string(b))
			if err == nil {
				t.Fatalf("expected error for missing fields %v, got nil", removed)
			}

			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}

			for _, f := range removed {
				found := false
				for _, ef := range ve.Fields {
					if ef == f {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidationError.Fields missing removed field %q; got %v", f, ve.Fields)
				}
			}
		})
	})

	t.Run("non-string optional field", func(t *testing.T) {
		m := map[string]any{
			"word":               "test",
			"type":               "noun",
			"article":            "de",
			"definition":         "def",
			"example":            "ex",
			"english":            "eng",
			"target_translation": "tgt",
			"notes":              42, // non-string
		}
		b, _ := json.Marshal(m)
		_, err := ValidateResponse("words", string(b))
		if err == nil {
			t.Fatal("expected error for non-string optional field")
		}
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected *ValidationError, got %T", err)
		}
	})

	t.Run("malformed translation field", func(t *testing.T) {
		m := map[string]any{
			"word":               "test",
			"type":               "noun",
			"article":            "de",
			"definition":         "def",
			"example":            "ex",
			"english":            42, // neither string nor valid object
			"target_translation": "tgt",
		}
		b, _ := json.Marshal(m)
		_, err := ValidateResponse("words", string(b))
		if err == nil {
			t.Fatal("expected error for malformed translation field")
		}
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected *ValidationError, got %T", err)
		}
	})

	t.Run("translation object missing primary", func(t *testing.T) {
		m := map[string]any{
			"word":               "test",
			"type":               "noun",
			"article":            "de",
			"definition":         "def",
			"example":            "ex",
			"english":            map[string]any{"alternatives": "alt"},
			"target_translation": "tgt",
		}
		b, _ := json.Marshal(m)
		_, err := ValidateResponse("words", string(b))
		if err == nil {
			t.Fatal("expected error for translation object missing primary")
		}
	})
}

// TestPropertyP9ValidationAcceptsAnyValidEnglishSchemaJSON verifies that
// ValidateResponse succeeds for any complete valid JSON with all required
// English fields as strings and translation fields as string or valid object.
//
// Validates: Requirements 3.1, 3.2, 43.9
func TestPropertyP9ValidationAcceptsAnyValidEnglishSchemaJSON(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		mode := rapid.SampledFrom([]string{"words", "expressions"}).Draw(t, "mode")

		var m map[string]any
		if mode == "words" {
			m = validWordsJSON(t)
		} else {
			m = validExpressionsJSON(t)
		}

		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		entry, err := ValidateResponse(mode, string(b))
		if err != nil {
			t.Fatalf("ValidateResponse failed for valid %s JSON: %v\nJSON: %s", mode, err, string(b))
		}

		// Verify entry is non-nil
		if entry == nil {
			t.Fatal("ValidateResponse returned nil entry without error")
		}
	})
}

// FuzzValidateResponse fuzzes ValidateResponse with random JSON strings to find
// panics or unexpected behavior. Tests both "words" and "expressions" modes.
//
// Validates: Requirements 43.3
func FuzzValidateResponse(f *testing.F) {
	// Seed corpus: valid words JSON
	f.Add("words", `{"word":"huis","type":"zelfstandig naamwoord","article":"het","definition":"een gebouw","example":"Het huis is groot.","english":"house","target_translation":"ház"}`)
	// Seed corpus: valid expressions JSON
	f.Add("expressions", `{"expression":"op de hoogte","definition":"geïnformeerd zijn","example":"Ik ben op de hoogte.","english":"up to date","target_translation":"naprakész"}`)
	// Seed corpus: empty string
	f.Add("words", "")
	f.Add("expressions", "")
	// Seed corpus: empty object
	f.Add("words", "{}")
	f.Add("expressions", "{}")
	// Seed corpus: JSON array
	f.Add("words", "[]")
	f.Add("expressions", "[]")
	// Seed corpus: malformed JSON
	f.Add("words", `{"word":`)
	f.Add("expressions", `not json at all`)
	// Seed corpus: valid JSON with translation objects
	f.Add("words", `{"word":"werk","type":"noun","article":"het","definition":"def","example":"ex","english":{"primary":"work","alternatives":"job"},"target_translation":{"primary":"munka","alternatives":"dolgozat"}}`)
	// Seed corpus: markdown-wrapped JSON
	f.Add("words", "```json\n{\"word\":\"test\",\"type\":\"noun\",\"article\":\"de\",\"definition\":\"d\",\"example\":\"e\",\"english\":\"eng\",\"target_translation\":\"tgt\"}\n```")

	f.Fuzz(func(t *testing.T, mode, rawJSON string) {
		// Force mode to valid values to focus fuzzing on JSON input
		if mode != "words" && mode != "expressions" {
			mode = "words"
		}
		// ValidateResponse must never panic regardless of input
		_, _ = ValidateResponse(mode, rawJSON)
	})
}
