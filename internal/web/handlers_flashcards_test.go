package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/config"
	"github.com/user/vocabgen/internal/db"
)

// flashcardMockStore embeds stubStore and overrides methods needed for
// flashcard handler tests with in-memory data and call tracking.
type flashcardMockStore struct {
	stubStore

	words       []db.WordRow
	expressions []db.ExpressionRow
	tags        []string

	// Track the last ListFilter passed to ListWords/ListExpressions.
	lastWordFilter       db.ListFilter
	lastExpressionFilter db.ListFilter

	// Track difficulty updates.
	updatedWordDifficulties       map[int64]string
	updatedExpressionDifficulties map[int64]string
}

func newFlashcardMockStore() *flashcardMockStore {
	return &flashcardMockStore{
		updatedWordDifficulties:       make(map[int64]string),
		updatedExpressionDifficulties: make(map[int64]string),
	}
}

func (m *flashcardMockStore) ListWords(_ context.Context, filter db.ListFilter) ([]db.WordRow, int, error) {
	m.lastWordFilter = filter
	var result []db.WordRow
	for _, w := range m.words {
		if !matchesFilter(w.SourceLanguage, w.TargetLanguage, w.Tags, w.Difficulty, filter) {
			continue
		}
		result = append(result, w)
	}
	return result, len(result), nil
}

func (m *flashcardMockStore) ListExpressions(_ context.Context, filter db.ListFilter) ([]db.ExpressionRow, int, error) {
	m.lastExpressionFilter = filter
	var result []db.ExpressionRow
	for _, e := range m.expressions {
		if !matchesFilter(e.SourceLanguage, e.TargetLanguage, e.Tags, e.Difficulty, filter) {
			continue
		}
		result = append(result, e)
	}
	return result, len(result), nil
}

func (m *flashcardMockStore) ListDistinctTags(_ context.Context) ([]string, error) {
	return m.tags, nil
}

func (m *flashcardMockStore) GetWord(_ context.Context, id int64) (*db.WordRow, error) {
	for _, w := range m.words {
		if w.ID == id {
			// Return current difficulty (may have been updated).
			if d, ok := m.updatedWordDifficulties[id]; ok {
				w.Difficulty = d
			}
			return &w, nil
		}
	}
	return nil, nil
}

func (m *flashcardMockStore) GetExpression(_ context.Context, id int64) (*db.ExpressionRow, error) {
	for _, e := range m.expressions {
		if e.ID == id {
			if d, ok := m.updatedExpressionDifficulties[id]; ok {
				e.Difficulty = d
			}
			return &e, nil
		}
	}
	return nil, nil
}

func (m *flashcardMockStore) UpdateWordDifficulty(_ context.Context, id int64, difficulty string) error {
	m.updatedWordDifficulties[id] = difficulty
	return nil
}

func (m *flashcardMockStore) UpdateExpressionDifficulty(_ context.Context, id int64, difficulty string) error {
	m.updatedExpressionDifficulties[id] = difficulty
	return nil
}

