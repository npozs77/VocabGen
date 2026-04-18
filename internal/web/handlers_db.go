package web

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/output"
	"github.com/xuri/excelize/v2"
)

// parseListParams extracts pagination and filter parameters from the request URL query string.
func parseListParams(r *http.Request) (db.ListFilter, error) {
	filter := db.ListFilter{
		SourceLang: r.URL.Query().Get("source_lang"),
		TargetLang: r.URL.Query().Get("target_lang"),
		Search:     r.URL.Query().Get("search"),
		Tags:       r.URL.Query().Get("tags"),
		Page:       1,
		PageSize:   20,
	}
	if p := r.URL.Query().Get("page"); p != "" {
		page, err := strconv.Atoi(p)
		if err != nil || page < 1 {
			return filter, fmt.Errorf("invalid page: %s", p)
		}
		filter.Page = page
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		pageSize, err := strconv.Atoi(ps)
		if err != nil || pageSize < 1 || pageSize > 200 {
			return filter, fmt.Errorf("invalid page_size: %s", ps)
		}
		filter.PageSize = pageSize
	}
	return filter, nil
}

// handleListWords handles GET /api/words — paginated word list.
func (s *Server) handleListWords(w http.ResponseWriter, r *http.Request) {
	// If type=expressions, delegate to the expressions handler.
	if r.URL.Query().Get("type") == "expressions" {
		s.handleListExpressions(w, r)
		return
	}

	filter, err := parseListParams(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	words, total, err := s.store.ListWords(r.Context(), filter)
	if err != nil {
		s.logger.Error("list words failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check if HTMX request — return partial
	if r.Header.Get("HX-Request") == "true" {
		totalPages := (total + filter.PageSize - 1) / filter.PageSize
		if totalPages < 1 {
			totalPages = 1
		}
		data := map[string]any{
			"Words":      words,
			"Total":      total,
			"Page":       filter.Page,
			"PageSize":   filter.PageSize,
			"TotalPages": totalPages,
			"PrevPage":   filter.Page - 1,
			"NextPage":   filter.Page + 1,
			"IsWords":    true,
			"BaseURL":    "/api/words",
			"SourceLang": filter.SourceLang,
			"TargetLang": filter.TargetLang,
			"Search":     filter.Search,
			"Tags":       filter.Tags,
		}
		_ = renderPartial(w, "entry_table", data)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"words": words,
		"total": total,
		"page":  filter.Page,
	})
}

// handleListExpressions handles GET /api/expressions — paginated expression list.
func (s *Server) handleListExpressions(w http.ResponseWriter, r *http.Request) {
	filter, err := parseListParams(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	expressions, total, err := s.store.ListExpressions(r.Context(), filter)
	if err != nil {
		s.logger.Error("list expressions failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		totalPages := (total + filter.PageSize - 1) / filter.PageSize
		if totalPages < 1 {
			totalPages = 1
		}
		data := map[string]any{
			"Expressions": expressions,
			"Total":       total,
			"Page":        filter.Page,
			"PageSize":    filter.PageSize,
			"TotalPages":  totalPages,
			"PrevPage":    filter.Page - 1,
			"NextPage":    filter.Page + 1,
			"IsWords":     false,
			"BaseURL":     "/api/words?type=expressions",
			"SourceLang":  filter.SourceLang,
			"TargetLang":  filter.TargetLang,
			"Search":      filter.Search,
			"Tags":        filter.Tags,
		}
		_ = renderPartial(w, "entry_table", data)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"expressions": expressions,
		"total":       total,
		"page":        filter.Page,
	})
}

// handleUpdateWord handles PUT /api/words/{id}.
func (s *Server) handleUpdateWord(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	// Load existing row so we preserve fields not in the form.
	existing, err := s.store.GetWord(r.Context(), id)
	if err != nil || existing == nil {
		writeJSONError(w, http.StatusNotFound, "word not found")
		return
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := decodeJSON(r, existing); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form")
			return
		}
		// Only overwrite fields that are present in the form.
		if v := r.FormValue("word"); v != "" || r.Form.Has("word") {
			existing.Word = v
		}
		if v := r.FormValue("part_of_speech"); v != "" || r.Form.Has("part_of_speech") {
			existing.PartOfSpeech = v
		}
		if v := r.FormValue("article"); v != "" || r.Form.Has("article") {
			existing.Article = v
		}
		if v := r.FormValue("definition"); v != "" || r.Form.Has("definition") {
			existing.Definition = v
		}
		if v := r.FormValue("english_definition"); v != "" || r.Form.Has("english_definition") {
			existing.EnglishDefinition = v
		}
		if v := r.FormValue("example"); v != "" || r.Form.Has("example") {
			existing.Example = v
		}
		if v := r.FormValue("english"); v != "" || r.Form.Has("english") {
			existing.English = v
		}
		if v := r.FormValue("target_translation"); v != "" || r.Form.Has("target_translation") {
			existing.TargetTranslation = v
		}
		if v := r.FormValue("notes"); v != "" || r.Form.Has("notes") {
			existing.Notes = v
		}
		if v := r.FormValue("connotation"); v != "" || r.Form.Has("connotation") {
			existing.Connotation = v
		}
		if v := r.FormValue("register"); v != "" || r.Form.Has("register") {
			existing.Register = v
		}
		if v := r.FormValue("collocations"); v != "" || r.Form.Has("collocations") {
			existing.Collocations = v
		}
		if v := r.FormValue("contrastive_notes"); v != "" || r.Form.Has("contrastive_notes") {
			existing.ContrastiveNotes = v
		}
		if v := r.FormValue("secondary_meanings"); v != "" || r.Form.Has("secondary_meanings") {
			existing.SecondaryMeanings = v
		}
		if r.Form.Has("tags") {
			existing.Tags = r.FormValue("tags")
		}
		if v := r.FormValue("source_language"); v != "" {
			existing.SourceLanguage = v
		}
		if v := r.FormValue("target_language"); v != "" {
			existing.TargetLanguage = v
		}
	}
	existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.store.UpdateWord(r.Context(), id, existing); err != nil {
		s.logger.Error("update word failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		s.handleListWords(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleEditWord handles GET /api/words/{id}/edit — renders the inline edit form partial.
func (s *Server) handleEditWord(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	word, err := s.store.GetWord(r.Context(), id)
	if err != nil || word == nil {
		writeJSONError(w, http.StatusNotFound, "word not found")
		return
	}

	if err := renderPartial(w, "entry_edit", word); err != nil {
		s.logger.Error("render entry_edit failed", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// handleEditExpression handles GET /api/expressions/{id}/edit — renders the inline edit form partial.
func (s *Server) handleEditExpression(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	expr, err := s.store.GetExpression(r.Context(), id)
	if err != nil || expr == nil {
		writeJSONError(w, http.StatusNotFound, "expression not found")
		return
	}

	// Convert to map so template can check {{if .Word}} without error
	data := map[string]any{
		"ID":                expr.ID,
		"Word":              "", // empty → template takes the expression branch
		"Expression":        expr.Expression,
		"Definition":        expr.Definition,
		"EnglishDefinition": expr.EnglishDefinition,
		"Example":           expr.Example,
		"English":           expr.English,
		"TargetTranslation": expr.TargetTranslation,
		"Notes":             expr.Notes,
		"Connotation":       expr.Connotation,
		"Register":          expr.Register,
		"ContrastiveNotes":  expr.ContrastiveNotes,
		"Tags":              expr.Tags,
		"SourceLanguage":    expr.SourceLanguage,
		"TargetLanguage":    expr.TargetLanguage,
	}

	if err := renderPartial(w, "entry_edit", data); err != nil {
		s.logger.Error("render entry_edit failed", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// handleUpdateExpression handles PUT /api/expressions/{id}.
func (s *Server) handleUpdateExpression(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	existing, err := s.store.GetExpression(r.Context(), id)
	if err != nil || existing == nil {
		writeJSONError(w, http.StatusNotFound, "expression not found")
		return
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := decodeJSON(r, existing); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form")
			return
		}
		if v := r.FormValue("expression"); v != "" || r.Form.Has("expression") {
			existing.Expression = v
		}
		if v := r.FormValue("definition"); v != "" || r.Form.Has("definition") {
			existing.Definition = v
		}
		if v := r.FormValue("english_definition"); v != "" || r.Form.Has("english_definition") {
			existing.EnglishDefinition = v
		}
		if v := r.FormValue("example"); v != "" || r.Form.Has("example") {
			existing.Example = v
		}
		if v := r.FormValue("english"); v != "" || r.Form.Has("english") {
			existing.English = v
		}
		if v := r.FormValue("target_translation"); v != "" || r.Form.Has("target_translation") {
			existing.TargetTranslation = v
		}
		if v := r.FormValue("notes"); v != "" || r.Form.Has("notes") {
			existing.Notes = v
		}
		if v := r.FormValue("connotation"); v != "" || r.Form.Has("connotation") {
			existing.Connotation = v
		}
		if v := r.FormValue("register"); v != "" || r.Form.Has("register") {
			existing.Register = v
		}
		if v := r.FormValue("contrastive_notes"); v != "" || r.Form.Has("contrastive_notes") {
			existing.ContrastiveNotes = v
		}
		if r.Form.Has("tags") {
			existing.Tags = r.FormValue("tags")
		}
		if v := r.FormValue("source_language"); v != "" {
			existing.SourceLanguage = v
		}
		if v := r.FormValue("target_language"); v != "" {
			existing.TargetLanguage = v
		}
	}
	existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.store.UpdateExpression(r.Context(), id, existing); err != nil {
		s.logger.Error("update expression failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		s.handleListExpressions(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleDeleteWord handles DELETE /api/words/{id}.
func (s *Server) handleDeleteWord(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.store.DeleteWord(r.Context(), id); err != nil {
		s.logger.Error("delete word failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		s.handleListWords(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleDeleteExpression handles DELETE /api/expressions/{id}.
func (s *Server) handleDeleteExpression(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.store.DeleteExpression(r.Context(), id); err != nil {
		s.logger.Error("delete expression failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		s.handleListExpressions(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// bulkDeleteRequest is the JSON body for bulk delete endpoints.
type bulkDeleteRequest struct {
	IDs []int64 `json:"ids"`
}

// handleBulkDeleteWords handles DELETE /api/words/bulk.
func (s *Server) handleBulkDeleteWords(w http.ResponseWriter, r *http.Request) {
	var req bulkDeleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "ids must not be empty")
		return
	}

	if err := s.store.DeleteWords(r.Context(), req.IDs); err != nil {
		s.logger.Error("bulk delete words failed", "count", len(req.IDs), "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("bulk deleted words", "count", len(req.IDs))
	if r.Header.Get("HX-Request") == "true" {
		s.handleListWords(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "count": strconv.Itoa(len(req.IDs))})
}

// handleBulkDeleteExpressions handles DELETE /api/expressions/bulk.
func (s *Server) handleBulkDeleteExpressions(w http.ResponseWriter, r *http.Request) {
	var req bulkDeleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "ids must not be empty")
		return
	}

	if err := s.store.DeleteExpressions(r.Context(), req.IDs); err != nil {
		s.logger.Error("bulk delete expressions failed", "count", len(req.IDs), "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("bulk deleted expressions", "count", len(req.IDs))
	if r.Header.Get("HX-Request") == "true" {
		s.handleListExpressions(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "count": strconv.Itoa(len(req.IDs))})
}

// handleImport handles POST /api/import — CSV import.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "upload exceeds 10 MB limit")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read file: "+err.Error())
		return
	}

	sourceLang := r.FormValue("source_lang")
	targetLang := r.FormValue("target_lang")
	importType := r.FormValue("type")
	if importType == "" {
		importType = "words"
	}

	// Detect file type by extension
	isXLSX := strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx")

	var records [][]string
	if isXLSX {
		records, err = parseXLSXRecords(fileBytes, importType)
		if err != nil {
			msg := "XLSX parse error: " + err.Error()
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = fmt.Fprintf(w, `<p class="text-red-600 text-sm">%s</p>`, msg)
				return
			}
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
	} else {
		// CSV — validate UTF-8
		fileBytes = bytes.TrimPrefix(fileBytes, []byte{0xEF, 0xBB, 0xBF})
		if !utf8.Valid(fileBytes) {
			msg := `File is not UTF-8 encoded. Use "Export XLSX" then re-import the .xlsx file, or save CSV as UTF-8.`
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = fmt.Fprintf(w, `<p class="text-red-600 text-sm">⚠ %s</p>`, msg)
				return
			}
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		reader := csv.NewReader(bytes.NewReader(fileBytes))
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true
		reader.TrimLeadingSpace = true
		records, err = reader.ReadAll()
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "CSV read error: "+err.Error())
			return
		}
	}

	if len(records) == 0 {
		writeJSONError(w, http.StatusBadRequest, "file is empty")
		return
	}

	// Detect header row
	colMap := make(map[string]int)
	firstRow := records[0]
	hasHeader := false
	for i, h := range firstRow {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "word" || key == "expression" || key == "definition" || key == "english" || key == "type" {
			hasHeader = true
		}
		colMap[key] = i
	}
	dataRows := records
	if hasHeader {
		dataRows = records[1:]
	}

	now := time.Now().UTC().Format(time.RFC3339)

	getCol := func(record []string, name string, fallbackIdx int) string {
		if hasHeader {
			if idx, ok := colMap[name]; ok && idx < len(record) {
				return strings.TrimSpace(record[idx])
			}
			return ""
		}
		if fallbackIdx < len(record) {
			return strings.TrimSpace(record[fallbackIdx])
		}
		return ""
	}

	containsGarbage := func(s string) bool {
		return strings.Contains(s, "\ufffd") || strings.Contains(s, "�")
	}

	if importType == "words" {
		var rows []db.WordRow
		skippedGarbage := 0
		for _, record := range dataRows {
			if len(record) == 0 {
				continue
			}
			word := getCol(record, "word", 0)
			if word == "" {
				continue
			}
			if containsGarbage(word) {
				skippedGarbage++
				continue
			}
			row := db.WordRow{
				Word:              word,
				PartOfSpeech:      getCol(record, "type", 1),
				Article:           getCol(record, "article", 2),
				Definition:        getCol(record, "definition", 3),
				EnglishDefinition: getCol(record, "english_definition", 4),
				Example:           getCol(record, "example", 5),
				English:           getCol(record, "english", 6),
				TargetTranslation: getCol(record, "target_translation", 7),
				Notes:             getCol(record, "notes", 8),
				Connotation:       getCol(record, "connotation", 9),
				Register:          getCol(record, "register", 10),
				Collocations:      getCol(record, "collocations", 11),
				ContrastiveNotes:  getCol(record, "contrastive_notes", 12),
				SecondaryMeanings: getCol(record, "secondary_meanings", 13),
				Tags:              getCol(record, "tags", 14),
				SourceLanguage:    sourceLang,
				TargetLanguage:    targetLang,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			rows = append(rows, row)
		}
		imported, skipped, failed, err := s.store.ImportWords(r.Context(), rows)
		if err != nil {
			s.logger.Error("import words failed", "error", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		skipped += skippedGarbage
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			msg := fmt.Sprintf(`<p class="text-green-600 text-sm">Imported %d words (%d skipped, %d failed)</p>`, imported, skipped, failed)
			if skippedGarbage > 0 {
				msg += fmt.Sprintf(`<p class="text-yellow-600 text-sm">⚠ %d rows skipped due to encoding issues</p>`, skippedGarbage)
			}
			_, _ = fmt.Fprint(w, msg)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "skipped": skipped, "failed": failed, "type": "words"})
	} else {
		var rows []db.ExpressionRow
		skippedGarbage := 0
		for _, record := range dataRows {
			if len(record) == 0 {
				continue
			}
			expr := getCol(record, "expression", 0)
			if expr == "" {
				continue
			}
			if containsGarbage(expr) {
				skippedGarbage++
				continue
			}
			row := db.ExpressionRow{
				Expression:        expr,
				Definition:        getCol(record, "definition", 1),
				EnglishDefinition: getCol(record, "english_definition", 2),
				Example:           getCol(record, "example", 3),
				English:           getCol(record, "english", 4),
				TargetTranslation: getCol(record, "target_translation", 5),
				Notes:             getCol(record, "notes", 6),
				Connotation:       getCol(record, "connotation", 7),
				Register:          getCol(record, "register", 8),
				ContrastiveNotes:  getCol(record, "contrastive_notes", 9),
				Tags:              getCol(record, "tags", 10),
				SourceLanguage:    sourceLang,
				TargetLanguage:    targetLang,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			rows = append(rows, row)
		}
		imported, skipped, failed, err := s.store.ImportExpressions(r.Context(), rows)
		if err != nil {
			s.logger.Error("import expressions failed", "error", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		skipped += skippedGarbage
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			msg := fmt.Sprintf(`<p class="text-green-600 text-sm">Imported %d expressions (%d skipped, %d failed)</p>`, imported, skipped, failed)
			if skippedGarbage > 0 {
				msg += fmt.Sprintf(`<p class="text-yellow-600 text-sm">⚠ %d rows skipped due to encoding issues</p>`, skippedGarbage)
			}
			_, _ = fmt.Fprint(w, msg)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "skipped": skipped, "failed": failed, "type": "expressions"})
	}
}

// parseXLSXRecords reads an XLSX file and returns rows as string slices.
// For words, reads the "Words" sheet (or first sheet). For expressions, reads "Expressions" (or second/first sheet).
func parseXLSXRecords(data []byte, importType string) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Pick the right sheet
	sheetName := ""
	sheets := f.GetSheetList()
	if importType == "expressions" {
		for _, s := range sheets {
			if strings.EqualFold(s, "Expressions") {
				sheetName = s
				break
			}
		}
	} else {
		for _, s := range sheets {
			if strings.EqualFold(s, "Words") {
				sheetName = s
				break
			}
		}
	}
	if sheetName == "" && len(sheets) > 0 {
		sheetName = sheets[0] // fallback to first sheet
	}
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in xlsx")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("read sheet %q: %w", sheetName, err)
	}
	return rows, nil
}

// handleExport handles GET /api/export — Excel export.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	sourceLang := r.URL.Query().Get("source_lang")
	targetLang := r.URL.Query().Get("target_lang")
	search := r.URL.Query().Get("search")
	tags := r.URL.Query().Get("tags")

	filter := db.ListFilter{
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Search:     search,
		Tags:       tags,
		Page:       1,
		PageSize:   10000, // export all
	}

	// Fetch words
	var wordEntries []output.Entry
	words, _, err := s.store.ListWords(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, wr := range words {
		wordEntries = append(wordEntries, output.Entry{
			Word: wr.Word, Type: wr.PartOfSpeech, Article: wr.Article,
			Definition: wr.Definition, EnglishDefinition: wr.EnglishDefinition,
			Example: wr.Example, English: wr.English, TargetTranslation: wr.TargetTranslation,
			Notes: wr.Notes, Connotation: wr.Connotation, Register: wr.Register,
			Collocations: wr.Collocations, ContrastiveNotes: wr.ContrastiveNotes,
			SecondaryMeanings: wr.SecondaryMeanings, Tags: wr.Tags,
		})
	}

	// Fetch expressions
	var exprEntries []output.Entry
	exprs, _, err := s.store.ListExpressions(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, er := range exprs {
		exprEntries = append(exprEntries, output.Entry{
			Expression: er.Expression, Definition: er.Definition,
			EnglishDefinition: er.EnglishDefinition, Example: er.Example,
			English: er.English, TargetTranslation: er.TargetTranslation,
			Notes: er.Notes, Connotation: er.Connotation, Register: er.Register,
			ContrastiveNotes: er.ContrastiveNotes, Tags: er.Tags,
		})
	}

	data, err := output.ExportBothToExcel(wordEntries, exprEntries)
	if err != nil {
		s.logger.Error("export failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "export failed: "+err.Error())
		return
	}

	lang := sourceLang
	if lang == "" {
		lang = "all"
	}
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("vocabgen-%s-%s.xlsx", lang, date)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write(data)
}

// decodeJSON is a helper to decode JSON request bodies.
func decodeJSON(r *http.Request, v any) error {
	return decodeJSONBody(r, v)
}
