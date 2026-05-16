package web

import (
	"strings"

	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/service"
)

// disambiguateWords modifies the Word field in-place to add numeric suffixes
// when multiple entries share the same headword (case-insensitive).
// Single-meaning words are left unchanged.
func disambiguateWords(words []db.WordRow) {
	if len(words) == 0 {
		return
	}

	// Count occurrences per normalized word.
	counts := make(map[string]int)
	for i := range words {
		key := strings.ToLower(words[i].Word)
		counts[key]++
	}

	// Assign indices for words with multiple meanings.
	indices := make(map[string]int)
	for i := range words {
		key := strings.ToLower(words[i].Word)
		total := counts[key]
		if total > 1 {
			indices[key]++
			words[i].Word = service.DisambiguatedWord(words[i].Word, indices[key], total)
		}
	}
}

// disambiguateExpressions modifies the Expression field in-place to add numeric
// suffixes when multiple entries share the same headword (case-insensitive).
// Single-meaning expressions are left unchanged.
func disambiguateExpressions(expressions []db.ExpressionRow) {
	if len(expressions) == 0 {
		return
	}

	counts := make(map[string]int)
	for i := range expressions {
		key := strings.ToLower(expressions[i].Expression)
		counts[key]++
	}

	indices := make(map[string]int)
	for i := range expressions {
		key := strings.ToLower(expressions[i].Expression)
		total := counts[key]
		if total > 1 {
			indices[key]++
			expressions[i].Expression = service.DisambiguatedWord(expressions[i].Expression, indices[key], total)
		}
	}
}
