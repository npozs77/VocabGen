package web

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/user/vocabgen/internal/parsing"
	"github.com/user/vocabgen/internal/service"
)

const maxUploadSize = 10 << 20 // 10 MB

// parseTextList splits a plain-text word list (one token per line) into
// []parsing.TokenWithContext. Empty/whitespace-only lines are skipped.
// Each line may optionally contain a comma-separated context sentence:
// "token, context sentence". Returns an error if the input is empty after
// trimming blank lines.
func parseTextList(text string) ([]parsing.TokenWithContext, error) {
	var results []parsing.TokenWithContext
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		tc := parsing.TokenWithContext{}
		if idx := strings.IndexByte(line, ','); idx >= 0 {
			tc.Token = strings.TrimSpace(line[:idx])
			tc.Context = strings.TrimSpace(line[idx+1:])
		} else {
			tc.Token = line
		}
		if tc.Token == "" {
			continue
		}
		results = append(results, tc)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("word list is empty")
	}
	return results, nil
}

// parseBatchInput extracts tokens from either the word_list form field or the
// uploaded CSV file. Returns the tokens and an optional file closer (nil when
// word_list was used). The caller must close the file if non-nil.
func parseBatchInput(r *http.Request) ([]parsing.TokenWithContext, io.Closer, error) {
	if wl := r.FormValue("word_list"); strings.TrimSpace(wl) != "" {
		tokens, err := parseTextList(wl)
		return tokens, nil, err
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, nil, fmt.Errorf("CSV file or word list is required")
	}
	tokens, err := readCSVFromReader(file)
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return tokens, file, nil
}

// readCSVFromReader parses CSV tokens from an io.Reader.
func readCSVFromReader(r io.Reader) ([]parsing.TokenWithContext, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	var results []parsing.TokenWithContext
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV read error: %w", err)
		}
		if len(record) == 0 {
			continue
		}
		token := strings.TrimSpace(record[0])
		if token == "" {
			continue
		}
		tc := parsing.TokenWithContext{Token: token}
		if len(record) >= 2 {
			tc.Context = strings.TrimSpace(record[1])
		}
		results = append(results, tc)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}
	return results, nil
}

