package output

import (
	"fmt"

	"github.com/user/vocabgen/internal/language"
)

// Entry is the final output struct with flattened translations.
// Used for CLI JSON output, web UI display, and DB storage.
// Sentence fields are only populated for ephemeral sentence lookups.
type Entry struct {
	Word              string `json:"word,omitempty"`
	Expression        string `json:"expression,omitempty"`
	Type              string `json:"type,omitempty"`
	Article           string `json:"article,omitempty"`
	Definition        string `json:"definition,omitempty"`
	EnglishDefinition string `json:"english_definition,omitempty"`
	Example           string `json:"example,omitempty"`
	English           string `json:"english,omitempty"`
	TargetTranslation string `json:"target_translation,omitempty"`
	Notes             string `json:"notes,omitempty"`
	Connotation       string `json:"connotation,omitempty"`
	Register          string `json:"register,omitempty"`
	Collocations      string `json:"collocations,omitempty"`
	ContrastiveNotes  string `json:"contrastive_notes,omitempty"`
	SecondaryMeanings string `json:"secondary_meanings,omitempty"`
	Tags              string `json:"tags,omitempty"`

	// Sentence-specific fields (ephemeral, not stored in DB)
	Sentence          string                  `json:"sentence,omitempty"`
	CorrectedSentence string                  `json:"corrected_sentence,omitempty"`
	IsCorrect         bool                    `json:"is_correct,omitempty"`
	GrammarErrors     []language.GrammarError `json:"grammar_errors"`
	KeyVocabulary     []language.VocabItem    `json:"key_vocabulary"`
}

// FlattenTranslation converts a Translation to a display string.
// Returns "primary (alternatives)" when alternatives is non-empty, else "primary".
func FlattenTranslation(t language.Translation) string {
	if t.Alternatives != "" {
		return fmt.Sprintf("%s (%s)", t.Primary, t.Alternatives)
	}
	return t.Primary
}

// MapFields converts a ValidatedEntry to an output Entry,
// flattening translation objects to "primary (alternatives)" strings.
// Mode determines which fields are populated (words, expressions, or sentences).
// Non-translation fields (including english_definition) are passed through as-is.
func MapFields(v *language.ValidatedEntry, mode string) *Entry {
	if mode == "sentences" {
		ge := v.GrammarErrors
		if ge == nil {
			ge = []language.GrammarError{}
		}
		kv := v.KeyVocabulary
		if kv == nil {
			kv = []language.VocabItem{}
		}
		return &Entry{
			Sentence:          v.Sentence,
			CorrectedSentence: v.CorrectedSentence,
			IsCorrect:         v.IsCorrect,
			GrammarErrors:     ge,
			English:           FlattenTranslation(v.English),
			TargetTranslation: FlattenTranslation(v.TargetTranslation),
			KeyVocabulary:     kv,
			Notes:             v.Notes,
		}
	}

	e := &Entry{
		Definition:        v.Definition,
		EnglishDefinition: v.EnglishDefinition,
		Example:           v.Example,
		English:           FlattenTranslation(v.English),
		TargetTranslation: FlattenTranslation(v.TargetTranslation),
		Notes:             v.Notes,
		Connotation:       v.Connotation,
		Register:          v.Register,
		ContrastiveNotes:  v.ContrastiveNotes,
	}

	if mode == "words" {
		e.Word = v.Word
		e.Type = v.Type
		e.Article = v.Article
		e.Collocations = v.Collocations
		e.SecondaryMeanings = v.SecondaryMeanings
	} else {
		e.Expression = v.Expression
	}

	return e
}
