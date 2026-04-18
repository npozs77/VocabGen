package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/language"
	"github.com/user/vocabgen/internal/llm"
	"github.com/user/vocabgen/internal/output"
	"github.com/user/vocabgen/internal/parsing"
)

// LookupParams holds all parameters for a single vocabulary lookup.
type LookupParams struct {
	SourceLang string
	LookupType string // "word", "expression", or "sentence"
	Text       string
	Provider   llm.Provider
	ModelID    string
	Context    string
	TargetLang string
	Tags       string
	DryRun     bool
	Timeout    time.Duration
	OnConflict ConflictStrategy
	ReplaceID  int64
}

// LookupResult holds the outcome of a single lookup, including conflict info.
type LookupResult struct {
	Entry           *output.Entry
	Existing        []output.Entry
	ExistingIDs     []int64
	NeedsResolution bool
	FromCache       bool
	Warning         string // non-empty when a potential issue is detected (e.g., hallucination)
}

// mode returns the prompt mode string based on the lookup type.
func mode(lookupType string) string {
	if lookupType == "expression" || lookupType == "sentence" {
		return "expressions"
	}
	return "words"
}

// checkHallucination checks if the input token appears in the example sentence.
// Returns a warning string if the token is missing (possible hallucination), or "".
func checkHallucination(token, example string) string {
	if example == "" {
		return ""
	}
	lower := strings.ToLower(example)
	tokenLower := strings.ToLower(strings.TrimSpace(token))
	if tokenLower == "" {
		return ""
	}
	// Check for the full token first
	if strings.Contains(lower, tokenLower) {
		return ""
	}
	// Check for a prefix (≥3 chars) to catch conjugations/declensions
	if len(tokenLower) >= 4 {
		prefix := tokenLower[:len(tokenLower)*2/3] // first 2/3 of the token
		if len(prefix) >= 3 && strings.Contains(lower, prefix) {
			return ""
		}
	}
	return fmt.Sprintf("⚠ \"%s\" not found in example sentence — possible hallucination", token)
}

// checkNonWord detects when the LLM recognized the input as invalid/nonsensical.
// Returns a warning if the response contains markers like "—" for type or
// phrases indicating the word doesn't exist.
func checkNonWord(token string, entry *output.Entry) string {
	if entry == nil {
		return ""
	}
	// Type check only for words (expressions don't have a type field)
	if entry.Word != "" && (entry.Type == "—" || entry.Type == "") {
		return fmt.Sprintf("⚠ \"%s\" — LLM could not determine part of speech (possible non-word)", token)
	}
	// Definition contains markers indicating the LLM recognized it as invalid
	defLower := strings.ToLower(entry.Definition)
	markers := []string{
		"geen geldig", "niet bestaand", "geen bestaand", "does not exist",
		"not a valid", "not a real", "is not a word", "geen woord",
		"no meaning", "geen betekenis", "not recognized",
	}
	for _, m := range markers {
		if strings.Contains(defLower, m) {
			return fmt.Sprintf("⚠ \"%s\" — LLM indicates this is not a valid word", token)
		}
	}
	// Example is "—" or empty — LLM couldn't produce an example
	if entry.Example == "—" || entry.Example == "" {
		return fmt.Sprintf("⚠ \"%s\" — no example sentence produced (possible non-word)", token)
	}
	return ""
}

// checkQuality runs all quality checks on an LLM result and returns the first warning found.
func checkQuality(token string, entry *output.Entry) string {
	if w := checkNonWord(token, entry); w != "" {
		return w
	}
	return checkHallucination(token, entry.Example)
}

// isValidToken checks that a token contains only letters (any script), spaces,
// hyphens, apostrophes, and parentheses. Rejects digits and other special characters.
func isValidToken(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || r == '-' || r == '\'' || r == '(' || r == ')' || r == 'ʼ' {
			continue
		}
		return false
	}
	return true
}

