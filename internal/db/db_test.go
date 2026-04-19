package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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

func TestDeleteWords_BulkDelete(t *testing.T) {
	tests := []struct {
		name      string
		insertN   int
		deleteIDs func(ids []int64) []int64
		wantAfter int
		wantErr   bool
	}{
		{
			name:    "delete all",
			insertN: 3,
			deleteIDs: func(ids []int64) []int64 {
				return ids
			},
			wantAfter: 0,
		},
		{
			name:    "delete subset",
			insertN: 3,
			deleteIDs: func(ids []int64) []int64 {
				return ids[:2]
			},
			wantAfter: 1,
		},
		{
			name:    "empty slice is no-op",
			insertN: 2,
			deleteIDs: func(_ []int64) []int64 {
				return []int64{}
			},
			wantAfter: 2,
		},
		{
			name:    "non-existent IDs are ignored",
			insertN: 2,
			deleteIDs: func(_ []int64) []int64 {
				return []int64{99998, 99999}
			},
			wantAfter: 2,
		},
		{
			name:    "mixed valid and non-existent",
			insertN: 3,
			deleteIDs: func(ids []int64) []int64 {
				return []int64{ids[0], 99999}
			},
			wantAfter: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := context.Background()

			var ids []int64
			for i := 0; i < tc.insertN; i++ {
				row := makeWordRow("bulk"+string(rune('A'+i)), "nl")
				if err := store.InsertWord(ctx, row); err != nil {
					t.Fatalf("insert %d: %v", i, err)
				}
				ids = append(ids, row.ID)
			}

			err := store.DeleteWords(ctx, tc.deleteIDs(ids))
			if (err != nil) != tc.wantErr {
				t.Fatalf("DeleteWords error = %v, wantErr %v", err, tc.wantErr)
			}

			all, _, err := store.ListWords(ctx, ListFilter{SourceLang: "nl", Page: 1, PageSize: 100})
			if err != nil {
				t.Fatalf("ListWords: %v", err)
			}
			if len(all) != tc.wantAfter {
				t.Errorf("got %d entries, want %d", len(all), tc.wantAfter)
			}
		})
	}
}

func TestDeleteExpressions_BulkDelete(t *testing.T) {
	tests := []struct {
		name      string
		insertN   int
		deleteIDs func(ids []int64) []int64
		wantAfter int
	}{
		{
			name:    "delete all",
			insertN: 3,
			deleteIDs: func(ids []int64) []int64 {
				return ids
			},
			wantAfter: 0,
		},
		{
			name:    "delete subset",
			insertN: 3,
			deleteIDs: func(ids []int64) []int64 {
				return ids[:1]
			},
			wantAfter: 2,
		},
		{
			name:    "empty slice is no-op",
			insertN: 2,
			deleteIDs: func(_ []int64) []int64 {
				return []int64{}
			},
			wantAfter: 2,
		},
		{
			name:    "non-existent IDs are ignored",
			insertN: 1,
			deleteIDs: func(_ []int64) []int64 {
				return []int64{99999}
			},
			wantAfter: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := context.Background()

			var ids []int64
			for i := 0; i < tc.insertN; i++ {
				row := makeExpressionRow("bulk expr "+string(rune('A'+i)), "nl")
				if err := store.InsertExpression(ctx, row); err != nil {
					t.Fatalf("insert %d: %v", i, err)
				}
				ids = append(ids, row.ID)
			}

			if err := store.DeleteExpressions(ctx, tc.deleteIDs(ids)); err != nil {
				t.Fatalf("DeleteExpressions: %v", err)
			}

			all, _, err := store.ListExpressions(ctx, ListFilter{SourceLang: "nl", Page: 1, PageSize: 100})
			if err != nil {
				t.Fatalf("ListExpressions: %v", err)
			}
			if len(all) != tc.wantAfter {
				t.Errorf("got %d entries, want %d", len(all), tc.wantAfter)
			}
		})
	}
}

// --- Migration V2 tests ---

