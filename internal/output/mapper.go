package output

import (
	"fmt"

	"github.com/user/vocabgen/internal/language"
)

// Entry is the final output struct with flattened translations.
// Used for CLI JSON output, web UI display, and DB storage.
type Entry struct {
	Word              string `json:"word,omitempty"`
	Expression        string `json:"expression,omitempty"`
	Type              string `json:"type,omitempty"`
	Article           string `json:"article,omitempty"`
	Definition        string `json:"definition"`
	EnglishDefinition string `json:"english_definition,omitempty"`
	Example           string `json:"example"`
	English           string `json:"english"`
	TargetTranslation string `json:"target_translation"`
	Notes             string `json:"notes"`
	Connotation       string `json:"connotation"`
	Register          string `json:"register"`
	Collocations      string `json:"collocations,omitempty"`
	ContrastiveNotes  string `json:"contrastive_notes"`
	SecondaryMeanings string `json:"secondary_meanings,omitempty"`
	Tags              string `json:"tags,omitempty"`
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
// Mode determines which fields are populated (words vs expressions).
// Non-translation fields (including english_definition) are passed through as-is.
func MapFields(v *language.ValidatedEntry, mode string) *Entry {
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
