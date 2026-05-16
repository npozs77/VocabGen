package service

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

func TestDisambiguatedWord_SingleMeaning(t *testing.T) {
	tests := []struct {
		name  string
		word  string
		index int
		total int
		want  string
	}{
		{"single meaning returns word unchanged", "beslaan", 1, 1, "beslaan"},
		{"zero total returns word unchanged", "lopen", 1, 0, "lopen"},
		{"negative total returns word unchanged", "huis", 1, -1, "huis"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DisambiguatedWord(tt.word, tt.index, tt.total)
			if got != tt.want {
				t.Errorf("DisambiguatedWord(%q, %d, %d) = %q, want %q", tt.word, tt.index, tt.total, got, tt.want)
			}
		})
	}
}

func TestDisambiguatedWord_MultipleMeanings(t *testing.T) {
	tests := []struct {
		name  string
		word  string
		index int
		total int
		want  string
	}{
		{"two meanings, first", "beslaan", 1, 2, "beslaan (1)"},
		{"two meanings, second", "beslaan", 2, 2, "beslaan (2)"},
		{"three meanings, third", "run", 3, 3, "run (3)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DisambiguatedWord(tt.word, tt.index, tt.total)
			if got != tt.want {
				t.Errorf("DisambiguatedWord(%q, %d, %d) = %q, want %q", tt.word, tt.index, tt.total, got, tt.want)
			}
		})
	}
}

// TestPropertyP18_DisambiguationSuffix verifies that for any word with N meanings (N > 1),
// all N entries display correct disambiguation suffixes, and single-meaning words have no suffix.
func TestPropertyP18_DisambiguationSuffix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		word := rapid.StringMatching(`[a-z]{2,20}`).Draw(t, "word")
		total := rapid.IntRange(1, 50).Draw(t, "total")

		if total == 1 {
			got := DisambiguatedWord(word, 1, total)
			if got != word {
				t.Fatalf("single meaning: DisambiguatedWord(%q, 1, 1) = %q, want %q", word, got, word)
			}
		} else {
			for i := 1; i <= total; i++ {
				got := DisambiguatedWord(word, i, total)
				expected := fmt.Sprintf("%s (%d)", word, i)
				if got != expected {
					t.Fatalf("DisambiguatedWord(%q, %d, %d) = %q, want %q", word, i, total, got, expected)
				}
			}
		}
	})
}
