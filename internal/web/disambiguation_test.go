package web

import (
	"testing"

	"github.com/user/vocabgen/internal/db"
)

func TestDisambiguateWords_SingleMeaning(t *testing.T) {
	words := []db.WordRow{
		{ID: 1, Word: "huis"},
		{ID: 2, Word: "werk"},
		{ID: 3, Word: "fiets"},
	}
	disambiguateWords(words)

	// No suffixes for single-meaning words.
	for _, w := range words {
		switch w.ID {
		case 1:
			if w.Word != "huis" {
				t.Errorf("word ID 1: got %q, want %q", w.Word, "huis")
			}
		case 2:
			if w.Word != "werk" {
				t.Errorf("word ID 2: got %q, want %q", w.Word, "werk")
			}
		case 3:
			if w.Word != "fiets" {
				t.Errorf("word ID 3: got %q, want %q", w.Word, "fiets")
			}
		}
	}
}

func TestDisambiguateWords_MultipleMeanings(t *testing.T) {
	words := []db.WordRow{
		{ID: 1, Word: "beslaan"},
		{ID: 2, Word: "huis"},
		{ID: 3, Word: "beslaan"},
		{ID: 4, Word: "beslaan"},
	}
	disambiguateWords(words)

	expected := map[int64]string{
		1: "beslaan (1)",
		2: "huis",
		3: "beslaan (2)",
		4: "beslaan (3)",
	}
	for _, w := range words {
		want := expected[w.ID]
		if w.Word != want {
			t.Errorf("word ID %d: got %q, want %q", w.ID, w.Word, want)
		}
	}
}

func TestDisambiguateWords_CaseInsensitive(t *testing.T) {
	words := []db.WordRow{
		{ID: 1, Word: "Beslaan"},
		{ID: 2, Word: "beslaan"},
	}
	disambiguateWords(words)

	// Both should get suffixes since they match case-insensitively.
	if words[0].Word != "Beslaan (1)" {
		t.Errorf("word ID 1: got %q, want %q", words[0].Word, "Beslaan (1)")
	}
	if words[1].Word != "beslaan (2)" {
		t.Errorf("word ID 2: got %q, want %q", words[1].Word, "beslaan (2)")
	}
}

func TestDisambiguateWords_Empty(t *testing.T) {
	var words []db.WordRow
	disambiguateWords(words) // should not panic
}

func TestDisambiguateExpressions_MultipleMeanings(t *testing.T) {
	exprs := []db.ExpressionRow{
		{ID: 1, Expression: "op de hoogte"},
		{ID: 2, Expression: "op de hoogte"},
		{ID: 3, Expression: "in de war"},
	}
	disambiguateExpressions(exprs)

	expected := map[int64]string{
		1: "op de hoogte (1)",
		2: "op de hoogte (2)",
		3: "in de war",
	}
	for _, e := range exprs {
		want := expected[e.ID]
		if e.Expression != want {
			t.Errorf("expression ID %d: got %q, want %q", e.ID, e.Expression, want)
		}
	}
}
