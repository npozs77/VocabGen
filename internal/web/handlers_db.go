package web

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/output"
)

func parseListParams(r *http.Request) (db.ListFilter, error) {
	filter := db.ListFilter{
		SourceLang: r.URL.Query().Get("source_lang"),
		TargetLang: r.URL.Query().Get("target_lang"),
		Search:     r.URL.Query().Get("search"),
		Page:       1,
		PageSize:   50,
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
		}
		renderPartial(w, "entry_table", data)
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
			"BaseURL":     "/api/expressions",
			"SourceLang":  filter.SourceLang,
			"TargetLang":  filter.TargetLang,
			"Search":      filter.Search,
		}
		renderPartial(w, "entry_table", data)
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

	var row db.WordRow
	if r.Header.Get("Content-Type") == "application/json" {
		if err := decodeJSON(r, &row); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form")
			return
		}
		row = db.WordRow{
			Word:              r.FormValue("word"),
			PartOfSpeech:      r.FormValue("part_of_speech"),
			Article:           r.FormValue("article"),
			Definition:        r.FormValue("definition"),
			Example:           r.FormValue("example"),
			English:           r.FormValue("english"),
			TargetTranslation: r.FormValue("target_translation"),
			Notes:             r.FormValue("notes"),
			Tags:              r.FormValue("tags"),
		}
	}
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.store.UpdateWord(r.Context(), id, &row); err != nil {
		s.logger.Error("update word failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// For HTMX, re-render the table
	if r.Header.Get("HX-Request") == "true" {
		s.handleListWords(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleUpdateExpression handles PUT /api/expressions/{id}.
func (s *Server) handleUpdateExpression(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var row db.ExpressionRow
	if r.Header.Get("Content-Type") == "application/json" {
		if err := decodeJSON(r, &row); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form")
			return
		}
		row = db.ExpressionRow{
			Expression:        r.FormValue("expression"),
			Definition:        r.FormValue("definition"),
			Example:           r.FormValue("example"),
			English:           r.FormValue("english"),
			TargetTranslation: r.FormValue("target_translation"),
			Notes:             r.FormValue("notes"),
			Tags:              r.FormValue("tags"),
		}
	}
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.store.UpdateExpression(r.Context(), id, &row); err != nil {
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

// handleImport handles POST /api/import — CSV import.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
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

	sourceLang := r.FormValue("source_lang")
	targetLang := r.FormValue("target_lang")
	importType := r.FormValue("type")
	if importType == "" {
		importType = "words"
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	now := time.Now().UTC().Format(time.RFC3339)

	if importType == "words" {
		var rows []db.WordRow
		for {
			record, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "CSV read error: "+err.Error())
				return
			}
			if len(record) == 0 || strings.TrimSpace(record[0]) == "" {
				continue
			}
			row := db.WordRow{
				Word:           strings.TrimSpace(record[0]),
				SourceLanguage: sourceLang,
				TargetLanguage: targetLang,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if len(record) > 1 {
				row.Definition = strings.TrimSpace(record[1])
			}
			if len(record) > 2 {
				row.English = strings.TrimSpace(record[2])
			}
			if len(record) > 3 {
				row.Tags = strings.TrimSpace(record[3])
			}
			rows = append(rows, row)
		}
		imported, skipped, failed, err := s.store.ImportWords(r.Context(), rows)
		if err != nil {
			s.logger.Error("import words failed", "error", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result := map[string]any{
			"imported": imported,
			"skipped":  skipped,
			"failed":   failed,
			"type":     "words",
		}
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<p class="text-green-600 text-sm">Imported %d words (%d skipped, %d failed)</p>`, imported, skipped, failed)
			return
		}
		writeJSON(w, http.StatusOK, result)
	} else {
		var rows []db.ExpressionRow
		for {
			record, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "CSV read error: "+err.Error())
				return
			}
			if len(record) == 0 || strings.TrimSpace(record[0]) == "" {
				continue
			}
			row := db.ExpressionRow{
				Expression:     strings.TrimSpace(record[0]),
				SourceLanguage: sourceLang,
				TargetLanguage: targetLang,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if len(record) > 1 {
				row.Definition = strings.TrimSpace(record[1])
			}
			if len(record) > 2 {
				row.English = strings.TrimSpace(record[2])
			}
			if len(record) > 3 {
				row.Tags = strings.TrimSpace(record[3])
			}
			rows = append(rows, row)
		}
		imported, skipped, failed, err := s.store.ImportExpressions(r.Context(), rows)
		if err != nil {
			s.logger.Error("import expressions failed", "error", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result := map[string]any{
			"imported": imported,
			"skipped":  skipped,
			"failed":   failed,
			"type":     "expressions",
		}
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<p class="text-green-600 text-sm">Imported %d expressions (%d skipped, %d failed)</p>`, imported, skipped, failed)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// handleExport handles GET /api/export — Excel export.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	sourceLang := r.URL.Query().Get("source_lang")
	targetLang := r.URL.Query().Get("target_lang")
	exportType := r.URL.Query().Get("type")
	if exportType == "" {
		exportType = "words"
	}

	filter := db.ListFilter{
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Page:       1,
		PageSize:   10000, // export all
	}

	var entries []output.Entry
	if exportType == "words" {
		words, _, err := s.store.ListWords(r.Context(), filter)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, wr := range words {
			entries = append(entries, output.Entry{
				Word:              wr.Word,
				Type:              wr.PartOfSpeech,
				Article:           wr.Article,
				Definition:        wr.Definition,
				EnglishDefinition: wr.EnglishDefinition,
				Example:           wr.Example,
				English:           wr.English,
				TargetTranslation: wr.TargetTranslation,
				Notes:             wr.Notes,
				Connotation:       wr.Connotation,
				Register:          wr.Register,
				Collocations:      wr.Collocations,
				ContrastiveNotes:  wr.ContrastiveNotes,
				SecondaryMeanings: wr.SecondaryMeanings,
				Tags:              wr.Tags,
			})
		}
	} else {
		exprs, _, err := s.store.ListExpressions(r.Context(), filter)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, er := range exprs {
			entries = append(entries, output.Entry{
				Expression:        er.Expression,
				Definition:        er.Definition,
				EnglishDefinition: er.EnglishDefinition,
				Example:           er.Example,
				English:           er.English,
				TargetTranslation: er.TargetTranslation,
				Notes:             er.Notes,
				Connotation:       er.Connotation,
				Register:          er.Register,
				ContrastiveNotes:  er.ContrastiveNotes,
				Tags:              er.Tags,
			})
		}
	}

	data, err := output.ExportToExcel(entries, exportType)
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
	filename := fmt.Sprintf("vocabgen-%s-%s-%s.xlsx", lang, exportType, date)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(data)
}

// decodeJSON is a helper to decode JSON request bodies.
func decodeJSON(r *http.Request, v any) error {
	return decodeJSONBody(r, v)
}
