package service

import "fmt"

// DisambiguatedWord returns the display form of a word/expression when
// multiple meanings exist. If total is 1, the word is returned unchanged.
// If total > 1, a numeric suffix is appended: "word (1)", "word (2)", etc.
// The index parameter is 1-based.
func DisambiguatedWord(word string, index, total int) string {
	if total <= 1 {
		return word
	}
	return fmt.Sprintf("%s (%d)", word, index)
}
