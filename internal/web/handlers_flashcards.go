package web

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"

	"github.com/user/vocabgen/internal/db"
)

// rateRequest is the JSON body for PUT /api/flashcards/rate.
type rateRequest struct {
	Type       string `json:"type"` // "word" or "expression"
	ID         int64  `json:"id"`
	Difficulty string `json:"difficulty"` // "easy", "hard", "natural"
}

// difficultyPresets maps difficulty filter preset names to the actual
// difficulty values used in the ListFilter query.
var difficultyPresets = map[string][]string{
	"hard_natural": {"hard", "natural"},
	"hard":         {"hard"},
	"all":          {},
	"easy":         {"easy"},
}

// handleFlashcardsHTML handles GET /api/flashcards/html — returns the
// flashcard_card partial with the filtered deck, tags, and filter state.
func (s *Server) handleFlashcardsHTML(w http.ResponseWriter, r *http.Request) {
	sourceLang := r.URL.Query().Get("source_lang")
	targetLang := r.URL.Query().Get("target_lang")
	tags := r.URL.Query().Get("tags")
	difficultyParam := r.URL.Query().Get("difficulty")

	// Default to hard_natural if not specified or unrecognized.
	difficultyValues, ok := difficultyPresets[difficultyParam]
	if !ok {
		difficultyValues = difficultyPresets["hard_natural"]
		difficultyParam = "hard_natural"
	}

	filter := db.ListFilter{
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Tags:       tags,
		Difficulty: difficultyValues,
		Page:       1,
		PageSize:   10000,
	}

	words, _, err := s.store.ListWords(r.Context(), filter)
	if err != nil {
		s.logger.Error("flashcards: list words failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	expressions, _, err := s.store.ListExpressions(r.Context(), filter)
	if err != nil {
		s.logger.Error("flashcards: list expressions failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Convert to FlashcardItem slice.
	deck := make([]db.FlashcardItem, 0, len(words)+len(expressions))
	for _, w := range words {
		deck = append(deck, db.FlashcardItem{
			ID:                w.ID,
			Type:              "word",
			Text:              w.Word,
			Definition:        w.Definition,
			English:           w.English,
			TargetTranslation: w.TargetTranslation,
			Difficulty:        w.Difficulty,
		})
	}
	for _, e := range expressions {
		deck = append(deck, db.FlashcardItem{
			ID:                e.ID,
			Type:              "expression",
			Text:              e.Expression,
			Definition:        e.Definition,
			English:           e.English,
			TargetTranslation: e.TargetTranslation,
			Difficulty:        e.Difficulty,
		})
	}

	// Fetch distinct tags for the tag dropdown — graceful degradation on error.
	allTags, err := s.store.ListDistinctTags(r.Context())
	if err != nil {
		s.logger.Error("flashcards: list distinct tags failed", "error", err)
		allTags = []string{}
	}

	// Shuffle the deck for varied study order.
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	deckJSON, _ := json.Marshal(deck)

	data := map[string]any{
		"Deck":       deck,
		"DeckJSON":   string(deckJSON),
		"DeckSize":   len(deck),
		"Tags":       allTags,
		"SourceLang": sourceLang,
		"TargetLang": targetLang,
		"ActiveTag":  tags,
		"Difficulty": difficultyParam,
	}

	_ = renderPartial(w, "flashcard_card", data)
}

// handleFlashcardsRate handles PUT /api/flashcards/rate — persists a
// difficulty rating for a word or expression entry.
func (s *Server) handleFlashcardsRate(w http.ResponseWriter, r *http.Request) {
	var req rateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate type.
	if req.Type != "word" && req.Type != "expression" {
		writeJSONError(w, http.StatusBadRequest, "invalid type: must be 'word' or 'expression'")
		return
	}

	// Validate difficulty.
	if req.Difficulty != "easy" && req.Difficulty != "hard" && req.Difficulty != "natural" {
		writeJSONError(w, http.StatusBadRequest, "invalid difficulty: must be 'easy', 'hard', or 'natural'")
		return
	}

	// Validate id.
	if req.ID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	ctx := r.Context()

	// Check existence and update.
	if req.Type == "word" {
		existing, err := s.store.GetWord(ctx, req.ID)
		if err != nil {
			s.logger.Error("flashcards: get word failed", "id", req.ID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "database error")
			return
		}
		if existing == nil {
			writeJSONError(w, http.StatusNotFound, "entry not found")
			return
		}
		if err := s.store.UpdateWordDifficulty(ctx, req.ID, req.Difficulty); err != nil {
			s.logger.Error("flashcards: update word difficulty failed", "id", req.ID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "database error")
			return
		}
	} else {
		existing, err := s.store.GetExpression(ctx, req.ID)
		if err != nil {
			s.logger.Error("flashcards: get expression failed", "id", req.ID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "database error")
			return
		}
		if existing == nil {
			writeJSONError(w, http.StatusNotFound, "entry not found")
			return
		}
		if err := s.store.UpdateExpressionDifficulty(ctx, req.ID, req.Difficulty); err != nil {
			s.logger.Error("flashcards: update expression difficulty failed", "id", req.ID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "ok",
		"difficulty": req.Difficulty,
	})
}