// handleBatchJSON handles POST /api/batch — JSON batch processing.
func (s *Server) handleBatchJSON(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "upload exceeds 10 MB limit")
		return
	}

	tokens, closer, err := parseBatchInput(r)
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	sourceLang := r.FormValue("source_language")
	if sourceLang == "" {
		sourceLang = s.cfg.DefaultSourceLanguage
	}
	targetLang := r.FormValue("target_language")
	if targetLang == "" {
		targetLang = s.cfg.DefaultTargetLanguage
	}
	mode := r.FormValue("mode")
	if mode == "" {
		mode = "words"
	}
	tags := r.FormValue("tags")
	onConflictStr := r.FormValue("on_conflict")
	if onConflictStr == "" {
		onConflictStr = "skip"
	}
	onConflict, err := service.ParseConflictStrategy(onConflictStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	provider, err := s.createProvider()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "provider error: "+err.Error())
		return
	}

	result, err := service.ProcessBatch(r.Context(), s.store, service.BatchParams{
		SourceLang: sourceLang,
		Mode:       mode,
		Tokens:     tokens,
		Provider:   provider,
		ModelID:    s.cfg.ModelID,
		TargetLang: targetLang,
		Tags:       tags,
		OnConflict: onConflict,
	})
	if err != nil {
		s.logger.Error("batch processing failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleBatchHTML handles POST /api/batch/html — HTMX multipart upload.
func (s *Server) handleBatchHTML(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		_ = renderPartial(w, "batch_summary", map[string]any{"Error": "Upload exceeds 10 MB limit"})
		return
	}

	tokens, closer, err := parseBatchInput(r)
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	if err != nil {
		_ = renderPartial(w, "batch_summary", map[string]any{"Error": err.Error()})
		return
	}

	sourceLang := r.FormValue("source_language")
	if sourceLang == "" {
		sourceLang = s.cfg.DefaultSourceLanguage
	}
	targetLang := r.FormValue("target_language")
	if targetLang == "" {
		targetLang = s.cfg.DefaultTargetLanguage
	}
	mode := r.FormValue("mode")
	if mode == "" {
		mode = "words"
	}
	tags := r.FormValue("tags")
	onConflictStr := r.FormValue("on_conflict")
	if onConflictStr == "" {
		onConflictStr = "skip"
	}
	onConflict, err := service.ParseConflictStrategy(onConflictStr)
	if err != nil {
		_ = renderPartial(w, "batch_summary", map[string]any{"Error": err.Error()})
		return
	}

	provider, err := s.createProvider()
	if err != nil {
		_ = renderPartial(w, "batch_summary", map[string]any{"Error": "Provider error: " + err.Error()})
		return
	}

	result, err := service.ProcessBatch(r.Context(), s.store, service.BatchParams{
		SourceLang: sourceLang,
		Mode:       mode,
		Tokens:     tokens,
		Provider:   provider,
		ModelID:    s.cfg.ModelID,
		TargetLang: targetLang,
		Tags:       tags,
		OnConflict: onConflict,
	})
	if err != nil {
		s.logger.Error("batch processing failed", "error", err)
		_ = renderPartial(w, "batch_summary", map[string]any{"Error": err.Error()})
		return
	}

	_ = renderPartial(w, "batch_summary", map[string]any{
		"Processed": result.Processed,
		"Cached":    result.Cached,
		"Failed":    result.Failed,
		"Replaced":  result.Replaced,
		"Added":     result.Added,
		"Errors":    result.Errors,
	})
}

// handleBatchStream handles POST /api/batch/stream — SSE endpoint for batch progress.
func (s *Server) handleBatchStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "event: error\ndata: {\"message\":\"Upload exceeds 10 MB limit\"}\n\n")
		flusher.Flush()
		return
	}

	tokens, closer, err := parseBatchInput(r)
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	if err != nil {
		w.Header().Set("Content-Type", "text/event-stream")
		data, _ := json.Marshal(map[string]string{"message": err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	sourceLang := r.FormValue("source_language")
	if sourceLang == "" {
		sourceLang = s.cfg.DefaultSourceLanguage
	}
	targetLang := r.FormValue("target_language")
	if targetLang == "" {
		targetLang = s.cfg.DefaultTargetLanguage
	}
	mode := r.FormValue("mode")
	if mode == "" {
		mode = "words"
	}
	tags := r.FormValue("tags")
	onConflictStr := r.FormValue("on_conflict")
	if onConflictStr == "" {
		onConflictStr = "skip"
	}
	onConflict, err := service.ParseConflictStrategy(onConflictStr)
	if err != nil {
		w.Header().Set("Content-Type", "text/event-stream")
		data, _ := json.Marshal(map[string]string{"message": err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	provider, err := s.createProvider()
	if err != nil {
		w.Header().Set("Content-Type", "text/event-stream")
		data, _ := json.Marshal(map[string]string{"message": "Provider error: " + err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial connected event
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\",\"total\":%d}\n\n", len(tokens))
	flusher.Flush()

	// Progress callback streams SSE events per item
	progressFn := func(current, total int, token, status string) {
		data, _ := json.Marshal(map[string]any{
			"current": current,
			"total":   total,
			"token":   token,
			"status":  status,
		})
		_, _ = fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()
	}

	result, err := service.ProcessBatch(r.Context(), s.store, service.BatchParams{
		SourceLang: sourceLang,
		Mode:       mode,
		Tokens:     tokens,
		Provider:   provider,
		ModelID:    s.cfg.ModelID,
		TargetLang: targetLang,
		Tags:       tags,
		OnConflict: onConflict,
		OnProgress: progressFn,
	})
	if err != nil {
		s.logger.Error("batch stream processing failed", "error", err)
		data, _ := json.Marshal(map[string]string{"message": err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Check if processing was cancelled (client disconnected or abort)
	if r.Context().Err() != nil {
		data, _ := json.Marshal(map[string]any{
			"processed": result.Processed,
			"cached":    result.Cached,
			"failed":    result.Failed,
			"skipped":   result.Skipped,
			"replaced":  result.Replaced,
			"added":     result.Added,
			"errors":    result.Errors,
		})
		_, _ = fmt.Fprintf(w, "event: cancelled\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Send final complete event with summary
	data, _ := json.Marshal(map[string]any{
		"processed": result.Processed,
		"cached":    result.Cached,
		"failed":    result.Failed,
		"skipped":   result.Skipped,
		"replaced":  result.Replaced,
		"added":     result.Added,
		"errors":    result.Errors,
	})
	_, _ = fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
	flusher.Flush()
}