func TestMigrationV2_AddsDifficultyColumn(t *testing.T) {
	// Requirements: 9.1, 9.3
	// Verifies the V2 migration adds the difficulty column to both tables,
	// sets schema version to '2', and defaults fresh inserts to 'natural'.

	tests := []struct {
		name  string
		table string
	}{
		{"words table has difficulty column", "words"},
		{"expressions table has difficulty column", "expressions"},
	}

	store := newTestStore(t)
	ctx := context.Background()

	// Sub-test: verify difficulty column exists on both tables.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// PRAGMA table_info returns columns; check that 'difficulty' is present.
			rows, err := store.db.QueryContext(ctx,
				fmt.Sprintf("PRAGMA table_info(%s)", tc.table))
			if err != nil {
				t.Fatalf("PRAGMA table_info(%s): %v", tc.table, err)
			}
			defer func() { _ = rows.Close() }()

			found := false
			for rows.Next() {
				var cid int
				var name, colType string
				var notNull int
				var dfltValue sql.NullString
				var pk int
				if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
					t.Fatalf("scan column info: %v", err)
				}
				if name == "difficulty" {
					found = true
					if !dfltValue.Valid || dfltValue.String != "'natural'" {
						t.Errorf("difficulty default = %v, want 'natural'", dfltValue)
					}
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows iteration: %v", err)
			}
			if !found {
				t.Errorf("table %q missing difficulty column", tc.table)
			}
		})
	}

	// Sub-test: verify schema version is '2'.
	t.Run("schema version is 2", func(t *testing.T) {
		var version string
		err := store.db.QueryRowContext(ctx,
			`SELECT value FROM metadata WHERE key = 'schema_version'`,
		).Scan(&version)
		if err != nil {
			t.Fatalf("query schema_version: %v", err)
		}
		if version != "2" {
			t.Errorf("schema_version = %q, want %q", version, "2")
		}
	})

	// Sub-test: verify fresh inserts default to 'natural'.
	t.Run("word insert defaults difficulty to natural", func(t *testing.T) {
		row := makeWordRow("testwoord", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("InsertWord: %v", err)
		}
		got, err := store.FindWord(ctx, "testwoord", "nl")
		if err != nil {
			t.Fatalf("FindWord: %v", err)
		}
		if got == nil {
			t.Fatal("FindWord returned nil")
		}
		if got.Difficulty != "natural" {
			t.Errorf("word difficulty = %q, want %q", got.Difficulty, "natural")
		}
	})

	t.Run("expression insert defaults difficulty to natural", func(t *testing.T) {
		row := makeExpressionRow("test uitdrukking", "nl")
		if err := store.InsertExpression(ctx, row); err != nil {
			t.Fatalf("InsertExpression: %v", err)
		}
		got, err := store.FindExpression(ctx, "test uitdrukking", "nl")
		if err != nil {
			t.Fatalf("FindExpression: %v", err)
		}
		if got == nil {
			t.Fatal("FindExpression returned nil")
		}
		if got.Difficulty != "natural" {
			t.Errorf("expression difficulty = %q, want %q", got.Difficulty, "natural")
		}
	})
}

// --- Property Test P13: Migration preserves data and defaults difficulty ---