// matchesFilter checks whether a row matches the given ListFilter criteria.
func matchesFilter(sourceLang, targetLang, tags, difficulty string, f db.ListFilter) bool {
	if f.SourceLang != "" && sourceLang != f.SourceLang {
		return false
	}
	if f.TargetLang != "" && targetLang != f.TargetLang {
		return false
	}
	if f.Tags != "" && !strings.Contains(tags, f.Tags) {
		return false
	}
	if len(f.Difficulty) > 0 {
		found := false
		for _, d := range f.Difficulty {
			if d == difficulty {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func newFlashcardTestServer(store *flashcardMockStore) *Server {
	cfg := config.DefaultConfig()
	return NewServer(store, &cfg, slog.Default(), "test", "unknown", "go1.22")
}

// TestFlashcardsPage_Returns200 verifies GET /flashcards returns 200 with HTML content type.
//
// Validates: Requirements 1.1
func TestFlashcardsPage_Returns200(t *testing.T) {
	store := newFlashcardMockStore()
	srv := newFlashcardTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/flashcards", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html; charset=utf-8, got %q", ct)
	}
}

// TestFlashcardsHTML_ReturnsPartial verifies GET /api/flashcards/html returns
// an HTML partial containing the data-deck attribute.
//
// Validates: Requirements 8.1
func TestFlashcardsHTML_ReturnsPartial(t *testing.T) {
	store := newFlashcardMockStore()
	store.words = []db.WordRow{
		{
			ID:                1,
			Word:              "huis",
			Definition:        "een gebouw",
			English:           "house",
			TargetTranslation: "ház",
			Difficulty:        "natural",
			SourceLanguage:    "nl",
			TargetLanguage:    "hu",
		},
	}
	store.tags = []string{"chapter-1"}
	srv := newFlashcardTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/flashcards/html", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html; charset=utf-8, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "data-deck") {
		t.Fatal("expected response to contain data-deck attribute")
	}
}

// TestFlashcardsHTML_EmptyDeck verifies the empty-state message when no entries match.
//
// Validates: Requirements 2.3
func TestFlashcardsHTML_EmptyDeck(t *testing.T) {
	store := newFlashcardMockStore()
	// No words or expressions — empty deck.
	srv := newFlashcardTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/flashcards/html", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No flashcards match your filters") {
		t.Fatal("expected empty-state message in response")
	}
}

// TestFlashcardsRate_ValidRequest is a table-driven test for valid rate requests
// covering word/expression × easy/hard/natural combinations.
//
// Validates: Requirements 10.2
func TestFlashcardsRate_ValidRequest(t *testing.T) {
	tests := []struct {
		name       string
		entryType  string
		id         int64
		difficulty string
	}{
		{"word easy", "word", 1, "easy"},
		{"word hard", "word", 1, "hard"},
		{"word natural", "word", 1, "natural"},
		{"expression easy", "expression", 10, "easy"},
		{"expression hard", "expression", 10, "hard"},
		{"expression natural", "expression", 10, "natural"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFlashcardMockStore()
			store.words = []db.WordRow{
				{ID: 1, Word: "huis", Difficulty: "natural", SourceLanguage: "nl", TargetLanguage: "hu"},
			}
			store.expressions = []db.ExpressionRow{
				{ID: 10, Expression: "op de hoogte", Difficulty: "natural", SourceLanguage: "nl", TargetLanguage: "hu"},
			}
			srv := newFlashcardTestServer(store)

			body := `{"type":"` + tc.entryType + `","id":` + idToString(tc.id) + `,"difficulty":"` + tc.difficulty + `"}`
			req := httptest.NewRequest(http.MethodPut, "/api/flashcards/rate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
			}

			var resp map[string]string
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode json: %v", err)
			}
			if resp["status"] != "ok" {
				t.Fatalf("expected status ok, got %q", resp["status"])
			}
			if resp["difficulty"] != tc.difficulty {
				t.Fatalf("expected difficulty %q, got %q", tc.difficulty, resp["difficulty"])
			}

			// Verify the mock recorded the update.
			if tc.entryType == "word" {
				if d, ok := store.updatedWordDifficulties[tc.id]; !ok || d != tc.difficulty {
					t.Fatalf("expected word %d difficulty %q, got %q (ok=%v)", tc.id, tc.difficulty, d, ok)
				}
			} else {
				if d, ok := store.updatedExpressionDifficulties[tc.id]; !ok || d != tc.difficulty {
					t.Fatalf("expected expression %d difficulty %q, got %q (ok=%v)", tc.id, tc.difficulty, d, ok)
				}
			}
		})
	}
}

// idToString converts an int64 to its string representation for JSON building.
func idToString(id int64) string {
	return fmt.Sprintf("%d", id)
}

// TestFlashcardsRate_InvalidType is a table-driven test for invalid type values returning 400.
//
// Validates: Requirements 10.3
func TestFlashcardsRate_InvalidType(t *testing.T) {
	tests := []struct {
		name      string
		entryType string
	}{
		{"empty type", ""},
		{"unknown type", "phrase"},
		{"numeric type", "123"},
		{"capitalized", "Word"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFlashcardMockStore()
			srv := newFlashcardTestServer(store)

			body := `{"type":"` + tc.entryType + `","id":1,"difficulty":"easy"}`
			req := httptest.NewRequest(http.MethodPut, "/api/flashcards/rate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
			}

			var resp map[string]string
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode json: %v", err)
			}
			if !strings.Contains(resp["detail"], "invalid type") {
				t.Fatalf("expected error about invalid type, got %q", resp["detail"])
			}
		})
	}
}

