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

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	tokens, err := readCSVFromReader(file)
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

	apiKey := r.FormValue("api_key")
	provider, err := s.createProvider(apiKey)
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
		renderPartial(w, "batch_summary", map[string]any{"Error": "Upload exceeds 10 MB limit"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		renderPartial(w, "batch_summary", map[string]any{"Error": "CSV file is required"})
		return
	}
	defer file.Close()

	tokens, err := readCSVFromReader(file)
	if err != nil {
		renderPartial(w, "batch_summary", map[string]any{"Error": err.Error()})
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
		renderPartial(w, "batch_summary", map[string]any{"Error": err.Error()})
		return
	}

	apiKey := r.FormValue("api_key")
	provider, err := s.createProvider(apiKey)
	if err != nil {
		renderPartial(w, "batch_summary", map[string]any{"Error": "Provider error: " + err.Error()})
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
		renderPartial(w, "batch_summary", map[string]any{"Error": err.Error()})
		return
	}

	renderPartial(w, "batch_summary", map[string]any{
		"Processed": result.Processed,
		"Cached":    result.Cached,
		"Failed":    result.Failed,
		"Replaced":  result.Replaced,
		"Added":     result.Added,
		"Errors":    result.Errors,
	})
}

// handleBatchStream handles GET /api/batch/stream — SSE endpoint for batch progress.
func (s *Server) handleBatchStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send an initial event to confirm connection
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	// SSE endpoint waits for batch processing to be triggered separately.
	// For now, send a complete event since batch processing is synchronous.
	data, _ := json.Marshal(map[string]any{
		"status":  "complete",
		"message": "Use POST /api/batch/html for batch processing",
	})
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
	flusher.Flush()
}
