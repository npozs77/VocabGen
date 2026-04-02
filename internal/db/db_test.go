package db

import (
	"context"
	"path/filepath"
	"testing"
	"unicode"

	"pgregory.net/rapid"
)

// newTestStore creates a temp SQLite store for testing. Caller should defer cleanup.
func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// makeWordRow creates a WordRow with the given word and source language.
func makeWordRow(word, sourceLang string) *WordRow {
	return &WordRow{
		Word:              word,
		PartOfSpeech:      "noun",
		Article:           "de",
		Definition:        "definitie",
		EnglishDefinition: "definition in English",
		Example:           "voorbeeld",
		English:           "example",
		TargetTranslation: "példa",
		Notes:             "notes",
		Connotation:       "neutral",
		Register:          "formeel",
		Collocations:      "a; b",
		ContrastiveNotes:  "contrast",
		SecondaryMeanings: "other",
		Tags:              "ch1",
		SourceLanguage:    sourceLang,
		TargetLanguage:    "hu",
	}
}

// makeExpressionRow creates an ExpressionRow with the given expression and source language.
func makeExpressionRow(expr, sourceLang string) *ExpressionRow {
	return &ExpressionRow{
		Expression:        expr,
		Definition:        "definitie",
		EnglishDefinition: "definition in English",
		Example:           "voorbeeld",
		English:           "example",
		TargetTranslation: "példa",
		Notes:             "notes",
		Connotation:       "neutral",
		Register:          "formeel",
		ContrastiveNotes:  "contrast",
		Tags:              "ch1",
		SourceLanguage:    sourceLang,
		TargetLanguage:    "hu",
	}
}

// --- Property Test P12: UTF-8 round-trip consistency ---

func TestPropertyP12_UTF8RoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rapid.Check(t, func(t *rapid.T) {
		// Generate random Unicode strings including accented, Cyrillic, and CJK characters.
		word := rapid.StringOfN(rapid.RuneFrom(nil,
			unicode.Latin, unicode.Cyrillic, unicode.Han, unicode.Hangul,
		), 1, 50, -1).Draw(t, "word")

		row := makeWordRow(word, "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("InsertWord: %v", err)
		}

		got, err := store.FindWord(ctx, word, "nl")
		if err != nil {
			t.Fatalf("FindWord: %v", err)
		}
		if got == nil {
			t.Fatal("FindWord returned nil for inserted word")
			return // unreachable, but satisfies staticcheck SA5011
		}
		if got.Word != word {
			t.Fatalf("UTF-8 mismatch: inserted %q, got %q", word, got.Word)
		}
	})
}

// --- Table-driven tests ---