// TestFlashcardsRate_InvalidDifficulty is a table-driven test for invalid difficulty values returning 400.
//
// Validates: Requirements 10.3
func TestFlashcardsRate_InvalidDifficulty(t *testing.T) {
	tests := []struct {
		name       string
		difficulty string
	}{
		{"empty difficulty", ""},
		{"unknown difficulty", "medium"},
		{"numeric difficulty", "5"},
		{"capitalized", "Easy"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFlashcardMockStore()
			srv := newFlashcardTestServer(store)

			body := `{"type":"word","id":1,"difficulty":"` + tc.difficulty + `"}`
			req := httptest.NewRequest(http.MethodPut, "/api/flashcards/rate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
			}

			var resp map[string]string
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode json: %v", err)
			}
			if !strings.Contains(resp["detail"], "invalid difficulty") {
				t.Fatalf("expected error about invalid difficulty, got %q", resp["detail"])
			}
		})
	}
}

// TestFlashcardsRate_DefaultDifficultyFilter verifies that when no difficulty
// query param is provided, the handler uses the hard_natural preset which
// excludes entries with difficulty "easy".
//
// Validates: Requirements 11.2
func TestFlashcardsRate_DefaultDifficultyFilter(t *testing.T) {
	store := newFlashcardMockStore()
	store.words = []db.WordRow{
		{ID: 1, Word: "huis", Difficulty: "natural", SourceLanguage: "nl", TargetLanguage: "hu"},
		{ID: 2, Word: "boek", Difficulty: "hard", SourceLanguage: "nl", TargetLanguage: "hu"},
		{ID: 3, Word: "kat", Difficulty: "easy", SourceLanguage: "nl", TargetLanguage: "hu"},
	}
	store.expressions = []db.ExpressionRow{
		{ID: 10, Expression: "op de hoogte", Difficulty: "easy", SourceLanguage: "nl", TargetLanguage: "hu"},
	}
	srv := newFlashcardTestServer(store)

	// No difficulty param — should default to hard_natural.
	req := httptest.NewRequest(http.MethodGet, "/api/flashcards/html", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// The response should contain "huis" (natural) and "boek" (hard) but NOT "kat" (easy)
	// and NOT "op de hoogte" (easy). We check the data-deck JSON.
	deckStart := strings.Index(body, `data-deck="`)
	if deckStart == -1 {
		t.Fatal("expected data-deck attribute in response")
	}
	deckStart += len(`data-deck="`)
	deckEnd := strings.Index(body[deckStart:], `"`)
	if deckEnd == -1 {
		t.Fatal("expected closing quote for data-deck attribute")
	}
	deckJSON := body[deckStart : deckStart+deckEnd]
	// HTML-decode the JSON (template escapes < > & " etc.)
	deckJSON = strings.ReplaceAll(deckJSON, "&amp;", "&")
	deckJSON = strings.ReplaceAll(deckJSON, "&lt;", "<")
	deckJSON = strings.ReplaceAll(deckJSON, "&gt;", ">")
	deckJSON = strings.ReplaceAll(deckJSON, "&#34;", `"`)
	deckJSON = strings.ReplaceAll(deckJSON, "&quot;", `"`)

	var deck []db.FlashcardItem
	if err := json.Unmarshal([]byte(deckJSON), &deck); err != nil {
		t.Fatalf("decode deck JSON: %v; raw: %s", err, deckJSON)
	}

	// Should have 2 items: huis (natural) and boek (hard).
	if len(deck) != 2 {
		t.Fatalf("expected 2 items in deck (hard+natural), got %d: %+v", len(deck), deck)
	}

	for _, item := range deck {
		if item.Difficulty == "easy" {
			t.Fatalf("deck should not contain easy items, but found: %+v", item)
		}
	}
}