// Feature: flashcards, Property 4: Migration preserves data and defaults difficulty
// **Validates: Requirements 9.2**
func TestPropertyP13_MigrationPreservesData(t *testing.T) {
	// Counter for unique DB file names across rapid iterations.
	dir := t.TempDir()
	var iterCount int64

	rapid.Check(t, func(t *rapid.T) {
		iterCount++

		// 1. Create a V1-only database: open raw SQLite, run only migrateV1.
		dbPath := filepath.Join(dir, fmt.Sprintf("migration_test_%d.db", iterCount))

		rawDB, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer func() { _ = rawDB.Close() }()

		if _, err := rawDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			t.Fatalf("set WAL: %v", err)
		}

		store := &SQLiteStore{db: rawDB, dbPath: dbPath}

		// Create metadata table and run V1 migration only.
		if _, err := rawDB.Exec(`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`); err != nil {
			t.Fatalf("create metadata: %v", err)
		}
		if err := store.migrateV1(); err != nil {
			t.Fatalf("migrateV1: %v", err)
		}

		// 2. Generate and insert random words via raw SQL (V1 schema, no difficulty column).
		type wordSnapshot struct {
			Word              string
			PartOfSpeech      string
			Article           string
			Definition        string
			EnglishDefinition string
			Example           string
			English           string
			TargetTranslation string
			Notes             string
			Connotation       string
			Register          string
			Collocations      string
			ContrastiveNotes  string
			SecondaryMeanings string
			Tags              string
			SourceLanguage    string
			TargetLanguage    string
			CreatedAt         string
			UpdatedAt         string
		}
		type exprSnapshot struct {
			Expression        string
			Definition        string
			EnglishDefinition string
			Example           string
			English           string
			TargetTranslation string
			Notes             string
			Connotation       string
			Register          string
			ContrastiveNotes  string
			Tags              string
			SourceLanguage    string
			TargetLanguage    string
			CreatedAt         string
			UpdatedAt         string
		}

		safeStr := rapid.StringOfN(rapid.RuneFrom(nil, &unicode.RangeTable{
			R16: []unicode.Range16{{Lo: 0x20, Hi: 0x7E, Stride: 1}},
		}), 1, 30, -1)

		numWords := rapid.IntRange(1, 5).Draw(t, "numWords")
		var insertedWords []wordSnapshot
		for i := 0; i < numWords; i++ {
			w := wordSnapshot{
				Word:              safeStr.Draw(t, fmt.Sprintf("word_%d", i)),
				PartOfSpeech:      safeStr.Draw(t, fmt.Sprintf("pos_%d", i)),
				Article:           safeStr.Draw(t, fmt.Sprintf("article_%d", i)),
				Definition:        safeStr.Draw(t, fmt.Sprintf("def_%d", i)),
				EnglishDefinition: safeStr.Draw(t, fmt.Sprintf("endef_%d", i)),
				Example:           safeStr.Draw(t, fmt.Sprintf("example_%d", i)),
				English:           safeStr.Draw(t, fmt.Sprintf("english_%d", i)),
				TargetTranslation: safeStr.Draw(t, fmt.Sprintf("target_%d", i)),
				Notes:             safeStr.Draw(t, fmt.Sprintf("notes_%d", i)),
				Connotation:       safeStr.Draw(t, fmt.Sprintf("connotation_%d", i)),
				Register:          safeStr.Draw(t, fmt.Sprintf("register_%d", i)),
				Collocations:      safeStr.Draw(t, fmt.Sprintf("collocations_%d", i)),
				ContrastiveNotes:  safeStr.Draw(t, fmt.Sprintf("contrastive_%d", i)),
				SecondaryMeanings: safeStr.Draw(t, fmt.Sprintf("secondary_%d", i)),
				Tags:              safeStr.Draw(t, fmt.Sprintf("tags_%d", i)),
				SourceLanguage:    "nl",
				TargetLanguage:    "hu",
				CreatedAt:         "2025-01-01T00:00:00Z",
				UpdatedAt:         "2025-01-01T00:00:00Z",
			}
			_, err := rawDB.Exec(
				`INSERT INTO words (word, part_of_speech, article, definition, english_definition,
					example, english, target_translation, notes, connotation, register,
					collocations, contrastive_notes, secondary_meanings, tags,
					source_language, target_language, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				w.Word, w.PartOfSpeech, w.Article, w.Definition, w.EnglishDefinition,
				w.Example, w.English, w.TargetTranslation, w.Notes, w.Connotation, w.Register,
				w.Collocations, w.ContrastiveNotes, w.SecondaryMeanings, w.Tags,
				w.SourceLanguage, w.TargetLanguage, w.CreatedAt, w.UpdatedAt,
			)
			if err != nil {
				t.Fatalf("insert word %d: %v", i, err)
			}
			insertedWords = append(insertedWords, w)
		}

		numExprs := rapid.IntRange(1, 5).Draw(t, "numExprs")
		var insertedExprs []exprSnapshot
		for i := 0; i < numExprs; i++ {
			e := exprSnapshot{
				Expression:        safeStr.Draw(t, fmt.Sprintf("expr_%d", i)),
				Definition:        safeStr.Draw(t, fmt.Sprintf("exprdef_%d", i)),
				EnglishDefinition: safeStr.Draw(t, fmt.Sprintf("exprendef_%d", i)),
				Example:           safeStr.Draw(t, fmt.Sprintf("exprexample_%d", i)),
				English:           safeStr.Draw(t, fmt.Sprintf("exprenglish_%d", i)),
				TargetTranslation: safeStr.Draw(t, fmt.Sprintf("exprtarget_%d", i)),
				Notes:             safeStr.Draw(t, fmt.Sprintf("exprnotes_%d", i)),
				Connotation:       safeStr.Draw(t, fmt.Sprintf("exprconnotation_%d", i)),
				Register:          safeStr.Draw(t, fmt.Sprintf("exprregister_%d", i)),
				ContrastiveNotes:  safeStr.Draw(t, fmt.Sprintf("exprcontrastive_%d", i)),
				Tags:              safeStr.Draw(t, fmt.Sprintf("exprtags_%d", i)),
				SourceLanguage:    "nl",
				TargetLanguage:    "hu",
				CreatedAt:         "2025-01-01T00:00:00Z",
				UpdatedAt:         "2025-01-01T00:00:00Z",
			}
			_, err := rawDB.Exec(
				`INSERT INTO expressions (expression, definition, english_definition, example, english,
					target_translation, notes, connotation, register, contrastive_notes, tags,
					source_language, target_language, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				e.Expression, e.Definition, e.EnglishDefinition, e.Example, e.English,
				e.TargetTranslation, e.Notes, e.Connotation, e.Register, e.ContrastiveNotes, e.Tags,
				e.SourceLanguage, e.TargetLanguage, e.CreatedAt, e.UpdatedAt,
			)
			if err != nil {
				t.Fatalf("insert expression %d: %v", i, err)
			}
			insertedExprs = append(insertedExprs, e)
		}

		// 3. Run migrateV2.
		if err := store.migrateV2(); err != nil {
			t.Fatalf("migrateV2: %v", err)
		}

		// 4. Read back all words and verify fields unchanged + difficulty = 'natural'.
		wordRows, err := rawDB.Query(
			`SELECT word, part_of_speech, article, definition, english_definition,
				example, english, target_translation, notes, connotation, register,
				collocations, contrastive_notes, secondary_meanings, tags,
				source_language, target_language, difficulty, created_at, updated_at
			FROM words ORDER BY id`)
		if err != nil {
			t.Fatalf("query words: %v", err)
		}
		defer func() { _ = wordRows.Close() }()

		idx := 0
		for wordRows.Next() {
			var w, pos, art, def, endef, ex, eng, tgt, notes, conn, reg, coll, contr, sec, tags, sl, tl, diff, ca, ua string
			if err := wordRows.Scan(&w, &pos, &art, &def, &endef, &ex, &eng, &tgt, &notes, &conn, &reg, &coll, &contr, &sec, &tags, &sl, &tl, &diff, &ca, &ua); err != nil {
				t.Fatalf("scan word row %d: %v", idx, err)
			}
			if idx >= len(insertedWords) {
				t.Fatalf("more word rows than inserted (%d)", idx)
			}
			orig := insertedWords[idx]
			if diff != "natural" {
				t.Fatalf("word %d: difficulty = %q, want %q", idx, diff, "natural")
			}
			if w != orig.Word || pos != orig.PartOfSpeech || art != orig.Article ||
				def != orig.Definition || endef != orig.EnglishDefinition ||
				ex != orig.Example || eng != orig.English || tgt != orig.TargetTranslation ||
				notes != orig.Notes || conn != orig.Connotation || reg != orig.Register ||
				coll != orig.Collocations || contr != orig.ContrastiveNotes || sec != orig.SecondaryMeanings ||
				tags != orig.Tags || sl != orig.SourceLanguage || tl != orig.TargetLanguage ||
				ca != orig.CreatedAt || ua != orig.UpdatedAt {
				t.Fatalf("word %d: fields changed after migration\ngot:  %+v\nwant: %+v", idx,
					[]string{w, pos, art, def, endef, ex, eng, tgt, notes, conn, reg, coll, contr, sec, tags, sl, tl, ca, ua},
					[]string{orig.Word, orig.PartOfSpeech, orig.Article, orig.Definition, orig.EnglishDefinition,
						orig.Example, orig.English, orig.TargetTranslation, orig.Notes, orig.Connotation, orig.Register,
						orig.Collocations, orig.ContrastiveNotes, orig.SecondaryMeanings, orig.Tags,
						orig.SourceLanguage, orig.TargetLanguage, orig.CreatedAt, orig.UpdatedAt})
			}
			idx++
		}
		if err := wordRows.Err(); err != nil {
			t.Fatalf("word rows iteration: %v", err)
		}
		if idx != len(insertedWords) {
			t.Fatalf("expected %d word rows, got %d", len(insertedWords), idx)
		}

		// 5. Read back all expressions and verify fields unchanged + difficulty = 'natural'.
		exprRows, err := rawDB.Query(
			`SELECT expression, definition, english_definition, example, english,
				target_translation, notes, connotation, register, contrastive_notes, tags,
				source_language, target_language, difficulty, created_at, updated_at
			FROM expressions ORDER BY id`)
		if err != nil {
			t.Fatalf("query expressions: %v", err)
		}
		defer func() { _ = exprRows.Close() }()

		idx = 0
		for exprRows.Next() {
			var expr, def, endef, ex, eng, tgt, notes, conn, reg, contr, tags, sl, tl, diff, ca, ua string
			if err := exprRows.Scan(&expr, &def, &endef, &ex, &eng, &tgt, &notes, &conn, &reg, &contr, &tags, &sl, &tl, &diff, &ca, &ua); err != nil {
				t.Fatalf("scan expression row %d: %v", idx, err)
			}
			if idx >= len(insertedExprs) {
				t.Fatalf("more expression rows than inserted (%d)", idx)
			}
			orig := insertedExprs[idx]
			if diff != "natural" {
				t.Fatalf("expression %d: difficulty = %q, want %q", idx, diff, "natural")
			}
			if expr != orig.Expression || def != orig.Definition || endef != orig.EnglishDefinition ||
				ex != orig.Example || eng != orig.English || tgt != orig.TargetTranslation ||
				notes != orig.Notes || conn != orig.Connotation || reg != orig.Register ||
				contr != orig.ContrastiveNotes || tags != orig.Tags ||
				sl != orig.SourceLanguage || tl != orig.TargetLanguage ||
				ca != orig.CreatedAt || ua != orig.UpdatedAt {
				t.Fatalf("expression %d: fields changed after migration\ngot:  %+v\nwant: %+v", idx,
					[]string{expr, def, endef, ex, eng, tgt, notes, conn, reg, contr, tags, sl, tl, ca, ua},
					[]string{orig.Expression, orig.Definition, orig.EnglishDefinition,
						orig.Example, orig.English, orig.TargetTranslation, orig.Notes, orig.Connotation, orig.Register,
						orig.ContrastiveNotes, orig.Tags, orig.SourceLanguage, orig.TargetLanguage, orig.CreatedAt, orig.UpdatedAt})
			}
			idx++
		}
		if err := exprRows.Err(); err != nil {
			t.Fatalf("expression rows iteration: %v", err)
		}
		if idx != len(insertedExprs) {
			t.Fatalf("expected %d expression rows, got %d", len(insertedExprs), idx)
		}
	})
}

