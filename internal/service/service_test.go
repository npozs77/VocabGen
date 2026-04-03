package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/output"
	"github.com/user/vocabgen/internal/parsing"
	"pgregory.net/rapid"
)

// newTestStore creates a temp SQLite store for testing.
func newTestStore(t *testing.T) *db.SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// newTempStore creates a temp SQLite store without testing.T (for use inside rapid).
func newTempStore() (*db.SQLiteStore, func(), error) {
	dir, err := os.MkdirTemp("", "service-test-*")
	if err != nil {
		return nil, nil, err
	}
	dbPath := filepath.Join(dir, "test.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, nil, err
	}
	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, cleanup, nil
}

// makeWordRow creates a WordRow for testing.
func makeWordRow(word, sourceLang string) *db.WordRow {
	return &db.WordRow{
		Word:              word,
		PartOfSpeech:      "znw",
		Article:           "het",
		Definition:        "een definitie",
		EnglishDefinition: "a definition",
		Example:           "een voorbeeld",
		English:           "test",
		TargetTranslation: "teszt",
		SourceLanguage:    sourceLang,
		TargetLanguage:    "hu",
		CreatedAt:         "2026-01-01T00:00:00Z",
		UpdatedAt:         "2026-01-01T00:00:00Z",
	}
}

// safeAlphaString generates non-empty alphabetic strings safe for use as tokens.
func safeAlphaString() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-z]{3,10}`)
}

// --- Property Tests ---

// TestPropertyP11_CacheIdempotency verifies that looking up the same token twice
// results in exactly one LLM invocation and one cache hit.
func TestPropertyP11_CacheIdempotency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store, cleanup, err := newTempStore()
		if err != nil {
			rt.Fatalf("create store: %v", err)
		}
		defer cleanup()

		provider := &mockProvider{}
		token := safeAlphaString().Draw(rt, "token")
		ctx := context.Background()

		params := LookupParams{
			SourceLang: "nl",
			LookupType: "word",
			Text:       token,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
		}

		// First lookup — should invoke provider
		result1, err := Lookup(ctx, store, params)
		if err != nil {
			rt.Fatalf("first lookup failed: %v", err)
		}
		if result1.FromCache {
			rt.Fatal("first lookup should not be from cache")
		}
		if provider.invocations.Load() != 1 {
			rt.Fatalf("expected 1 invocation after first lookup, got %d", provider.invocations.Load())
		}

		// Second lookup — should be cache hit
		result2, err := Lookup(ctx, store, params)
		if err != nil {
			rt.Fatalf("second lookup failed: %v", err)
		}
		if !result2.FromCache {
			rt.Fatal("second lookup should be from cache")
		}
		if provider.invocations.Load() != 1 {
			rt.Fatalf("expected 1 invocation after second lookup, got %d", provider.invocations.Load())
		}

		// Results should be identical
		if result1.Entry.Definition != result2.Entry.Definition {
			rt.Fatalf("definitions differ: %q vs %q", result1.Entry.Definition, result2.Entry.Definition)
		}
		if result1.Entry.English != result2.Entry.English {
			rt.Fatalf("english translations differ: %q vs %q", result1.Entry.English, result2.Entry.English)
		}
	})
}

// TestPropertyP13_DryRunNoSideEffects verifies dry-run mode doesn't invoke
// the provider or write to the database.
func TestPropertyP13_DryRunNoSideEffects(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store, cleanup, err := newTempStore()
		if err != nil {
			rt.Fatalf("create store: %v", err)
		}
		defer cleanup()

		provider := &panicProvider{}
		n := rapid.IntRange(1, 20).Draw(rt, "tokenCount")
		tokens := make([]parsing.TokenWithContext, n)
		for i := range tokens {
			tokens[i] = parsing.TokenWithContext{
				Token: safeAlphaString().Draw(rt, "token"),
			}
		}

		ctx := context.Background()
		params := BatchParams{
			SourceLang: "nl",
			Mode:       "words",
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
			DryRun:     true,
		}

		result, err := ProcessBatch(ctx, store, params)
		if err != nil {
			rt.Fatalf("dry-run batch failed: %v", err)
		}

		// Verify no DB entries were created
		words, _, err := store.ListWords(ctx, db.ListFilter{SourceLang: "nl", PageSize: 100})
		if err != nil {
			rt.Fatalf("list words failed: %v", err)
		}
		if len(words) != 0 {
			rt.Fatalf("expected 0 DB entries in dry-run, got %d", len(words))
		}

		// All non-empty tokens should be counted as processed
		expectedProcessed := 0
		for _, tc := range tokens {
			if parsing.NormalizeWord(tc.Token) != "" {
				expectedProcessed++
			}
		}
		if result.Processed != expectedProcessed {
			rt.Fatalf("expected %d processed, got %d", expectedProcessed, result.Processed)
		}
	})
}

// TestPropertyP15_LimitEnforcement verifies that ProcessBatch respects the limit.
func TestPropertyP15_LimitEnforcement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store, cleanup, err := newTempStore()
		if err != nil {
			rt.Fatalf("create store: %v", err)
		}
		defer cleanup()

		provider := &countingMockProvider{}
		limit := rapid.IntRange(1, 10).Draw(rt, "limit")
		tokenCount := rapid.IntRange(limit+1, limit+20).Draw(rt, "tokenCount")

		tokens := make([]parsing.TokenWithContext, tokenCount)
		for i := range tokens {
			tokens[i] = parsing.TokenWithContext{
				Token: safeAlphaString().Draw(rt, "token") + rapid.StringMatching(`[0-9]{4}`).Draw(rt, "suffix"),
			}
		}

		ctx := context.Background()
		params := BatchParams{
			SourceLang: "nl",
			Mode:       "words",
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
			Limit:      limit,
		}

		_, err = ProcessBatch(ctx, store, params)
		if err != nil {
			rt.Fatalf("batch failed: %v", err)
		}

		if int(provider.invocations.Load()) > limit {
			rt.Fatalf("expected at most %d invocations, got %d", limit, provider.invocations.Load())
		}
	})
}

// TestPropertyP16_ErrorResilience verifies that batch processing continues
// after per-item failures.
func TestPropertyP16_ErrorResilience(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store, cleanup, err := newTempStore()
		if err != nil {
			rt.Fatalf("create store: %v", err)
		}
		defer cleanup()

		tokenCount := rapid.IntRange(3, 15).Draw(rt, "tokenCount")
		tokens := make([]parsing.TokenWithContext, tokenCount)
		failTokens := make(map[string]bool)

		for i := range tokens {
			tok := safeAlphaString().Draw(rt, "token") + rapid.StringMatching(`[0-9]{4}`).Draw(rt, "suffix")
			tokens[i] = parsing.TokenWithContext{Token: tok}
			if rapid.Bool().Draw(rt, "shouldFail") {
				failTokens[parsing.NormalizeWord(tok)] = true
			}
		}

		provider := newFailingMockProvider(failTokens)
		ctx := context.Background()
		params := BatchParams{
			SourceLang: "nl",
			Mode:       "words",
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
		}

		result, err := ProcessBatch(ctx, store, params)
		if err != nil {
			rt.Fatalf("batch failed: %v", err)
		}

		// Count non-empty tokens
		nonEmpty := 0
		for _, tc := range tokens {
			if parsing.NormalizeWord(tc.Token) != "" {
				nonEmpty++
			}
		}

		total := result.Processed + result.Failed + result.Cached + result.Skipped
		if total != nonEmpty {
			rt.Fatalf("expected total=%d, got processed=%d failed=%d cached=%d skipped=%d (total=%d)",
				nonEmpty, result.Processed, result.Failed, result.Cached, result.Skipped, total)
		}
	})
}

// TestPropertyP18_MultiVersionEntryIntegrity verifies conflict resolution
// strategies maintain entry integrity.
func TestPropertyP18_MultiVersionEntryIntegrity(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ctx := context.Background()
		word := safeAlphaString().Draw(rt, "word")
		n := rapid.IntRange(1, 5).Draw(rt, "existingCount")

		newEntry := &output.Entry{
			Word:              word,
			Type:              "ww",
			Article:           "—",
			Definition:        "new definition",
			EnglishDefinition: "new english def",
			Example:           "new example",
			English:           "new english",
			TargetTranslation: "new target",
		}

		// Test "add" strategy
		{
			store, cleanup, err := newTempStore()
			if err != nil {
				rt.Fatalf("create store: %v", err)
			}
			for i := 0; i < n; i++ {
				row := makeWordRow(word, "nl")
				row.Definition = rapid.String().Draw(rt, "def")
				if err := store.InsertWord(ctx, row); err != nil {
					rt.Fatalf("insert failed: %v", err)
				}
			}
			entries, _ := store.FindWords(ctx, word, "nl")
			err = ResolveConflict(ctx, store, ConflictAdd, "words", newEntry, entries[0].ID, "nl", "hu", "")
			if err != nil {
				rt.Fatalf("add resolve failed: %v", err)
			}
			after, _ := store.FindWords(ctx, word, "nl")
			if len(after) != n+1 {
				rt.Fatalf("expected %d entries after add, got %d", n+1, len(after))
			}
			cleanup()
		}

		// Test "replace" strategy
		{
			store, cleanup, err := newTempStore()
			if err != nil {
				rt.Fatalf("create store: %v", err)
			}
			for i := 0; i < n; i++ {
				row := makeWordRow(word, "nl")
				if err := store.InsertWord(ctx, row); err != nil {
					rt.Fatalf("insert failed: %v", err)
				}
			}
			entries, _ := store.FindWords(ctx, word, "nl")
			targetID := entries[0].ID
			err = ResolveConflict(ctx, store, ConflictReplace, "words", newEntry, targetID, "nl", "hu", "")
			if err != nil {
				rt.Fatalf("replace resolve failed: %v", err)
			}
			after, _ := store.FindWords(ctx, word, "nl")
			if len(after) != n {
				rt.Fatalf("expected %d entries after replace, got %d", n, len(after))
			}
			for _, e := range after {
				if e.ID == targetID && e.Definition != "new definition" {
					rt.Fatalf("replaced entry not updated: %q", e.Definition)
				}
			}
			cleanup()
		}

		// Test "skip" strategy
		{
			store, cleanup, err := newTempStore()
			if err != nil {
				rt.Fatalf("create store: %v", err)
			}
			for i := 0; i < n; i++ {
				row := makeWordRow(word, "nl")
				if err := store.InsertWord(ctx, row); err != nil {
					rt.Fatalf("insert failed: %v", err)
				}
			}
			entries, _ := store.FindWords(ctx, word, "nl")
			err = ResolveConflict(ctx, store, ConflictSkip, "words", newEntry, entries[0].ID, "nl", "hu", "")
			if err != nil {
				rt.Fatalf("skip resolve failed: %v", err)
			}
			after, _ := store.FindWords(ctx, word, "nl")
			if len(after) != n {
				rt.Fatalf("expected %d entries after skip, got %d", n, len(after))
			}
			cleanup()
		}

		// Test FindWords returns empty slice for non-existent word
		{
			store, cleanup, err := newTempStore()
			if err != nil {
				rt.Fatalf("create store: %v", err)
			}
			empty, err := store.FindWords(ctx, "nonexistent_xyz", "nl")
			if err != nil {
				rt.Fatalf("find non-existent failed: %v", err)
			}
			if len(empty) != 0 {
				rt.Fatalf("expected empty slice, got %d", len(empty))
			}
			cleanup()
		}
	})
}

// TestPropertyP19_ContextAwareCacheBypass verifies cache bypass behavior.
func TestPropertyP19_ContextAwareCacheBypass(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store, cleanup, err := newTempStore()
		if err != nil {
			rt.Fatalf("create store: %v", err)
		}
		defer cleanup()

		ctx := context.Background()
		word := safeAlphaString().Draw(rt, "word")

		// Insert an existing entry
		row := makeWordRow(word, "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			rt.Fatalf("insert failed: %v", err)
		}

		// Lookup with empty context → cache hit, no provider invocation
		provider1 := &countingMockProvider{}
		result, err := Lookup(ctx, store, LookupParams{
			SourceLang: "nl",
			LookupType: "word",
			Text:       word,
			Provider:   provider1,
			ModelID:    "test-model",
			TargetLang: "hu",
		})
		if err != nil {
			rt.Fatalf("cache lookup failed: %v", err)
		}
		if !result.FromCache {
			rt.Fatal("expected cache hit with empty context")
		}
		if provider1.invocations.Load() != 0 {
			rt.Fatalf("expected 0 invocations for cache hit, got %d", provider1.invocations.Load())
		}

		// Lookup with context → cache bypass, provider invoked once
		provider2 := &countingMockProvider{}
		resultCtx, err := Lookup(ctx, store, LookupParams{
			SourceLang: "nl",
			LookupType: "word",
			Text:       word,
			Provider:   provider2,
			ModelID:    "test-model",
			Context:    "Dit is een context zin.",
			TargetLang: "hu",
			OnConflict: ConflictSkip,
		})
		if err != nil {
			rt.Fatalf("context lookup failed: %v", err)
		}
		if provider2.invocations.Load() != 1 {
			rt.Fatalf("expected 1 invocation for context bypass, got %d", provider2.invocations.Load())
		}
		if resultCtx.Entry == nil {
			rt.Fatal("expected non-nil entry from context bypass")
		}
		if len(resultCtx.Existing) == 0 {
			rt.Fatal("expected existing entries in context bypass result")
		}

		// Lookup with no existing entry → provider invoked once
		provider3 := &countingMockProvider{}
		newWord := safeAlphaString().Draw(rt, "newWord") + "unique"
		resultNew, err := Lookup(ctx, store, LookupParams{
			SourceLang: "nl",
			LookupType: "word",
			Text:       newWord,
			Provider:   provider3,
			ModelID:    "test-model",
			TargetLang: "hu",
		})
		if err != nil {
			rt.Fatalf("new word lookup failed: %v", err)
		}
		if provider3.invocations.Load() != 1 {
			rt.Fatalf("expected 1 invocation for new word, got %d", provider3.invocations.Load())
		}
		if resultNew.FromCache {
			rt.Fatal("new word should not be from cache")
		}
	})
}

// --- Integration Tests ---

// TestIntegration_FullLookupFlow tests a complete lookup with mock provider and real DB.
func TestIntegration_FullLookupFlow(t *testing.T) {
	store := newTestStore(t)
	provider := &mockProvider{}
	ctx := context.Background()

	params := LookupParams{
		SourceLang: "nl",
		LookupType: "word",
		Text:       "uitkomen",
		Provider:   provider,
		ModelID:    "test-model",
		TargetLang: "hu",
		Tags:       "chapter-1",
	}

	result, err := Lookup(ctx, store, params)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if result.Entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if result.FromCache {
		t.Fatal("first lookup should not be from cache")
	}
	if result.Entry.Definition == "" {
		t.Fatal("expected non-empty definition")
	}
	if result.Entry.Tags != "chapter-1" {
		t.Fatalf("expected tags 'chapter-1', got %q", result.Entry.Tags)
	}

	// Verify DB contains the entry
	rows, err := store.FindWords(ctx, "uitkomen", "nl")
	if err != nil {
		t.Fatalf("find words failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 DB entry, got %d", len(rows))
	}
	if rows[0].SourceLanguage != "nl" {
		t.Fatalf("expected source_language 'nl', got %q", rows[0].SourceLanguage)
	}
	if rows[0].TargetLanguage != "hu" {
		t.Fatalf("expected target_language 'hu', got %q", rows[0].TargetLanguage)
	}
	if rows[0].CreatedAt == "" {
		t.Fatal("expected non-empty created_at")
	}
}

// TestIntegration_BatchCacheHits processes tokens twice and verifies cache behavior.
func TestIntegration_BatchCacheHits(t *testing.T) {
	store := newTestStore(t)
	provider := &countingMockProvider{}
	ctx := context.Background()

	tokens := []parsing.TokenWithContext{
		{Token: "huis"},
		{Token: "werk"},
		{Token: "boek"},
	}

	params := BatchParams{
		SourceLang: "nl",
		Mode:       "words",
		Tokens:     tokens,
		Provider:   provider,
		ModelID:    "test-model",
		TargetLang: "hu",
	}

	// First run — all new
	result1, err := ProcessBatch(ctx, store, params)
	if err != nil {
		t.Fatalf("first batch failed: %v", err)
	}
	if result1.Processed != 3 {
		t.Fatalf("expected 3 processed, got %d", result1.Processed)
	}
	firstRunInvocations := provider.invocations.Load()
	if firstRunInvocations != 3 {
		t.Fatalf("expected 3 invocations, got %d", firstRunInvocations)
	}

	// Second run — all cache hits
	result2, err := ProcessBatch(ctx, store, params)
	if err != nil {
		t.Fatalf("second batch failed: %v", err)
	}
	if result2.Cached != 3 {
		t.Fatalf("expected 3 cached, got %d", result2.Cached)
	}
	if result2.Processed != 0 {
		t.Fatalf("expected 0 processed on second run, got %d", result2.Processed)
	}
	if provider.invocations.Load() != firstRunInvocations {
		t.Fatalf("expected no new invocations, got %d total", provider.invocations.Load())
	}
}

// TestIntegration_ConflictResolutionFlows tests all conflict resolution paths.
func TestIntegration_ConflictResolutionFlows(t *testing.T) {
	ctx := context.Background()

	t.Run("lookup_with_context_needs_resolution", func(t *testing.T) {
		store := newTestStore(t)
		provider := &mockProvider{}

		row := makeWordRow("werk", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		result, err := Lookup(ctx, store, LookupParams{
			SourceLang: "nl",
			LookupType: "word",
			Text:       "werk",
			Provider:   provider,
			ModelID:    "test-model",
			Context:    "Ik ga naar mijn werk.",
			TargetLang: "hu",
		})
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		if !result.NeedsResolution {
			t.Fatal("expected NeedsResolution=true")
		}
		if len(result.Existing) != 1 {
			t.Fatalf("expected 1 existing entry, got %d", len(result.Existing))
		}
		if len(result.ExistingIDs) != 1 {
			t.Fatalf("expected 1 existing ID, got %d", len(result.ExistingIDs))
		}
	})

	t.Run("resolve_replace_updates_entry", func(t *testing.T) {
		store := newTestStore(t)

		row := makeWordRow("werk", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		entries, _ := store.FindWords(ctx, "werk", "nl")
		targetID := entries[0].ID

		newEntry := &output.Entry{
			Word:              "werk",
			Type:              "ww",
			Article:           "—",
			Definition:        "updated definition",
			Example:           "updated example",
			English:           "work (verb)",
			TargetTranslation: "dolgozni",
		}

		err := ResolveConflict(ctx, store, ConflictReplace, "words", newEntry, targetID, "nl", "hu", "")
		if err != nil {
			t.Fatalf("replace failed: %v", err)
		}

		after, _ := store.FindWords(ctx, "werk", "nl")
		if len(after) != 1 {
			t.Fatalf("expected 1 entry after replace, got %d", len(after))
		}
		if after[0].Definition != "updated definition" {
			t.Fatalf("expected updated definition, got %q", after[0].Definition)
		}
	})

	t.Run("resolve_add_inserts_alongside", func(t *testing.T) {
		store := newTestStore(t)

		row := makeWordRow("werk", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		entries, _ := store.FindWords(ctx, "werk", "nl")

		newEntry := &output.Entry{
			Word:              "werk",
			Type:              "ww",
			Definition:        "verb definition",
			English:           "to work",
			TargetTranslation: "dolgozni",
		}

		err := ResolveConflict(ctx, store, ConflictAdd, "words", newEntry, entries[0].ID, "nl", "hu", "")
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}

		after, _ := store.FindWords(ctx, "werk", "nl")
		if len(after) != 2 {
			t.Fatalf("expected 2 entries after add, got %d", len(after))
		}
	})

	t.Run("batch_replace_with_context", func(t *testing.T) {
		store := newTestStore(t)
		provider := &countingMockProvider{}

		row := makeWordRow("huis", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		tokens := []parsing.TokenWithContext{
			{Token: "huis", Context: "Ik woon in een groot huis."},
		}

		result, err := ProcessBatch(ctx, store, BatchParams{
			SourceLang: "nl",
			Mode:       "words",
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
			OnConflict: ConflictReplace,
		})
		if err != nil {
			t.Fatalf("batch failed: %v", err)
		}
		if result.Replaced != 1 {
			t.Fatalf("expected 1 replaced, got %d", result.Replaced)
		}
		if provider.invocations.Load() != 1 {
			t.Fatalf("expected 1 invocation, got %d", provider.invocations.Load())
		}
	})

	t.Run("batch_skip_with_context", func(t *testing.T) {
		store := newTestStore(t)
		provider := &countingMockProvider{}

		row := makeWordRow("boek", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		tokens := []parsing.TokenWithContext{
			{Token: "boek", Context: "Ik lees een boek."},
		}

		result, err := ProcessBatch(ctx, store, BatchParams{
			SourceLang: "nl",
			Mode:       "words",
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
			OnConflict: ConflictSkip,
		})
		if err != nil {
			t.Fatalf("batch failed: %v", err)
		}
		if result.Skipped != 1 {
			t.Fatalf("expected 1 skipped, got %d", result.Skipped)
		}
	})

	t.Run("batch_summary_counts", func(t *testing.T) {
		store := newTestStore(t)
		provider := &countingMockProvider{}

		row := makeWordRow("oud", "nl")
		if err := store.InsertWord(ctx, row); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		tokens := []parsing.TokenWithContext{
			{Token: "oud", Context: "Het is een oud huis."}, // existing + context → replace
			{Token: "nieuw"}, // new → processed
			{Token: "oud"},   // existing, no context → cached
		}

		result, err := ProcessBatch(ctx, store, BatchParams{
			SourceLang: "nl",
			Mode:       "words",
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    "test-model",
			TargetLang: "hu",
			OnConflict: ConflictReplace,
		})
		if err != nil {
			t.Fatalf("batch failed: %v", err)
		}
		if result.Replaced != 1 {
			t.Fatalf("expected 1 replaced, got %d", result.Replaced)
		}
		if result.Processed != 1 {
			t.Fatalf("expected 1 processed, got %d", result.Processed)
		}
		if result.Cached != 1 {
			t.Fatalf("expected 1 cached, got %d", result.Cached)
		}
	})
}

// --- Unit Tests ---

// TestParseConflictStrategy tests conflict strategy parsing.
func TestParseConflictStrategy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ConflictStrategy
		wantErr bool
	}{
		{"replace", "replace", ConflictReplace, false},
		{"add", "add", ConflictAdd, false},
		{"skip", "skip", ConflictSkip, false},
		{"invalid", "invalid", "", true},
		{"empty", "", "", true},
		{"uppercase", "Replace", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseConflictStrategy(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// TestGetSupportedLanguages verifies the language list is returned sorted.
func TestGetSupportedLanguages(t *testing.T) {
	langs := GetSupportedLanguages()
	if len(langs) == 0 {
		t.Fatal("expected non-empty language list")
	}

	// Verify sorted by name
	for i := 1; i < len(langs); i++ {
		if langs[i].Name < langs[i-1].Name {
			t.Fatalf("languages not sorted: %q before %q", langs[i-1].Name, langs[i].Name)
		}
	}

	// Verify Dutch is present
	found := false
	for _, l := range langs {
		if l.Code == "nl" && l.Name == "Dutch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Dutch (nl) in supported languages")
	}
}

// TestLookupEmptyToken verifies that an empty token returns an error.
func TestLookupEmptyToken(t *testing.T) {
	store := newTestStore(t)
	provider := &mockProvider{}
	ctx := context.Background()

	_, err := Lookup(ctx, store, LookupParams{
		SourceLang: "nl",
		LookupType: "word",
		Text:       "   ",
		Provider:   provider,
		ModelID:    "test-model",
		TargetLang: "hu",
	})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

// TestLookupDryRun verifies dry-run returns without DB or LLM interaction.
func TestLookupDryRun(t *testing.T) {
	store := newTestStore(t)
	provider := &panicProvider{}
	ctx := context.Background()

	result, err := Lookup(ctx, store, LookupParams{
		SourceLang: "nl",
		LookupType: "word",
		Text:       "huis",
		Provider:   provider,
		ModelID:    "test-model",
		TargetLang: "hu",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("dry-run lookup failed: %v", err)
	}
	if result.Entry == nil {
		t.Fatal("expected non-nil entry in dry-run")
	}
}

// TestLookupExpression verifies expression lookup flow.
func TestLookupExpression(t *testing.T) {
	store := newTestStore(t)
	provider := &mockExprProvider{}
	ctx := context.Background()

	result, err := Lookup(ctx, store, LookupParams{
		SourceLang: "nl",
		LookupType: "expression",
		Text:       "op de hoogte",
		Provider:   provider,
		ModelID:    "test-model",
		TargetLang: "hu",
	})
	if err != nil {
		t.Fatalf("expression lookup failed: %v", err)
	}
	if result.Entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if result.FromCache {
		t.Fatal("first lookup should not be from cache")
	}

	rows, err := store.FindExpressions(ctx, "op de hoogte", "nl")
	if err != nil {
		t.Fatalf("find expressions failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 DB entry, got %d", len(rows))
	}
}

// TestBatchSkipsEmptyTokens verifies empty tokens are skipped.
func TestBatchSkipsEmptyTokens(t *testing.T) {
	store := newTestStore(t)
	provider := &countingMockProvider{}
	ctx := context.Background()

	tokens := []parsing.TokenWithContext{
		{Token: "huis"},
		{Token: "   "},
		{Token: ""},
		{Token: "boek"},
	}

	result, err := ProcessBatch(ctx, store, BatchParams{
		SourceLang: "nl",
		Mode:       "words",
		Tokens:     tokens,
		Provider:   provider,
		ModelID:    "test-model",
		TargetLang: "hu",
	})
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}
	if result.Skipped != 2 {
		t.Fatalf("expected 2 skipped, got %d", result.Skipped)
	}
	if result.Processed != 2 {
		t.Fatalf("expected 2 processed, got %d", result.Processed)
	}
}

func TestIsValidToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple word", "gezellig", true},
		{"word with spaces", "op de hoogte", true},
		{"word with hyphen", "niet-roker", true},
		{"word with apostrophe", "s'avonds", true},
		{"word with parens", "lopen (liep)", true},
		{"unicode letters", "ő ű ë ï", true},
		{"cyrillic", "делать", true},
		{"contains digit", "test123", false},
		{"contains special char", "hello@world", false},
		{"contains exclamation", "wow!", false},
		{"only digits", "12345", false},
		{"mixed valid and digit", "werk2", false},
		{"empty string", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidToken(tc.input)
			if got != tc.want {
				t.Errorf("isValidToken(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCheckHallucination(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		example  string
		wantWarn bool
	}{
		{"token in example", "huis", "Het huis is groot.", false},
		{"token not in example", "ervel", "De erwten zijn lekker.", true},
		{"empty example", "huis", "", false},
		{"prefix match (conjugation)", "werken", "Hij werkt elke dag.", false},
		{"case insensitive", "Huis", "het huis is groot", false},
		{"dash example", "werk", "—", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkHallucination(tc.token, tc.example)
			if (got != "") != tc.wantWarn {
				t.Errorf("checkHallucination(%q, %q) = %q, wantWarn=%v", tc.token, tc.example, got, tc.wantWarn)
			}
		})
	}
}

func TestCheckNonWord(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		entry    *output.Entry
		wantWarn bool
	}{
		{"valid word", "huis", &output.Entry{Word: "huis", Type: "znw", Definition: "een gebouw", Example: "Het huis is groot."}, false},
		{"type is dash", "xyz", &output.Entry{Word: "xyz", Type: "—", Definition: "not valid", Example: "—"}, true},
		{"type is empty", "xyz", &output.Entry{Word: "xyz", Type: "", Definition: "something", Example: "test"}, true},
		{"definition says not valid", "abc", &output.Entry{Word: "abc", Type: "znw", Definition: "Dit is geen geldig Nederlands woord", Example: "test"}, true},
		{"example is dash", "abc", &output.Entry{Word: "abc", Type: "znw", Definition: "something", Example: "—"}, true},
		{"example is empty", "abc", &output.Entry{Word: "abc", Type: "znw", Definition: "something", Example: ""}, true},
		{"expression skips type check", "op de hoogte", &output.Entry{Expression: "op de hoogte", Type: "", Definition: "iets weten", Example: "test"}, false},
		{"nil entry", "test", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkNonWord(tc.token, tc.entry)
			if (got != "") != tc.wantWarn {
				t.Errorf("checkNonWord(%q) = %q, wantWarn=%v", tc.token, got, tc.wantWarn)
			}
		})
	}
}

func TestLookup_RejectsInvalidToken(t *testing.T) {
	store, cleanup, err := newTempStore()
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	_, err = Lookup(ctx, store, LookupParams{
		SourceLang: "nl",
		LookupType: "word",
		Text:       "test123",
		Provider:   &mockProvider{},
		ModelID:    "test",
		TargetLang: "hu",
	})
	if err == nil {
		t.Fatal("expected error for token with digits, got nil")
	}
	if !strings.Contains(err.Error(), "invalid input") {
		t.Errorf("expected 'invalid input' error, got: %v", err)
	}
}
