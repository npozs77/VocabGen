package output

import (
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/language"
	"pgregory.net/rapid"
)

// --- Property test P7: Field mapper pass-through preserves non-translation fields ---

func TestPropertyP7_MapFieldsPassThrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		mode := rapid.SampledFrom([]string{"words", "expressions", "sentences"}).Draw(t, "mode")

		v := &language.ValidatedEntry{
			Word:              rapid.String().Draw(t, "word"),
			Expression:        rapid.String().Draw(t, "expression"),
			Sentence:          rapid.String().Draw(t, "sentence"),
			Type:              rapid.String().Draw(t, "type"),
			Article:           rapid.String().Draw(t, "article"),
			Definition:        rapid.String().Draw(t, "definition"),
			EnglishDefinition: rapid.String().Draw(t, "english_definition"),
			Example:           rapid.String().Draw(t, "example"),
			English:           language.Translation{Primary: rapid.String().Draw(t, "eng_p"), Alternatives: rapid.String().Draw(t, "eng_a")},
			TargetTranslation: language.Translation{Primary: rapid.String().Draw(t, "tgt_p"), Alternatives: rapid.String().Draw(t, "tgt_a")},
			Notes:             rapid.String().Draw(t, "notes"),
			Connotation:       rapid.String().Draw(t, "connotation"),
			Register:          rapid.String().Draw(t, "register"),
			Collocations:      rapid.String().Draw(t, "collocations"),
			ContrastiveNotes:  rapid.String().Draw(t, "contrastive_notes"),
			SecondaryMeanings: rapid.String().Draw(t, "secondary_meanings"),
		}

		e := MapFields(v, mode)

		if mode == "sentences" {
			// Sentence mode only passes through sentence-specific fields + Notes.
			if e.Notes != v.Notes {
				t.Errorf("Notes: got %q, want %q", e.Notes, v.Notes)
			}
			if e.Sentence != v.Sentence {
				t.Errorf("Sentence: got %q, want %q", e.Sentence, v.Sentence)
			}
			return
		}

		// Non-translation fields pass through as-is (words/expressions modes).
		if e.Definition != v.Definition {
			t.Errorf("Definition: got %q, want %q", e.Definition, v.Definition)
		}
		if e.EnglishDefinition != v.EnglishDefinition {
			t.Errorf("EnglishDefinition: got %q, want %q", e.EnglishDefinition, v.EnglishDefinition)
		}
		if e.Example != v.Example {
			t.Errorf("Example: got %q, want %q", e.Example, v.Example)
		}
		if e.Notes != v.Notes {
			t.Errorf("Notes: got %q, want %q", e.Notes, v.Notes)
		}
		if e.Connotation != v.Connotation {
			t.Errorf("Connotation: got %q, want %q", e.Connotation, v.Connotation)
		}
		if e.Register != v.Register {
			t.Errorf("Register: got %q, want %q", e.Register, v.Register)
		}
		if e.ContrastiveNotes != v.ContrastiveNotes {
			t.Errorf("ContrastiveNotes: got %q, want %q", e.ContrastiveNotes, v.ContrastiveNotes)
		}

		switch mode {
		case "words":
			if e.Word != v.Word {
				t.Errorf("Word: got %q, want %q", e.Word, v.Word)
			}
			if e.Type != v.Type {
				t.Errorf("Type: got %q, want %q", e.Type, v.Type)
			}
			if e.Article != v.Article {
				t.Errorf("Article: got %q, want %q", e.Article, v.Article)
			}
			if e.Collocations != v.Collocations {
				t.Errorf("Collocations: got %q, want %q", e.Collocations, v.Collocations)
			}
			if e.SecondaryMeanings != v.SecondaryMeanings {
				t.Errorf("SecondaryMeanings: got %q, want %q", e.SecondaryMeanings, v.SecondaryMeanings)
			}
		case "expressions":
			if e.Expression != v.Expression {
				t.Errorf("Expression: got %q, want %q", e.Expression, v.Expression)
			}
		}
	})
}

// --- Property test P8: Translation flattening ---

func TestPropertyP8_TranslationFlattening(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		primary := rapid.String().Draw(t, "primary")
		alternatives := rapid.String().Draw(t, "alternatives")

		tr := language.Translation{Primary: primary, Alternatives: alternatives}
		got := FlattenTranslation(tr)

		if alternatives != "" {
			want := primary + " (" + alternatives + ")"
			if got != want {
				t.Errorf("FlattenTranslation(%+v) = %q, want %q", tr, got, want)
			}
		} else if got != primary {
			t.Errorf("FlattenTranslation(%+v) = %q, want %q", tr, got, primary)
		}
	})
}

// --- Table-driven tests for MapFields and FlattenTranslation ---