// --- Property Test P14: ListDistinctTags returns sorted deduplicated tags ---

// Feature: flashcards, Property 3: ListDistinctTags returns sorted deduplicated tags
// **Validates: Requirements 4.1, 4.2, 4.3**
func TestPropertyP14_ListDistinctTags(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rapid.Check(t, func(t *rapid.T) {
		// Clean tables between iterations to ensure isolation.
		_, _ = store.db.Exec("DELETE FROM words")
		_, _ = store.db.Exec("DELETE FROM expressions")

		// Generator for a single tag segment: 1-15 printable ASCII chars, no commas.
		tagSegment := rapid.StringOfN(rapid.RuneFrom(nil, &unicode.RangeTable{
			R16: []unicode.Range16{{Lo: 0x21, Hi: 0x2B, Stride: 1}, {Lo: 0x2D, Hi: 0x7E, Stride: 1}},
		}), 1, 15, -1)

		// Generator for a comma-separated tag string with optional whitespace padding.
		tagString := rapid.Custom(func(t *rapid.T) string {
			n := rapid.IntRange(0, 5).Draw(t, "numSegments")
			parts := make([]string, n)
			for i := 0; i < n; i++ {
				pad := rapid.StringOfN(rapid.Just(' '), 0, 3, -1).Draw(t, fmt.Sprintf("pad_%d", i))
				seg := tagSegment.Draw(t, fmt.Sprintf("seg_%d", i))
				parts[i] = pad + seg + pad
			}
			// Occasionally include empty segments (consecutive commas).
			if n > 0 && rapid.Bool().Draw(t, "extraComma") {
				idx := rapid.IntRange(0, len(parts)-1).Draw(t, "emptyIdx")
				parts = append(parts[:idx+1], append([]string{""}, parts[idx+1:]...)...)
			}
			return strings.Join(parts, ",")
		})

		// Insert random words with random tag strings.
		numWords := rapid.IntRange(0, 5).Draw(t, "numWords")
		for i := 0; i < numWords; i++ {
			row := makeWordRow(fmt.Sprintf("w_%d", i), "nl")
			row.Tags = tagString.Draw(t, fmt.Sprintf("wordTags_%d", i))
			if err := store.InsertWord(ctx, row); err != nil {
				t.Fatalf("InsertWord %d: %v", i, err)
			}
		}

		// Insert random expressions with random tag strings.
		numExprs := rapid.IntRange(0, 5).Draw(t, "numExprs")
		for i := 0; i < numExprs; i++ {
			row := makeExpressionRow(fmt.Sprintf("e_%d", i), "nl")
			row.Tags = tagString.Draw(t, fmt.Sprintf("exprTags_%d", i))
			if err := store.InsertExpression(ctx, row); err != nil {
				t.Fatalf("InsertExpression %d: %v", i, err)
			}
		}

		// Call ListDistinctTags.
		got, err := store.ListDistinctTags(ctx)
		if err != nil {
			t.Fatalf("ListDistinctTags: %v", err)
		}

		// Compute expected: collect all tags from all inserted rows, split, trim, deduplicate.
		expected := make(map[string]bool)

		// Re-read all words and expressions to get their tags (including any pre-existing from makeWordRow/makeExpressionRow defaults).
		allWords, _, err := store.ListWords(ctx, ListFilter{Page: 1, PageSize: 10000})
		if err != nil {
			t.Fatalf("ListWords: %v", err)
		}
		for _, w := range allWords {
			for _, tag := range strings.Split(w.Tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					expected[tag] = true
				}
			}
		}
		allExprs, _, err := store.ListExpressions(ctx, ListFilter{Page: 1, PageSize: 10000})
		if err != nil {
			t.Fatalf("ListExpressions: %v", err)
		}
		for _, e := range allExprs {
			for _, tag := range strings.Split(e.Tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					expected[tag] = true
				}
			}
		}

		expectedSlice := make([]string, 0, len(expected))
		for tag := range expected {
			expectedSlice = append(expectedSlice, tag)
		}
		sort.Strings(expectedSlice)

		// Verify: result is non-nil.
		if got == nil {
			t.Fatal("ListDistinctTags returned nil, want non-nil slice")
		}

		// Verify: result is sorted.
		if !sort.StringsAreSorted(got) {
			t.Fatalf("result is not sorted: %v", got)
		}

		// Verify: result has no duplicates.
		seen := make(map[string]bool)
		for _, tag := range got {
			if seen[tag] {
				t.Fatalf("duplicate tag in result: %q", tag)
			}
			seen[tag] = true
		}

		// Verify: result matches expected set exactly.
		if len(got) != len(expectedSlice) {
			t.Fatalf("length mismatch: got %d tags, want %d\ngot:  %v\nwant: %v", len(got), len(expectedSlice), got, expectedSlice)
		}
		for i := range got {
			if got[i] != expectedSlice[i] {
				t.Fatalf("tag mismatch at index %d: got %q, want %q\ngot:  %v\nwant: %v", i, got[i], expectedSlice[i], got, expectedSlice)
			}
		}
	})
}

