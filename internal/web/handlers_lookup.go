package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/user/vocabgen/internal/output"
	"github.com/user/vocabgen/internal/service"
)

// lookupRequest is the JSON body for POST /api/lookup.
type lookupRequest struct {
	SourceLanguage string `json:"source_language"`
	LookupType     string `json:"lookup_type"`
	Text           string `json:"text"`
	Context        string `json:"context"`
	TargetLanguage string `json:"target_language"`
	Tags           string `json:"tags"`
	APIKey         string `json:"api_key"`
}

// resolveRequest is the JSON body for POST /api/lookup/resolve.
type resolveRequest struct {
	Strategy       string        `json:"strategy"`
	TargetID       int64         `json:"target_id"`
	Entry          *output.Entry `json:"entry"`
	Mode           string        `json:"mode"`
	SourceLanguage string        `json:"source_language"`
	TargetLanguage string        `json:"target_language"`
	Tags           string        `json:"tags"`
}

func (s *Server) parseLookupParams(r *http.Request) (service.LookupParams, string, error) {
	var req lookupRequest

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return service.LookupParams{}, "", fmt.Errorf("invalid JSON: %w", err)
		}
	} else {
		// Form-encoded (HTMX)
		if err := r.ParseForm(); err != nil {
			return service.LookupParams{}, "", fmt.Errorf("invalid form: %w", err)
		}
		req = lookupRequest{
			SourceLanguage: r.FormValue("source_language"),
			LookupType:     r.FormValue("lookup_type"),
			Text:           r.FormValue("text"),
			Context:        r.FormValue("context"),
			TargetLanguage: r.FormValue("target_language"),
			Tags:           r.FormValue("tags"),
			APIKey:         r.FormValue("api_key"),
		}
	}

	if req.Text == "" {
		return service.LookupParams{}, "", fmt.Errorf("text is required")
	}
	if req.SourceLanguage == "" {
		req.SourceLanguage = s.cfg.DefaultSourceLanguage
	}
	if req.TargetLanguage == "" {
		req.TargetLanguage = s.cfg.DefaultTargetLanguage
	}
	if req.LookupType == "" {
		req.LookupType = "word"
	}

	provider, err := s.createProvider(req.APIKey)
	if err != nil {
		return service.LookupParams{}, "", fmt.Errorf("provider error: %w", err)
	}

	return service.LookupParams{
		SourceLang: req.SourceLanguage,
		LookupType: req.LookupType,
		Text:       req.Text,
		Provider:   provider,
		ModelID:    s.cfg.ModelID,
		Context:    req.Context,
		TargetLang: req.TargetLanguage,
		Tags:       req.Tags,
	}, req.APIKey, nil
}

// handleLookupJSON handles POST /api/lookup — JSON endpoint.
func (s *Server) handleLookupJSON(w http.ResponseWriter, r *http.Request) {
	params, _, err := s.parseLookupParams(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := service.Lookup(r.Context(), s.store, params)
	if err != nil {
		s.logger.Error("lookup failed", "error", err)
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleLookupHTML handles POST /api/lookup/html — HTMX endpoint.
func (s *Server) handleLookupHTML(w http.ResponseWriter, r *http.Request) {
	params, _, err := s.parseLookupParams(r)
	if err != nil {
		renderPartial(w, "lookup_result", map[string]any{"Error": err.Error()})
		return
	}

	result, err := service.Lookup(r.Context(), s.store, params)
	if err != nil {
		s.logger.Error("lookup failed", "error", err)
		renderPartial(w, "lookup_result", map[string]any{"Error": err.Error()})
		return
	}

	if result.NeedsResolution {
		entryJSON, _ := json.Marshal(result.Entry)
		mode := "words"
		if params.LookupType == "expression" || params.LookupType == "sentence" {
			mode = "expressions"
		}
		data := map[string]any{
			"Existing":    result.Existing,
			"ExistingIDs": result.ExistingIDs,
			"NewEntry":    result.Entry,
			"EntryJSON":   string(entryJSON),
			"Mode":        mode,
			"SourceLang":  params.SourceLang,
			"TargetLang":  params.TargetLang,
			"Tags":        params.Tags,
		}
		renderPartial(w, "lookup_conflict", data)
		return
	}

	renderPartial(w, "lookup_result", map[string]any{
		"Entry":     result.Entry,
		"FromCache": result.FromCache,
		"Warning":   result.Warning,
	})
}

// handleLookupResolveJSON handles POST /api/lookup/resolve — JSON endpoint.
func (s *Server) handleLookupResolveJSON(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	strategy, err := service.ParseConflictStrategy(req.Strategy)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Entry == nil {
		writeJSONError(w, http.StatusBadRequest, "entry is required")
		return
	}

	if err := service.ResolveConflict(r.Context(), s.store, strategy, req.Mode, req.Entry, req.TargetID, req.SourceLanguage, req.TargetLanguage, req.Tags); err != nil {
		s.logger.Error("resolve conflict failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"entry": req.Entry, "strategy": req.Strategy})
}

// handleLookupResolveHTML handles POST /api/lookup/resolve/html — HTMX endpoint.
func (s *Server) handleLookupResolveHTML(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPartial(w, "lookup_result", map[string]any{"Error": "invalid form"})
		return
	}

	strategyStr := r.FormValue("strategy")
	strategy, err := service.ParseConflictStrategy(strategyStr)
	if err != nil {
		renderPartial(w, "lookup_result", map[string]any{"Error": err.Error()})
		return
	}

	var entry output.Entry
	entryJSON := r.FormValue("entry")
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		renderPartial(w, "lookup_result", map[string]any{"Error": "invalid entry data"})
		return
	}

	var targetID int64
	fmt.Sscanf(r.FormValue("target_id"), "%d", &targetID)

	mode := r.FormValue("mode")
	sourceLang := r.FormValue("source_language")
	targetLang := r.FormValue("target_language")
	tags := r.FormValue("tags")

	if err := service.ResolveConflict(r.Context(), s.store, strategy, mode, &entry, targetID, sourceLang, targetLang, tags); err != nil {
		s.logger.Error("resolve conflict failed", "error", err)
		renderPartial(w, "lookup_result", map[string]any{"Error": err.Error()})
		return
	}

	renderPartial(w, "lookup_result", map[string]any{"Entry": &entry})
}