func TestFlattenTranslation(t *testing.T) {
	tests := []struct {
		name string
		in   language.Translation
		want string
	}{
		{
			name: "empty alternatives returns primary only",
			in:   language.Translation{Primary: "work", Alternatives: ""},
			want: "work",
		},
		{
			name: "non-empty alternatives returns primary (alternatives)",
			in:   language.Translation{Primary: "work", Alternatives: "labor; toil"},
			want: "work (labor; toil)",
		},
		{
			name: "empty primary and alternatives",
			in:   language.Translation{Primary: "", Alternatives: ""},
			want: "",
		},
		{
			name: "empty primary with alternatives",
			in:   language.Translation{Primary: "", Alternatives: "alt"},
			want: " (alt)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FlattenTranslation(tc.in)
			if got != tc.want {
				t.Errorf("FlattenTranslation(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapFields_WordsMode(t *testing.T) {
	v := &language.ValidatedEntry{
		Word:              "werk",
		Type:              "zelfstandig naamwoord",
		Article:           "het",
		Definition:        "activiteit",
		EnglishDefinition: "work or labor",
		Example:           "Ik ga naar het werk.",
		English:           language.Translation{Primary: "work", Alternatives: "labor"},
		TargetTranslation: language.Translation{Primary: "munka", Alternatives: ""},
		Notes:             "common",
		Connotation:       "neutral",
		Register:          "neutraal",
		Collocations:      "aan het werk; op het werk",
		ContrastiveNotes:  "baan: refers to the job position",
		SecondaryMeanings: "literary work; mechanism",
	}

	e := MapFields(v, "words")

	if e.Word != "werk" {
		t.Errorf("Word: got %q, want %q", e.Word, "werk")
	}
	if e.Collocations != "aan het werk; op het werk" {
		t.Errorf("Collocations: got %q", e.Collocations)
	}
	if e.SecondaryMeanings != "literary work; mechanism" {
		t.Errorf("SecondaryMeanings: got %q", e.SecondaryMeanings)
	}
	if e.English != "work (labor)" {
		t.Errorf("English: got %q, want %q", e.English, "work (labor)")
	}
	if e.TargetTranslation != "munka" {
		t.Errorf("TargetTranslation: got %q, want %q", e.TargetTranslation, "munka")
	}
	if e.Expression != "" {
		t.Errorf("Expression should be empty in words mode, got %q", e.Expression)
	}
}

func TestMapFields_ExpressionsMode(t *testing.T) {
	v := &language.ValidatedEntry{
		Expression:        "aan het werk",
		Definition:        "bezig met werken",
		EnglishDefinition: "at work, busy working",
		Example:           "Ze is aan het werk.",
		English:           language.Translation{Primary: "at work", Alternatives: "working"},
		TargetTranslation: language.Translation{Primary: "dolgozik", Alternatives: ""},
		Notes:             "",
		Connotation:       "neutral",
		Register:          "neutraal",
		Collocations:      "should not appear",
		ContrastiveNotes:  "op het werk: at the workplace",
		SecondaryMeanings: "should not appear",
	}

	e := MapFields(v, "expressions")

	if e.Expression != "aan het werk" {
		t.Errorf("Expression: got %q, want %q", e.Expression, "aan het werk")
	}
	// Expressions mode omits collocations and secondary_meanings.
	if e.Collocations != "" {
		t.Errorf("Collocations should be empty in expressions mode, got %q", e.Collocations)
	}
	if e.SecondaryMeanings != "" {
		t.Errorf("SecondaryMeanings should be empty in expressions mode, got %q", e.SecondaryMeanings)
	}
	if e.Word != "" {
		t.Errorf("Word should be empty in expressions mode, got %q", e.Word)
	}
	if e.English != "at work (working)" {
		t.Errorf("English: got %q, want %q", e.English, "at work (working)")
	}
}

// --- Fuzz test for FlattenTranslation ---
// Validates: Requirements 43.3
// Fuzz with random Translation structs to find panics.

func FuzzFlattenTranslation(f *testing.F) {
	// Seed corpus: empty strings, normal strings, long strings, Unicode, special chars, parentheses
	f.Add("", "")
	f.Add("work", "")
	f.Add("work", "labor; toil")
	f.Add("", "alternatives only")
	f.Add("hello world", "hi; hey; greetings")
	f.Add(strings.Repeat("a", 10000), strings.Repeat("b", 10000))
	f.Add("ëüőű", "àéîõ")
	f.Add("café", "kávéház")
	f.Add("naïve", "résumé")
	f.Add("(parentheses)", "(in) alternatives")
	f.Add("special!@#$%^&*", "chars<>{}[]|\\")
	f.Add("new\nline", "tab\there")
	f.Add("null\x00byte", "zero\x00width")

	f.Fuzz(func(t *testing.T, primary, alternatives string) {
		tr := language.Translation{Primary: primary, Alternatives: alternatives}

		// Must never panic
		got := FlattenTranslation(tr)

		// Result must contain the primary string
		if !strings.Contains(got, primary) {
			t.Errorf("result %q does not contain primary %q", got, primary)
		}

		// When alternatives is non-empty, result must use parens format
		if alternatives != "" {
			expected := primary + " (" + alternatives + ")"
			if got != expected {
				t.Errorf("FlattenTranslation(%q, %q) = %q, want %q", primary, alternatives, got, expected)
			}
		} else if got != primary {
			t.Errorf("FlattenTranslation(%q, %q) = %q, want %q", primary, alternatives, got, primary)
		}
	})
}

// TestMapFields_SentencesMode verifies that MapFields correctly maps
// sentence-specific fields in "sentences" mode.
//
// Validates: Issue #26
func TestMapFields_SentencesMode(t *testing.T) {
	v := &language.ValidatedEntry{
		Sentence:          "Ik ga morgen naar de markt",
		CorrectedSentence: "Ik ga morgen naar de markt.",
		IsCorrect:         false,
		GrammarErrors: []language.GrammarError{
			{Error: "markt", Correction: "markt.", Explanation: "Missing period"},
		},
		English:           language.Translation{Primary: "I go to the market tomorrow", Alternatives: ""},
		TargetTranslation: language.Translation{Primary: "Holnap a piacra megyek", Alternatives: ""},
		KeyVocabulary: []language.VocabItem{
			{Word: "morgen", Definition: "de volgende dag", English: "tomorrow"},
		},
		Notes: "informal",
	}

	e := MapFields(v, "sentences")

	if e.Sentence != "Ik ga morgen naar de markt" {
		t.Errorf("Sentence: got %q, want %q", e.Sentence, "Ik ga morgen naar de markt")
	}
	if e.CorrectedSentence != "Ik ga morgen naar de markt." {
		t.Errorf("CorrectedSentence: got %q", e.CorrectedSentence)
	}
	if e.IsCorrect {
		t.Error("IsCorrect should be false")
	}
	if len(e.GrammarErrors) != 1 {
		t.Fatalf("expected 1 grammar error, got %d", len(e.GrammarErrors))
	}
	if e.English != "I go to the market tomorrow" {
		t.Errorf("English: got %q", e.English)
	}
	if e.TargetTranslation != "Holnap a piacra megyek" {
		t.Errorf("TargetTranslation: got %q", e.TargetTranslation)
	}
	if len(e.KeyVocabulary) != 1 {
		t.Fatalf("expected 1 key vocabulary item, got %d", len(e.KeyVocabulary))
	}
	if e.Notes != "informal" {
		t.Errorf("Notes: got %q", e.Notes)
	}
	// Word/expression fields should be empty in sentences mode.
	if e.Word != "" {
		t.Errorf("Word should be empty in sentences mode, got %q", e.Word)
	}
	if e.Expression != "" {
		t.Errorf("Expression should be empty in sentences mode, got %q", e.Expression)
	}
	if e.Definition != "" {
		t.Errorf("Definition should be empty in sentences mode, got %q", e.Definition)
	}
}

// TestPropertyP22_MapFieldsSentencePassThrough verifies that MapFields in
// "sentences" mode correctly passes through all sentence-specific fields.
//
// Validates: Issue #26
func TestPropertyP22_MapFieldsSentencePassThrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := &language.ValidatedEntry{
			Sentence:          rapid.String().Draw(t, "sentence"),
			CorrectedSentence: rapid.String().Draw(t, "corrected"),
			IsCorrect:         rapid.Bool().Draw(t, "is_correct"),
			English:           language.Translation{Primary: rapid.String().Draw(t, "eng_p"), Alternatives: rapid.String().Draw(t, "eng_a")},
			TargetTranslation: language.Translation{Primary: rapid.String().Draw(t, "tgt_p"), Alternatives: rapid.String().Draw(t, "tgt_a")},
			Notes:             rapid.String().Draw(t, "notes"),
		}

		e := MapFields(v, "sentences")

		if e.Sentence != v.Sentence {
			t.Errorf("Sentence: got %q, want %q", e.Sentence, v.Sentence)
		}
		if e.CorrectedSentence != v.CorrectedSentence {
			t.Errorf("CorrectedSentence: got %q, want %q", e.CorrectedSentence, v.CorrectedSentence)
		}
		if e.IsCorrect != v.IsCorrect {
			t.Errorf("IsCorrect: got %v, want %v", e.IsCorrect, v.IsCorrect)
		}
		if e.Notes != v.Notes {
			t.Errorf("Notes: got %q, want %q", e.Notes, v.Notes)
		}
		// Word/expression fields must be empty.
		if e.Word != "" || e.Expression != "" || e.Definition != "" {
			t.Error("word/expression/definition fields should be empty in sentences mode")
		}
	})
}