// invokeLLM builds a prompt, invokes the provider, validates the response,
// and maps the fields to an output Entry.
func invokeLLM(ctx context.Context, p llm.Provider, modelID, sourceLang, m, token, ctxSentence, targetLang string) (*output.Entry, error) {
	prompt, err := language.BuildPrompt(sourceLang, m, token, ctxSentence, targetLang)
	if err != nil {
		return nil, err
	}

	slog.Debug("sending prompt to LLM",
		slog.String("provider", p.Name()),
		slog.String("model", modelID),
		slog.String("token", token),
		slog.String("mode", m),
	)

	raw, err := p.Invoke(ctx, prompt, modelID)
	if err != nil {
		slog.Error("LLM invocation failed",
			slog.String("provider", p.Name()),
			slog.String("token", token),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	slog.Debug("received LLM response",
		slog.String("provider", p.Name()),
		slog.String("token", token),
		slog.Int("response_len", len(raw)),
	)

	validated, err := language.ValidateResponse(m, raw)
	if err != nil {
		slog.Error("validation failed",
			slog.String("token", token),
			slog.String("mode", m),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	return output.MapFields(validated, m), nil
}

// wordRowToEntry converts a db.WordRow to an output.Entry.
func wordRowToEntry(r *db.WordRow) *output.Entry {
	return &output.Entry{
		Word:              r.Word,
		Type:              r.PartOfSpeech,
		Article:           r.Article,
		Definition:        r.Definition,
		EnglishDefinition: r.EnglishDefinition,
		Example:           r.Example,
		English:           r.English,
		TargetTranslation: r.TargetTranslation,
		Notes:             r.Notes,
		Connotation:       r.Connotation,
		Register:          r.Register,
		Collocations:      r.Collocations,
		ContrastiveNotes:  r.ContrastiveNotes,
		SecondaryMeanings: r.SecondaryMeanings,
		Tags:              r.Tags,
	}
}

// exprRowToEntry converts a db.ExpressionRow to an output.Entry.
func exprRowToEntry(r *db.ExpressionRow) *output.Entry {
	return &output.Entry{
		Expression:        r.Expression,
		Definition:        r.Definition,
		EnglishDefinition: r.EnglishDefinition,
		Example:           r.Example,
		English:           r.English,
		TargetTranslation: r.TargetTranslation,
		Notes:             r.Notes,
		Connotation:       r.Connotation,
		Register:          r.Register,
		ContrastiveNotes:  r.ContrastiveNotes,
		Tags:              r.Tags,
	}
}

// entryToWordRow converts an output.Entry to a db.WordRow for insertion.
func entryToWordRow(e *output.Entry, sourceLang, targetLang, tags string) *db.WordRow {
	now := time.Now().UTC().Format(time.RFC3339)
	t := tags
	if e.Tags != "" {
		t = e.Tags
	}
	return &db.WordRow{
		Word:              e.Word,
		PartOfSpeech:      e.Type,
		Article:           e.Article,
		Definition:        e.Definition,
		EnglishDefinition: e.EnglishDefinition,
		Example:           e.Example,
		English:           e.English,
		TargetTranslation: e.TargetTranslation,
		Notes:             e.Notes,
		Connotation:       e.Connotation,
		Register:          e.Register,
		Collocations:      e.Collocations,
		ContrastiveNotes:  e.ContrastiveNotes,
		SecondaryMeanings: e.SecondaryMeanings,
		Tags:              t,
		SourceLanguage:    sourceLang,
		TargetLanguage:    targetLang,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// entryToExprRow converts an output.Entry to a db.ExpressionRow for insertion.
func entryToExprRow(e *output.Entry, sourceLang, targetLang, tags string) *db.ExpressionRow {
	now := time.Now().UTC().Format(time.RFC3339)
	t := tags
	if e.Tags != "" {
		t = e.Tags
	}
	return &db.ExpressionRow{
		Expression:        e.Expression,
		Definition:        e.Definition,
		EnglishDefinition: e.EnglishDefinition,
		Example:           e.Example,
		English:           e.English,
		TargetTranslation: e.TargetTranslation,
		Notes:             e.Notes,
		Connotation:       e.Connotation,
		Register:          e.Register,
		ContrastiveNotes:  e.ContrastiveNotes,
		Tags:              t,
		SourceLanguage:    sourceLang,
		TargetLanguage:    targetLang,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// Lookup performs a single vocabulary lookup: normalize → cache check →
// build prompt → invoke LLM → validate → map fields → handle conflict or store.
func Lookup(ctx context.Context, store db.Store, params LookupParams) (*LookupResult, error) {
	m := mode(params.LookupType)

	// Normalize token
	var normalized string
	if m == "words" {
		normalized = parsing.NormalizeWord(params.Text)
	} else {
		normalized = parsing.NormalizeExpression(params.Text)
	}
	if normalized == "" {
		return nil, fmt.Errorf("empty token after normalization")
	}

	// Reject tokens containing digits or special characters (saves LLM credits)
	if m == "words" && !isValidToken(normalized) {
		return nil, fmt.Errorf("invalid input %q — words must contain only letters, spaces, hyphens, or parentheses", normalized)
	}

	// Dry-run: normalize and return without LLM or DB
	if params.DryRun {
		return &LookupResult{
			Entry: &output.Entry{Word: normalized},
		}, nil
	}

	// Apply timeout if set
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	sourceLang := language.ResolveLanguageName(params.SourceLang)
	targetLang := language.ResolveLanguageName(params.TargetLang)

	// Sentence lookups are ephemeral — always invoke LLM, never cache.
	if params.LookupType == "sentence" {
		slog.Info("sentence lookup (ephemeral)", slog.String("text", normalized), slog.String("source_lang", params.SourceLang))
		entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, m, normalized, params.Context, targetLang)
		if err != nil {
			return nil, err
		}
		entry.Tags = params.Tags
		return &LookupResult{Entry: entry}, nil
	}

	// Cache check
	if m == "words" {
		return lookupWord(ctx, store, params, m, normalized, sourceLang, targetLang)
	}
	return lookupExpression(ctx, store, params, m, normalized, sourceLang, targetLang)
}

// lookupWord handles the cache-check and LLM invocation flow for a single word lookup.
func lookupWord(ctx context.Context, store db.Store, params LookupParams, m, normalized, sourceLang, targetLang string) (*LookupResult, error) {
	existing, err := store.FindWords(ctx, normalized, params.SourceLang)
	if err != nil {
		slog.Error("database lookup failed", slog.String("word", normalized), slog.String("error", err.Error()))
		return nil, fmt.Errorf("database lookup failed: %w", err)
	}

	if len(existing) == 0 {
		// Cache miss — invoke LLM and insert
		slog.Info("cache miss", slog.String("word", normalized), slog.String("source_lang", params.SourceLang))
		entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, m, normalized, params.Context, targetLang)
		if err != nil {
			return nil, err
		}
		entry.Tags = params.Tags

		result := &LookupResult{Entry: entry}
		// Check quality before saving — skip DB insert for non-words
		if w := checkNonWord(normalized, entry); w != "" {
			result.Warning = w
			slog.Warn(w, slog.String("word", normalized))
			return result, nil
		}

		row := entryToWordRow(entry, params.SourceLang, params.TargetLang, params.Tags)
		row.Word = normalized // use normalized token for cache consistency
		if err := store.InsertWord(ctx, row); err != nil {
			slog.Error("database insert failed", slog.String("word", normalized), slog.String("error", err.Error()))
			return nil, fmt.Errorf("database insert failed: %w", err)
		}
		slog.Info("word processed", slog.String("word", normalized))
		if w := checkHallucination(normalized, entry.Example); w != "" {
			result.Warning = w
			slog.Warn(w, slog.String("word", normalized))
		}
		return result, nil
	}

	// Entries exist, no context → return first cached
	if params.Context == "" {
		slog.Info("cache hit", slog.String("word", normalized), slog.Int("versions", len(existing)))
		entry := wordRowToEntry(&existing[0])
		return &LookupResult{Entry: entry, FromCache: true}, nil
	}

	// Entries exist, context provided → cache bypass
	slog.Info("cache bypass (context provided)", slog.String("word", normalized), slog.Int("existing", len(existing)))
	entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, m, normalized, params.Context, targetLang)
	if err != nil {
		return nil, err
	}
	entry.Tags = params.Tags

	var existingEntries []output.Entry
	var existingIDs []int64
	for i := range existing {
		existingEntries = append(existingEntries, *wordRowToEntry(&existing[i]))
		existingIDs = append(existingIDs, existing[i].ID)
	}

	result := &LookupResult{
		Entry:           entry,
		Existing:        existingEntries,
		ExistingIDs:     existingIDs,
		NeedsResolution: true,
	}
	if w := checkQuality(normalized, entry); w != "" {
		result.Warning = w
		slog.Warn(w, slog.String("word", normalized))
	}

	// Auto-resolve if OnConflict is pre-set
	if params.OnConflict != "" {
		targetID := existingIDs[0]
		if params.ReplaceID != 0 {
			targetID = params.ReplaceID
		}
		// Ensure the entry uses the normalized token for DB consistency
		entry.Word = normalized
		if err := ResolveConflict(ctx, store, params.OnConflict, m, entry, targetID, params.SourceLang, params.TargetLang, params.Tags); err != nil {
			return nil, err
		}
		result.NeedsResolution = false
	}

	return result, nil
}

// lookupExpression handles the cache-check and LLM invocation flow for a single expression lookup.
func lookupExpression(ctx context.Context, store db.Store, params LookupParams, m, normalized, sourceLang, targetLang string) (*LookupResult, error) {
	existing, err := store.FindExpressions(ctx, normalized, params.SourceLang)
	if err != nil {
		slog.Error("database lookup failed", slog.String("expression", normalized), slog.String("error", err.Error()))
		return nil, fmt.Errorf("database lookup failed: %w", err)
	}

	if len(existing) == 0 {
		slog.Info("cache miss", slog.String("expression", normalized), slog.String("source_lang", params.SourceLang))
		entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, m, normalized, params.Context, targetLang)
		if err != nil {
			return nil, err
		}
		entry.Tags = params.Tags

		result := &LookupResult{Entry: entry}
		if w := checkNonWord(normalized, entry); w != "" {
			result.Warning = w
			slog.Warn(w, slog.String("expression", normalized))
			return result, nil
		}

		row := entryToExprRow(entry, params.SourceLang, params.TargetLang, params.Tags)
		row.Expression = normalized
		if err := store.InsertExpression(ctx, row); err != nil {
			slog.Error("database insert failed", slog.String("expression", normalized), slog.String("error", err.Error()))
			return nil, fmt.Errorf("database insert failed: %w", err)
		}
		slog.Info("expression processed", slog.String("expression", normalized))
		return result, nil
	}

	if params.Context == "" {
		slog.Info("cache hit", slog.String("expression", normalized), slog.Int("versions", len(existing)))
		entry := exprRowToEntry(&existing[0])
		return &LookupResult{Entry: entry, FromCache: true}, nil
	}

	slog.Info("cache bypass (context provided)", slog.String("expression", normalized), slog.Int("existing", len(existing)))
	entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, m, normalized, params.Context, targetLang)
	if err != nil {
		return nil, err
	}
	entry.Tags = params.Tags

	var existingEntries []output.Entry
	var existingIDs []int64
	for i := range existing {
		existingEntries = append(existingEntries, *exprRowToEntry(&existing[i]))
		existingIDs = append(existingIDs, existing[i].ID)
	}

	result := &LookupResult{
		Entry:           entry,
		Existing:        existingEntries,
		ExistingIDs:     existingIDs,
		NeedsResolution: true,
	}

	if params.OnConflict != "" {
		targetID := existingIDs[0]
		if params.ReplaceID != 0 {
			targetID = params.ReplaceID
		}
		// Ensure the entry uses the normalized token for DB consistency
		entry.Expression = normalized
		if err := ResolveConflict(ctx, store, params.OnConflict, m, entry, targetID, params.SourceLang, params.TargetLang, params.Tags); err != nil {
			return nil, err
		}
		result.NeedsResolution = false
	}

	return result, nil
}

// ResolveConflict applies a conflict resolution strategy after a cache-bypass lookup.
func ResolveConflict(ctx context.Context, store db.Store, strategy ConflictStrategy, m string, entry *output.Entry, targetID int64, sourceLang, targetLang, tags string) error {
	slog.Info("resolving conflict", slog.String("strategy", string(strategy)), slog.String("mode", m), slog.Int64("target_id", targetID))
	switch strategy {
	case ConflictReplace:
		if m == "words" {
			row := entryToWordRow(entry, sourceLang, targetLang, tags)
			return store.UpdateWord(ctx, targetID, row)
		}
		row := entryToExprRow(entry, sourceLang, targetLang, tags)
		return store.UpdateExpression(ctx, targetID, row)

	case ConflictAdd:
		if m == "words" {
			row := entryToWordRow(entry, sourceLang, targetLang, tags)
			return store.InsertWord(ctx, row)
		}
		row := entryToExprRow(entry, sourceLang, targetLang, tags)
		return store.InsertExpression(ctx, row)

	case ConflictSkip:
		return nil

	default:
		return fmt.Errorf("unknown conflict strategy: %q", strategy)
	}
}

// ProgressFunc is called after each item during batch processing.
// current is the 1-based index of the item just processed, total is the
// total number of items, token is the raw input token, and status describes
// the outcome (e.g. "processed", "cached", "failed", "skipped").
type ProgressFunc func(current, total int, token, status string)

// BatchParams holds all parameters for batch processing.
type BatchParams struct {
	SourceLang string
	Mode       string // "words" or "expressions"
	Tokens     []parsing.TokenWithContext
	Provider   llm.Provider
	ModelID    string
	TargetLang string
	Tags       string
	Limit      int
	DryRun     bool
	Timeout    time.Duration
	OnConflict ConflictStrategy // default: "skip"
	OnProgress ProgressFunc     // optional progress callback
}

// BatchResult holds the outcome of batch processing.
type BatchResult struct {
	Results   []output.Entry
	Errors    []BatchError
	Processed int
	Cached    int
	Failed    int
	Skipped   int // empty tokens after normalization
	Replaced  int
	Added     int
}

// BatchError pairs a token with its error message.
type BatchError struct {
	Token   string
	Message string
}

// reportProgress calls the progress callback if set.
func reportProgress(fn ProgressFunc, current, total int, token, status string) {
	if fn != nil {
		fn(current, total, token, status)
	}
}

// ProcessBatch processes a list of tokens through the LLM with caching.
func ProcessBatch(ctx context.Context, store db.Store, params BatchParams) (*BatchResult, error) {
	if params.OnConflict == "" {
		params.OnConflict = ConflictSkip
	}

	sourceLang := language.ResolveLanguageName(params.SourceLang)
	targetLang := language.ResolveLanguageName(params.TargetLang)

	result := &BatchResult{}
	newCount := 0 // tracks items counting toward limit
	total := len(params.Tokens)

	for i, tc := range params.Tokens {
		// Check for context cancellation (e.g. client disconnect)
		if ctx.Err() != nil {
			break
		}

		// Normalize
		var normalized string
		if params.Mode == "words" {
			normalized = parsing.NormalizeWord(tc.Token)
		} else {
			normalized = parsing.NormalizeExpression(tc.Token)
		}
		if normalized == "" {
			result.Skipped++
			reportProgress(params.OnProgress, i+1, total, tc.Token, "skipped")
			continue
		}

		// Dry-run: just count, no LLM or DB
		if params.DryRun {
			result.Processed++
			reportProgress(params.OnProgress, i+1, total, normalized, "processed")
			continue
		}

		// Check limit before doing new work
		if params.Limit > 0 && newCount >= params.Limit {
			break
		}

		// Snapshot counters to determine outcome after processing
		prevProcessed := result.Processed
		prevCached := result.Cached
		prevFailed := result.Failed
		prevReplaced := result.Replaced
		prevAdded := result.Added
		prevSkipped := result.Skipped

		// Cache check + process
		if params.Mode == "words" {
			processBatchWord(ctx, store, params, sourceLang, targetLang, normalized, tc.Context, result, &newCount)
		} else {
			processBatchExpression(ctx, store, params, sourceLang, targetLang, normalized, tc.Context, result, &newCount)
		}

		// Determine status from counter changes
		status := "processed"
		switch {
		case result.Cached > prevCached:
			status = "cached"
		case result.Failed > prevFailed:
			status = "failed"
		case result.Replaced > prevReplaced:
			status = "replaced"
		case result.Added > prevAdded:
			status = "added"
		case result.Skipped > prevSkipped:
			status = "skipped"
		case result.Processed > prevProcessed:
			status = "processed"
		}
		reportProgress(params.OnProgress, i+1, total, normalized, status)
	}

	return result, nil
}

// processBatchWord processes a single word within a batch: checks the cache,
// invokes the LLM on miss, and applies the configured conflict strategy.
func processBatchWord(ctx context.Context, store db.Store, params BatchParams, sourceLang, targetLang, normalized, ctxSentence string, result *BatchResult, newCount *int) {
	if ctx.Err() != nil {
		return
	}

	existing, err := store.FindWords(ctx, normalized, params.SourceLang)
	if err != nil {
		slog.Error("batch: database lookup failed", slog.String("word", normalized), slog.String("error", err.Error()))
		result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
		result.Failed++
		return
	}

	// Existing entries, no context → cache hit
	if len(existing) > 0 && ctxSentence == "" {
		slog.Debug("batch: cache hit", slog.String("word", normalized))
		result.Cached++
		result.Results = append(result.Results, *wordRowToEntry(&existing[0]))
		return
	}

	// Check limit before LLM invocation
	if params.Limit > 0 && *newCount >= params.Limit {
		return
	}

	// Invoke LLM
	entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, params.Mode, normalized, ctxSentence, targetLang)
	if err != nil {
		slog.Error("batch: LLM failed", slog.String("word", normalized), slog.String("error", err.Error()))
		result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
		result.Failed++
		*newCount++
		return
	}
	entry.Tags = params.Tags

	if len(existing) > 0 {
		// Context provided + existing entries → apply conflict strategy
		switch params.OnConflict {
		case ConflictReplace:
			row := entryToWordRow(entry, params.SourceLang, params.TargetLang, params.Tags)
			row.Word = normalized
			if err := store.UpdateWord(ctx, existing[0].ID, row); err != nil {
				result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
				result.Failed++
			} else {
				result.Replaced++
				result.Results = append(result.Results, *entry)
			}
		case ConflictAdd:
			row := entryToWordRow(entry, params.SourceLang, params.TargetLang, params.Tags)
			row.Word = normalized
			if err := store.InsertWord(ctx, row); err != nil {
				result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
				result.Failed++
			} else {
				result.Added++
				result.Results = append(result.Results, *entry)
			}
		case ConflictSkip:
			result.Skipped++
		}
	} else {
		// No existing entries → insert
		row := entryToWordRow(entry, params.SourceLang, params.TargetLang, params.Tags)
		row.Word = normalized // use normalized token for cache consistency
		if err := store.InsertWord(ctx, row); err != nil {
			result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
			result.Failed++
		} else {
			result.Processed++
			result.Results = append(result.Results, *entry)
		}
	}
	*newCount++
}

// processBatchExpression processes a single expression within a batch: checks the cache,
// invokes the LLM on miss, and applies the configured conflict strategy.
func processBatchExpression(ctx context.Context, store db.Store, params BatchParams, sourceLang, targetLang, normalized, ctxSentence string, result *BatchResult, newCount *int) {
	if ctx.Err() != nil {
		return
	}

	existing, err := store.FindExpressions(ctx, normalized, params.SourceLang)
	if err != nil {
		slog.Error("batch: database lookup failed", slog.String("expression", normalized), slog.String("error", err.Error()))
		result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
		result.Failed++
		return
	}

	if len(existing) > 0 && ctxSentence == "" {
		slog.Debug("batch: cache hit", slog.String("expression", normalized))
		result.Cached++
		result.Results = append(result.Results, *exprRowToEntry(&existing[0]))
		return
	}

	if params.Limit > 0 && *newCount >= params.Limit {
		return
	}

	entry, err := invokeLLM(ctx, params.Provider, params.ModelID, sourceLang, params.Mode, normalized, ctxSentence, targetLang)
	if err != nil {
		slog.Error("batch: LLM failed", slog.String("expression", normalized), slog.String("error", err.Error()))
		result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
		result.Failed++
		*newCount++
		return
	}
	entry.Tags = params.Tags

	if len(existing) > 0 {
		switch params.OnConflict {
		case ConflictReplace:
			row := entryToExprRow(entry, params.SourceLang, params.TargetLang, params.Tags)
			row.Expression = normalized
			if err := store.UpdateExpression(ctx, existing[0].ID, row); err != nil {
				result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
				result.Failed++
			} else {
				result.Replaced++
				result.Results = append(result.Results, *entry)
			}
		case ConflictAdd:
			row := entryToExprRow(entry, params.SourceLang, params.TargetLang, params.Tags)
			row.Expression = normalized
			if err := store.InsertExpression(ctx, row); err != nil {
				result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
				result.Failed++
			} else {
				result.Added++
				result.Results = append(result.Results, *entry)
			}
		case ConflictSkip:
			result.Skipped++
		}
	} else {
		row := entryToExprRow(entry, params.SourceLang, params.TargetLang, params.Tags)
		row.Expression = normalized // use normalized token for cache consistency
		if err := store.InsertExpression(ctx, row); err != nil {
			result.Errors = append(result.Errors, BatchError{Token: normalized, Message: err.Error()})
			result.Failed++
		} else {
			result.Processed++
			result.Results = append(result.Results, *entry)
		}
	}
	*newCount++
}

// LanguageInfo holds a language code and its full name.
type LanguageInfo struct {
	Code string
	Name string
}

// GetSupportedLanguages returns the language registry as a sorted slice.
func GetSupportedLanguages() []LanguageInfo {
	var langs []LanguageInfo
	for code, name := range language.SupportedLanguages {
		langs = append(langs, LanguageInfo{Code: code, Name: name})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Name < langs[j].Name
	})
	return langs
}
