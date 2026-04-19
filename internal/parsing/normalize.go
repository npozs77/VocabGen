package parsing

import (
	"regexp"
	"strings"
)

// quoteChars are the quote characters to strip from tokens.
var quoteChars = strings.NewReplacer(
	`"`, "",
	`'`, "",
	"\u2018", "", // '
	"\u2019", "", // '
	"\u201C", "", // "
	"\u201D", "", // "
	"\u00AB", "", // «
	"\u00BB", "", // »
)

// multiSpace matches two or more consecutive whitespace characters.
var multiSpace = regexp.MustCompile(`\s{2,}`)

// sepMarker matches the "(sep.)" separable-verb annotation (with optional surrounding whitespace).
var sepMarker = regexp.MustCompile(`\s*\(sep\.?\)\s*`)

// conjugationInfo matches parenthetical groups containing commas — these are
// conjugation annotations like "(kwam uit, is uitgekomen)" not part of the word.
var conjugationInfo = regexp.MustCompile(`\s*\([^)]*,[^)]*\)\s*`)

// leadingArrow matches a leading ">" possibly followed by whitespace/tab (related-form indicator).
var leadingArrow = regexp.MustCompile(`^>\s*`)

// leadingArticle matches a leading Dutch article (de/het/een) followed by whitespace.
// Only used for word normalization — the article is stored in a separate DB field.
var leadingArticle = regexp.MustCompile(`(?i)^(de|het|een)\s+`)

// stripMarkers removes vocabulary-list annotations (* prefix/suffix, > prefix,
// (sep.) suffix, conjugation parentheticals) that are not part of the actual
// word or expression. Matches Python prototype's normalize_for_skip_check.
func stripMarkers(s string) string {
	// Strip leading ">" with optional whitespace/tab
	s = leadingArrow.ReplaceAllString(s, "")
	// Strip leading/trailing asterisks (frequency markers)
	s = strings.TrimLeft(s, "* ")
	s = strings.TrimRight(s, "* ")
	// Strip "(sep.)" annotations
	s = sepMarker.ReplaceAllString(s, " ")
	// Strip conjugation info like (kwam uit, is uitgekomen)
	s = conjugationInfo.ReplaceAllString(s, " ")
	return s
}

// NormalizeWord strips quotes, vocabulary-list markers, leading Dutch articles
// (de/het/een), collapses whitespace, and preserves simple parenthetical info.
// Returns empty string for whitespace-only input.
func NormalizeWord(raw string) string {
	s := quoteChars.Replace(raw)
	s = stripMarkers(s)
	s = leadingArticle.ReplaceAllString(s, "")
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}

// NormalizeExpression strips quotes, vocabulary-list markers, and collapses whitespace.
// Returns empty string for whitespace-only input.
func NormalizeExpression(raw string) string {
	s := quoteChars.Replace(raw)
	s = stripMarkers(s)
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}
