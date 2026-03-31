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

// NormalizeWord strips quotes, collapses whitespace, preserves parenthetical
// inflection info. Returns empty string for whitespace-only input.
func NormalizeWord(raw string) string {
	s := quoteChars.Replace(raw)
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}

// NormalizeExpression strips quotes and collapses whitespace.
// Returns empty string for whitespace-only input.
func NormalizeExpression(raw string) string {
	s := quoteChars.Replace(raw)
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}