func TestMigration_TablesAndIndexesExist(t *testing.T) {
	store := newTestStore(t)

	tables := []string{"words", "expressions", "metadata"}
	for _, name := range tables {
		exists, err := tableExists(store.db, name)
		if err != nil {
			t.Fatalf("tableExists(%s): %v", name, err)
		}
		if !exists {
			t.Errorf("table %q should exist after migration", name)
		}
	}

	// Check indexes.
	indexes := []struct {
		name  string
		table string
	}{
		{"idx_words_source_word", "words"},
		{"idx_expressions_source_expr", "expressions"},
	}
	for _, idx := range indexes {
		var count int
		err := store.db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=? AND tbl_name=?`,
			idx.name, idx.table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("check index %s: %v", idx.name, err)
		}
		if count == 0 {
			t.Errorf("index %q on table %q should exist", idx.name, idx.table)
		}
	}
}

func TestInsertWord_FindWord_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row := makeWordRow("huis", "nl")
	if err := store.InsertWord(ctx, row); err != nil {
		t.Fatalf("InsertWord: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("InsertWord should set ID")
	}
	if row.CreatedAt == "" || row.UpdatedAt == "" {
		t.Fatal("InsertWord should set timestamps")
	}

	got, err := store.FindWord(ctx, "huis", "nl")
	if err != nil {
		t.Fatalf("FindWord: %v", err)
	}
	if got == nil {
		t.Fatal("FindWord returned nil")
	}
	if got.Word != "huis" || got.SourceLanguage != "nl" {
		t.Errorf("got word=%q lang=%q, want huis/nl", got.Word, got.SourceLanguage)
	}
	if got.EnglishDefinition != "definition in English" {
		t.Errorf("EnglishDefinition = %q, want %q", got.EnglishDefinition, "definition in English")
	}
}

func TestInsertExpression_FindExpression_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row := makeExpressionRow("op de hoogte", "nl")
	if err := store.InsertExpression(ctx, row); err != nil {
		t.Fatalf("InsertExpression: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("InsertExpression should set ID")
	}

	got, err := store.FindExpression(ctx, "op de hoogte", "nl")
	if err != nil {
		t.Fatalf("FindExpression: %v", err)
	}
	if got == nil {
		t.Fatal("FindExpression returned nil")
	}
	if got.Expression != "op de hoogte" {
		t.Errorf("got expression=%q, want %q", got.Expression, "op de hoogte")
	}
	if got.EnglishDefinition != "definition in English" {
		t.Errorf("EnglishDefinition = %q, want %q", got.EnglishDefinition, "definition in English")
	}
}

func TestFindWord_NonExistent_ReturnsNil(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	got, err := store.FindWord(ctx, "nonexistent", "nl")
	if err != nil {
		t.Fatalf("FindWord: %v", err)
	}
	if got != nil {
		t.Error("FindWord should return nil for non-existent entry")
	}
}

func TestFindWords_NonExistent_ReturnsEmptySlice(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	got, err := store.FindWords(ctx, "nonexistent", "nl")
	if err != nil {
		t.Fatalf("FindWords: %v", err)
	}
	if got == nil {
		t.Fatal("FindWords should return empty slice, not nil")
	}
	if len(got) != 0 {
		t.Errorf("FindWords should return 0 entries, got %d", len(got))
	}
}

func TestFindWords_MultiVersion(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert two entries for the same word (different POS).
	row1 := makeWordRow("werk", "nl")
	row1.PartOfSpeech = "noun"
	row2 := makeWordRow("werk", "nl")
	row2.PartOfSpeech = "verb"

	if err := store.InsertWord(ctx, row1); err != nil {
		t.Fatalf("InsertWord 1: %v", err)
	}
	if err := store.InsertWord(ctx, row2); err != nil {
		t.Fatalf("InsertWord 2: %v", err)
	}

	got, err := store.FindWords(ctx, "werk", "nl")
	if err != nil {
		t.Fatalf("FindWords: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindWords should return 2 entries, got %d", len(got))
	}

	// Verify both POS values are present.
	posSet := map[string]bool{}
	for _, w := range got {
		posSet[w.PartOfSpeech] = true
	}
	if !posSet["noun"] || !posSet["verb"] {
		t.Errorf("expected noun and verb POS, got %v", posSet)
	}
}

func TestFindExpressions_MultiVersion(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row1 := makeExpressionRow("in de war", "nl")
	row1.Notes = "version 1"
	row2 := makeExpressionRow("in de war", "nl")
	row2.Notes = "version 2"

	if err := store.InsertExpression(ctx, row1); err != nil {
		t.Fatalf("InsertExpression 1: %v", err)
	}
	if err := store.InsertExpression(ctx, row2); err != nil {
		t.Fatalf("InsertExpression 2: %v", err)
	}

	got, err := store.FindExpressions(ctx, "in de war", "nl")
	if err != nil {
		t.Fatalf("FindExpressions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindExpressions should return 2 entries, got %d", len(got))
	}
}

func TestUpdateWord_SetsUpdatedAt(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row1 := makeWordRow("werk", "nl")
	row1.PartOfSpeech = "noun"
	row2 := makeWordRow("werk", "nl")
	row2.PartOfSpeech = "verb"

	if err := store.InsertWord(ctx, row1); err != nil {
		t.Fatalf("InsertWord 1: %v", err)
	}
	if err := store.InsertWord(ctx, row2); err != nil {
		t.Fatalf("InsertWord 2: %v", err)
	}

	// Force a known past timestamp on row1 so we can detect the update.
	pastTime := "2024-01-01T00:00:00Z"
	if _, err := store.db.ExecContext(ctx,
		`UPDATE words SET updated_at = ? WHERE id = ?`, pastTime, row1.ID,
	); err != nil {
		t.Fatalf("force past timestamp: %v", err)
	}

	// Update only the first entry.
	updated := makeWordRow("werk", "nl")
	updated.PartOfSpeech = "noun"
	updated.Definition = "updated definition"
	if err := store.UpdateWord(ctx, row1.ID, updated); err != nil {
		t.Fatalf("UpdateWord: %v", err)
	}

	// Verify the updated entry.
	all, err := store.FindWords(ctx, "werk", "nl")
	if err != nil {
		t.Fatalf("FindWords: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	var found bool
	for _, w := range all {
		if w.ID == row1.ID {
			found = true
			if w.Definition != "updated definition" {
				t.Errorf("definition not updated: got %q", w.Definition)
			}
			if w.UpdatedAt == pastTime {
				t.Error("updated_at should have changed from the past timestamp")
			}
		}
		if w.ID == row2.ID {
			if w.PartOfSpeech != "verb" {
				t.Errorf("other entry POS changed: got %q", w.PartOfSpeech)
			}
		}
	}
	if !found {
		t.Error("updated entry not found")
	}
}

func TestDeleteWord_RemovesByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row1 := makeWordRow("werk", "nl")
	row1.PartOfSpeech = "noun"
	row2 := makeWordRow("werk", "nl")
	row2.PartOfSpeech = "verb"

	if err := store.InsertWord(ctx, row1); err != nil {
		t.Fatalf("InsertWord 1: %v", err)
	}
	if err := store.InsertWord(ctx, row2); err != nil {
		t.Fatalf("InsertWord 2: %v", err)
	}

	// Delete the first entry.
	if err := store.DeleteWord(ctx, row1.ID); err != nil {
		t.Fatalf("DeleteWord: %v", err)
	}

	all, err := store.FindWords(ctx, "werk", "nl")
	if err != nil {
		t.Fatalf("FindWords: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", len(all))
	}
	if all[0].ID != row2.ID {
		t.Errorf("wrong entry survived: got ID %d, want %d", all[0].ID, row2.ID)
	}
}

func TestListWords_Pagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert 5 words.
	for i := 0; i < 5; i++ {
		row := makeWordRow("word"+string(rune('a'+i)), "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("InsertWord %d: %v", i, err)
		}
	}

	// Page 1, size 2.
	results, total, err := store.ListWords(ctx, ListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListWords: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(results) != 2 {
		t.Errorf("page size = %d, want 2", len(results))
	}

	// Page 3, size 2 — should get 1 result.
	results, total, err = store.ListWords(ctx, ListFilter{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("ListWords page 3: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(results) != 1 {
		t.Errorf("page 3 size = %d, want 1", len(results))
	}
}

func TestImportWords_SkipsDuplicates(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert one word first.
	existing := makeWordRow("huis", "nl")
	if err := store.InsertWord(ctx, existing); err != nil {
		t.Fatalf("InsertWord: %v", err)
	}

	// Import batch including the duplicate.
	batch := []WordRow{
		*makeWordRow("huis", "nl"),  // duplicate
		*makeWordRow("fiets", "nl"), // new
		*makeWordRow("boom", "nl"),  // new
	}

	imported, skipped, failed, err := store.ImportWords(ctx, batch)
	if err != nil {
		t.Fatalf("ImportWords: %v", err)
	}
	if imported != 2 {
		t.Errorf("imported = %d, want 2", imported)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
}

func TestGetWord_ByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row := &WordRow{
		Word: "huis", PartOfSpeech: "znw", Article: "het",
		Definition: "gebouw", Example: "Het huis is groot.",
		English: "house", TargetTranslation: "ház",
		SourceLanguage: "nl", TargetLanguage: "hu",
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
	}
	if err := store.InsertWord(ctx, row); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Find the ID
	found, err := store.FindWord(ctx, "huis", "nl")
	if err != nil || found == nil {
		t.Fatalf("FindWord: %v", err)
	}

	// GetWord by ID
	got, err := store.GetWord(ctx, found.ID)
	if err != nil {
		t.Fatalf("GetWord: %v", err)
	}
	if got == nil {
		t.Fatal("GetWord returned nil")
	}
	if got.Word != "huis" || got.ID != found.ID {
		t.Errorf("GetWord returned wrong entry: got %q (ID %d), want %q (ID %d)", got.Word, got.ID, "huis", found.ID)
	}

	// GetWord with non-existent ID
	missing, err := store.GetWord(ctx, 99999)
	if err != nil {
		t.Fatalf("GetWord non-existent: %v", err)
	}
	if missing != nil {
		t.Error("GetWord should return nil for non-existent ID")
	}
}

func TestGetExpression_ByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	row := &ExpressionRow{
		Expression: "op de hoogte", Definition: "informed",
		Example: "Ik ben op de hoogte.", English: "up to date",
		TargetTranslation: "naprakész",
		SourceLanguage:    "nl", TargetLanguage: "hu",
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
	}
	if err := store.InsertExpression(ctx, row); err != nil {
		t.Fatalf("insert: %v", err)
	}

	found, err := store.FindExpression(ctx, "op de hoogte", "nl")
	if err != nil || found == nil {
		t.Fatalf("FindExpression: %v", err)
	}

	got, err := store.GetExpression(ctx, found.ID)
	if err != nil {
		t.Fatalf("GetExpression: %v", err)
	}
	if got == nil {
		t.Fatal("GetExpression returned nil")
	}
	if got.Expression != "op de hoogte" {
		t.Errorf("got %q, want %q", got.Expression, "op de hoogte")
	}
}
