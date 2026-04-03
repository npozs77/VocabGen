package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Test 1: GET /api/health returns 200 {"status": "ok"} ---

func TestIntegration_HealthEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

// --- Test 2: GET /api/languages returns supported languages as JSON array ---

func TestIntegration_LanguagesEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/languages", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content type, got %q", ct)
	}
	var langs []map[string]string
	if err := json.NewDecoder(w.Body).Decode(&langs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(langs) == 0 {
		t.Fatal("expected non-empty languages list")
	}
	// Each entry should have Code and Name
	for _, l := range langs {
		if l["Code"] == "" && l["Name"] == "" {
			t.Fatal("language entry missing Code and Name")
		}
	}
}

// --- Test 3: GET /api/config returns current config as JSON ---

func TestIntegration_GetConfig(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var cfg map[string]any
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Config uses yaml tags; json.Encoder falls back to Go field names when no json tag.
	provider, ok := cfg["Provider"]
	if !ok {
		provider, ok = cfg["provider"]
	}
	if !ok {
		t.Fatalf("expected provider field in response, got keys: %v", cfg)
	}
	if provider != "bedrock" {
		t.Fatalf("expected provider bedrock, got %v", provider)
	}
}

// --- Test 4: PUT /api/config with form-encoded body updates config ---

func TestIntegration_PutConfig_FormEncoded(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	srv := newTestServer()

	form := "provider=openai&model_id=gpt-4o&default_source_language=hu&default_target_language=en"
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// PUT /api/config returns HTML for HTMX, so just check it's not an error status
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	// Verify in-memory config was updated
	if srv.cfg.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", srv.cfg.Provider)
	}
	if srv.cfg.ModelID != "gpt-4o" {
		t.Fatalf("expected model_id gpt-4o, got %q", srv.cfg.ModelID)
	}
}

// --- Test 5: POST /api/lookup with empty text returns 400 ---

func TestIntegration_Lookup_EmptyText(t *testing.T) {
	srv := newTestServer()

	body := `{"source_language":"nl","text":"","lookup_type":"word"}`
	req := httptest.NewRequest(http.MethodPost, "/api/lookup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
	var errBody map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errBody["detail"] == "" {
		t.Fatal("expected non-empty detail in error response")
	}
}

// --- Test 6: POST /api/lookup/resolve with invalid strategy returns 400 ---

func TestIntegration_LookupResolve_InvalidStrategy(t *testing.T) {
	srv := newTestServer()

	body := `{"strategy":"invalid_strategy","entry":{"word":"test","definition":"d","example":"e","english":"en","target_translation":"tt","notes":""},"mode":"words"}`
	req := httptest.NewRequest(http.MethodPost, "/api/lookup/resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
	var errBody map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(errBody["detail"], "invalid conflict strategy") {
		t.Fatalf("expected invalid strategy error, got %q", errBody["detail"])
	}
}

// --- Test 7: GET /api/words returns JSON with words array and total count ---

func TestIntegration_ListWords(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/words", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// stubStore returns nil/0, so words should be null and total 0
	if _, ok := body["words"]; !ok {
		t.Fatal("expected 'words' key in response")
	}
	total, ok := body["total"].(float64)
	if !ok {
		t.Fatal("expected 'total' key as number")
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %v", total)
	}
	if _, ok := body["page"]; !ok {
		t.Fatal("expected 'page' key in response")
	}
}

// --- Test 8: GET /api/expressions returns JSON with expressions array and total count ---

func TestIntegration_ListExpressions(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/expressions", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["expressions"]; !ok {
		t.Fatal("expected 'expressions' key in response")
	}
	total, ok := body["total"].(float64)
	if !ok {
		t.Fatal("expected 'total' key as number")
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %v", total)
	}
}

// --- Test 9: DELETE /api/words/{id} returns 200 with stub store ---

func TestIntegration_DeleteWord(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodDelete, "/api/words/42", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "deleted" {
		t.Fatalf("expected status deleted, got %q", body["status"])
	}
}

// --- Test 10: DELETE /api/expressions/{id} returns 200 with stub store ---

func TestIntegration_DeleteExpression(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodDelete, "/api/expressions/99", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "deleted" {
		t.Fatalf("expected status deleted, got %q", body["status"])
	}
}

// --- Test 11: POST /api/import with multipart form returns import summary ---

func TestIntegration_Import_Multipart(t *testing.T) {
	srv := newTestServer()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add CSV file part
	part, err := writer.CreateFormFile("file", "words.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = fmt.Fprintln(part, "huis,a house,house,chapter-1")
	_, _ = fmt.Fprintln(part, "werk,work/job,work,chapter-1")

	// Add form fields
	_ = writer.WriteField("source_lang", "nl")
	_ = writer.WriteField("target_lang", "hu")
	_ = writer.WriteField("type", "words")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["type"] != "words" {
		t.Fatalf("expected type words, got %v", body["type"])
	}
	// stubStore.ImportWords returns (0,0,0,nil), so imported=0
	if _, ok := body["imported"]; !ok {
		t.Fatal("expected 'imported' key in response")
	}
	if _, ok := body["skipped"]; !ok {
		t.Fatal("expected 'skipped' key in response")
	}
	if _, ok := body["failed"]; !ok {
		t.Fatal("expected 'failed' key in response")
	}
}

// --- Test 12: POST /api/batch with oversized body returns 413 ---

func TestIntegration_Batch_OversizedBody(t *testing.T) {
	srv := newTestServer()

	// Create a multipart body that exceeds 10 MB
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "huge.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	// Write >10 MB of data
	line := strings.Repeat("a", 1024) + "\n"
	for i := 0; i < 11*1024; i++ { // ~11 MB
		_, _ = part.Write([]byte(line))
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/batch", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d; body: %s", w.Code, w.Body.String())
	}
}

// --- Test 13: Error responses include {"detail": "..."} format ---

func TestIntegration_ErrorResponseFormat(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		ct     string
	}{
		{
			name:   "lookup empty text",
			method: http.MethodPost,
			path:   "/api/lookup",
			body:   `{"text":""}`,
			ct:     "application/json",
		},
		{
			name:   "resolve invalid strategy",
			method: http.MethodPost,
			path:   "/api/lookup/resolve",
			body:   `{"strategy":"bogus","entry":{"word":"x","definition":"d","example":"e","english":"en","target_translation":"tt","notes":""}}`,
			ct:     "application/json",
		},
		{
			name:   "words invalid page",
			method: http.MethodGet,
			path:   "/api/words?page=-1",
		},
		{
			name:   "expressions invalid page_size",
			method: http.MethodGet,
			path:   "/api/expressions?page_size=999",
		},
		{
			name:   "delete word invalid id",
			method: http.MethodDelete,
			path:   "/api/words/abc",
		},
	}

	srv := newTestServer()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			if tc.ct != "" {
				req.Header.Set("Content-Type", tc.ct)
			}
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code < 400 {
				t.Fatalf("expected error status (>=400), got %d", w.Code)
			}
			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Fatalf("expected JSON content type for error, got %q", ct)
			}
			var errBody map[string]string
			if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if errBody["detail"] == "" {
				t.Fatalf("expected non-empty 'detail' field in error response, got: %v", errBody)
			}
		})
	}
}