// --- Unit Test: ListDistinctTags on empty database ---

func TestListDistinctTags_EmptyDB(t *testing.T) {
	// Requirements: 4.4
	store := newTestStore(t)
	ctx := context.Background()

	got, err := store.ListDistinctTags(ctx)
	if err != nil {
		t.Fatalf("ListDistinctTags: %v", err)
	}
	if got == nil {
		t.Fatal("ListDistinctTags returned nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("ListDistinctTags returned %d tags, want 0: %v", len(got), got)
	}
}

// --- Property Test P15: Difficulty rating round-trip ---

// Feature: flashcards, Property 5: Difficulty rating round-trip
// **Validates: Requirements 10.2**
func TestPropertyP15_DifficultyRatingRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	validDifficulties := []string{"easy", "hard", "natural"}

	rapid.Check(t, func(t *rapid.T) {
		// Pick a random difficulty value.
		difficulty := rapid.SampledFrom(validDifficulties).Draw(t, "difficulty")

		// Pick whether to test a word or an expression.
		isWord := rapid.Bool().Draw(t, "isWord")

		if isWord {
			row := makeWordRow(
				rapid.StringMatching(`[a-z]{1,20}`).Draw(t, "word"),
				"nl",
			)
			if err := store.InsertWord(ctx, row); err != nil {
				t.Fatalf("InsertWord: %v", err)
			}

			// Update difficulty.
			if err := store.UpdateWordDifficulty(ctx, row.ID, difficulty); err != nil {
				t.Fatalf("UpdateWordDifficulty: %v", err)
			}

			// Read back and verify.
			got, err := store.GetWord(ctx, row.ID)
			if err != nil {
				t.Fatalf("GetWord: %v", err)
			}
			if got == nil {
				t.Fatal("GetWord returned nil for inserted word")
				return
			}
			if got.Difficulty != difficulty {
				t.Fatalf("word difficulty mismatch: got %q, want %q", got.Difficulty, difficulty)
			}
		} else {
			row := makeExpressionRow(
				rapid.StringMatching(`[a-z]{1,20}`).Draw(t, "expression"),
				"nl",
			)
			if err := store.InsertExpression(ctx, row); err != nil {
				t.Fatalf("InsertExpression: %v", err)
			}

			// Update difficulty.
			if err := store.UpdateExpressionDifficulty(ctx, row.ID, difficulty); err != nil {
				t.Fatalf("UpdateExpressionDifficulty: %v", err)
			}

			// Read back and verify.
			got, err := store.GetExpression(ctx, row.ID)
			if err != nil {
				t.Fatalf("GetExpression: %v", err)
			}
			if got == nil {
				t.Fatal("GetExpression returned nil for inserted expression")
				return
			}
			if got.Difficulty != difficulty {
				t.Fatalf("expression difficulty mismatch: got %q, want %q", got.Difficulty, difficulty)
			}
		}
	})
}

// --- Property Test P16: Difficulty filter ---

// Feature: flashcards, Property 1: Filtered deck contains exactly the matching entries
// **Validates: Requirements 2.1, 3.4, 11.3, 11.4**
func TestPropertyP16_DifficultyFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	validDifficulties := []string{"easy", "hard", "natural"}

	rapid.Check(t, func(t *rapid.T) {
		// Clean tables between iterations.
		_, _ = store.db.Exec("DELETE FROM words")
		_, _ = store.db.Exec("DELETE FROM expressions")

		// 1. Insert random words with random difficulties.
		numWords := rapid.IntRange(1, 8).Draw(t, "numWords")
		type wordEntry struct {
			id         int64
			difficulty string
		}
		words := make([]wordEntry, 0, numWords)
		for i := 0; i < numWords; i++ {
			row := makeWordRow(fmt.Sprintf("w_%d", i), "nl")
			if err := store.InsertWord(ctx, row); err != nil {
				t.Fatalf("InsertWord %d: %v", i, err)
			}
			diff := rapid.SampledFrom(validDifficulties).Draw(t, fmt.Sprintf("wordDiff_%d", i))
			if err := store.UpdateWordDifficulty(ctx, row.ID, diff); err != nil {
				t.Fatalf("UpdateWordDifficulty %d: %v", i, err)
			}
			words = append(words, wordEntry{id: row.ID, difficulty: diff})
		}

		// 2. Insert random expressions with random difficulties.
		numExprs := rapid.IntRange(1, 8).Draw(t, "numExprs")
		type exprEntry struct {
			id         int64
			difficulty string
		}
		exprs := make([]exprEntry, 0, numExprs)
		for i := 0; i < numExprs; i++ {
			row := makeExpressionRow(fmt.Sprintf("e_%d", i), "nl")
			if err := store.InsertExpression(ctx, row); err != nil {
				t.Fatalf("InsertExpression %d: %v", i, err)
			}
			diff := rapid.SampledFrom(validDifficulties).Draw(t, fmt.Sprintf("exprDiff_%d", i))
			if err := store.UpdateExpressionDifficulty(ctx, row.ID, diff); err != nil {
				t.Fatalf("UpdateExpressionDifficulty %d: %v", i, err)
			}
			exprs = append(exprs, exprEntry{id: row.ID, difficulty: diff})
		}

		// 3. Pick a random subset of difficulty values as the filter.
		subsetMask := rapid.IntRange(0, 7).Draw(t, "subsetMask") // 3 bits for 3 values
		var filterDiffs []string
		for i, d := range validDifficulties {
			if subsetMask&(1<<i) != 0 {
				filterDiffs = append(filterDiffs, d)
			}
		}

		// Build a set for quick lookup.
		filterSet := make(map[string]bool, len(filterDiffs))
		for _, d := range filterDiffs {
			filterSet[d] = true
		}

		filter := ListFilter{
			Page:       1,
			PageSize:   10000,
			Difficulty: filterDiffs,
		}

		// 4. Query words with the difficulty filter.
		gotWords, gotWordTotal, err := store.ListWords(ctx, filter)
		if err != nil {
			t.Fatalf("ListWords: %v", err)
		}

		// 5. Compute expected word IDs.
		var expectedWordIDs []int64
		for _, w := range words {
			if len(filterDiffs) == 0 || filterSet[w.difficulty] {
				expectedWordIDs = append(expectedWordIDs, w.id)
			}
		}

		// Verify word count matches.
		if gotWordTotal != len(expectedWordIDs) {
			t.Fatalf("word total mismatch: got %d, want %d (filter=%v)", gotWordTotal, len(expectedWordIDs), filterDiffs)
		}
		if len(gotWords) != len(expectedWordIDs) {
			t.Fatalf("word rows mismatch: got %d, want %d (filter=%v)", len(gotWords), len(expectedWordIDs), filterDiffs)
		}

		gotWordIDSet := make(map[int64]bool, len(gotWords))
		for _, w := range gotWords {
			gotWordIDSet[w.ID] = true
		}
		for _, id := range expectedWordIDs {
			if !gotWordIDSet[id] {
				t.Fatalf("expected word ID %d not in results (filter=%v)", id, filterDiffs)
			}
		}

		// 6. Query expressions with the difficulty filter.
		gotExprs, gotExprTotal, err := store.ListExpressions(ctx, filter)
		if err != nil {
			t.Fatalf("ListExpressions: %v", err)
		}

		// Compute expected expression IDs.
		var expectedExprIDs []int64
		for _, e := range exprs {
			if len(filterDiffs) == 0 || filterSet[e.difficulty] {
				expectedExprIDs = append(expectedExprIDs, e.id)
			}
		}

		// Verify expression count matches.
		if gotExprTotal != len(expectedExprIDs) {
			t.Fatalf("expression total mismatch: got %d, want %d (filter=%v)", gotExprTotal, len(expectedExprIDs), filterDiffs)
		}
		if len(gotExprs) != len(expectedExprIDs) {
			t.Fatalf("expression rows mismatch: got %d, want %d (filter=%v)", len(gotExprs), len(expectedExprIDs), filterDiffs)
		}

		gotExprIDSet := make(map[int64]bool, len(gotExprs))
		for _, e := range gotExprs {
			gotExprIDSet[e.ID] = true
		}
		for _, id := range expectedExprIDs {
			if !gotExprIDSet[id] {
				t.Fatalf("expected expression ID %d not in results (filter=%v)", id, filterDiffs)
			}
		}

		// 7. Verify empty filter returns all entries.
		emptyFilter := ListFilter{Page: 1, PageSize: 10000}
		allWords, allWordTotal, err := store.ListWords(ctx, emptyFilter)
		if err != nil {
			t.Fatalf("ListWords (empty filter): %v", err)
		}
		if allWordTotal != numWords {
			t.Fatalf("empty filter word total: got %d, want %d", allWordTotal, numWords)
		}
		if len(allWords) != numWords {
			t.Fatalf("empty filter word rows: got %d, want %d", len(allWords), numWords)
		}

		allExprs, allExprTotal, err := store.ListExpressions(ctx, emptyFilter)
		if err != nil {
			t.Fatalf("ListExpressions (empty filter): %v", err)
		}
		if allExprTotal != numExprs {
			t.Fatalf("empty filter expression total: got %d, want %d", allExprTotal, numExprs)
		}
		if len(allExprs) != numExprs {
			t.Fatalf("empty filter expression rows: got %d, want %d", len(allExprs), numExprs)
		}
	})
}

// --- Property Test P17: FlashcardItem mapping preserves source data ---

// wordRowToFlashcardItem converts a WordRow to a FlashcardItem.
// This is the canonical mapping used by the flashcard handler.
func wordRowToFlashcardItem(w *WordRow) FlashcardItem {
	return FlashcardItem{
		ID:                w.ID,
		Type:              "word",
		Text:              w.Word,
		Definition:        w.Definition,
		English:           w.English,
		TargetTranslation: w.TargetTranslation,
		Difficulty:        w.Difficulty,
	}
}

// expressionRowToFlashcardItem converts an ExpressionRow to a FlashcardItem.
// This is the canonical mapping used by the flashcard handler.
func expressionRowToFlashcardItem(e *ExpressionRow) FlashcardItem {
	return FlashcardItem{
		ID:                e.ID,
		Type:              "expression",
		Text:              e.Expression,
		Definition:        e.Definition,
		English:           e.English,
		TargetTranslation: e.TargetTranslation,
		Difficulty:        e.Difficulty,
	}
}

// Feature: flashcards, Property 2: FlashcardItem mapping preserves source data
// **Validates: Requirements 2.2**
func TestPropertyP17_FlashcardItemMapping(t *testing.T) {
	validDifficulties := []string{"easy", "hard", "natural"}

	safeStr := rapid.StringOfN(rapid.RuneFrom(nil, &unicode.RangeTable{
		R16: []unicode.Range16{{Lo: 0x20, Hi: 0x7E, Stride: 1}},
	}), 1, 50, -1)

	rapid.Check(t, func(t *rapid.T) {
		isWord := rapid.Bool().Draw(t, "isWord")

		if isWord {
			// Generate a random WordRow.
			w := &WordRow{
				ID:                rapid.Int64Range(1, 1_000_000).Draw(t, "id"),
				Word:              safeStr.Draw(t, "word"),
				Definition:        safeStr.Draw(t, "definition"),
				English:           safeStr.Draw(t, "english"),
				TargetTranslation: safeStr.Draw(t, "targetTranslation"),
				Difficulty:        rapid.SampledFrom(validDifficulties).Draw(t, "difficulty"),
				PartOfSpeech:      safeStr.Draw(t, "pos"),
				Article:           safeStr.Draw(t, "article"),
				SourceLanguage:    "nl",
				TargetLanguage:    "hu",
			}

			item := wordRowToFlashcardItem(w)

			if item.ID != w.ID {
				t.Fatalf("ID mismatch: got %d, want %d", item.ID, w.ID)
			}
			if item.Type != "word" {
				t.Fatalf("Type mismatch: got %q, want %q", item.Type, "word")
			}
			if item.Text != w.Word {
				t.Fatalf("Text mismatch: got %q, want %q", item.Text, w.Word)
			}
			if item.Definition != w.Definition {
				t.Fatalf("Definition mismatch: got %q, want %q", item.Definition, w.Definition)
			}
			if item.English != w.English {
				t.Fatalf("English mismatch: got %q, want %q", item.English, w.English)
			}
			if item.TargetTranslation != w.TargetTranslation {
				t.Fatalf("TargetTranslation mismatch: got %q, want %q", item.TargetTranslation, w.TargetTranslation)
			}
			if item.Difficulty != w.Difficulty {
				t.Fatalf("Difficulty mismatch: got %q, want %q", item.Difficulty, w.Difficulty)
			}
		} else {
			// Generate a random ExpressionRow.
			e := &ExpressionRow{
				ID:                rapid.Int64Range(1, 1_000_000).Draw(t, "id"),
				Expression:        safeStr.Draw(t, "expression"),
				Definition:        safeStr.Draw(t, "definition"),
				English:           safeStr.Draw(t, "english"),
				TargetTranslation: safeStr.Draw(t, "targetTranslation"),
				Difficulty:        rapid.SampledFrom(validDifficulties).Draw(t, "difficulty"),
				SourceLanguage:    "nl",
				TargetLanguage:    "hu",
			}

			item := expressionRowToFlashcardItem(e)

			if item.ID != e.ID {
				t.Fatalf("ID mismatch: got %d, want %d", item.ID, e.ID)
			}
			if item.Type != "expression" {
				t.Fatalf("Type mismatch: got %q, want %q", item.Type, "expression")
			}
			if item.Text != e.Expression {
				t.Fatalf("Text mismatch: got %q, want %q", item.Text, e.Expression)
			}
			if item.Definition != e.Definition {
				t.Fatalf("Definition mismatch: got %q, want %q", item.Definition, e.Definition)
			}
			if item.English != e.English {
				t.Fatalf("English mismatch: got %q, want %q", item.English, e.English)
			}
			if item.TargetTranslation != e.TargetTranslation {
				t.Fatalf("TargetTranslation mismatch: got %q, want %q", item.TargetTranslation, e.TargetTranslation)
			}
			if item.Difficulty != e.Difficulty {
				t.Fatalf("Difficulty mismatch: got %q, want %q", item.Difficulty, e.Difficulty)
			}
		}
	})
}
