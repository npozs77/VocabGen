package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/config"
	"pgregory.net/rapid"
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

// --- Test: About page backward compatibility ---

// TestIntegration_AboutPage_BackwardCompatibility verifies that GET /about
// returns 200 with text/html and contains expected content.
//
// Validates: Requirements 2.1, 2.2
func TestIntegration_AboutPage_BackwardCompatibility(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Verify status code
	if w.Code != http.StatusOK {
		t.Fatalf("GET /about: expected 200, got %d", w.Code)
	}

	// Verify content type
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("GET /about: expected text/html; charset=utf-8, got %q", ct)
	}

	body := w.Body.String()

	// Verify "About VocabGen" heading is present
	if !strings.Contains(body, "About VocabGen") {
		t.Fatal("GET /about: response body does not contain 'About VocabGen'")
	}

	// Verify version is rendered (the test server uses "test" as version)
	if !strings.Contains(body, "test") {
		t.Fatal("GET /about: response body does not contain the version string")
	}

	// Verify build date is rendered
	if !strings.Contains(body, "unknown") {
		t.Fatal("GET /about: response body does not contain the build date")
	}

	// Verify Go version is rendered
	if !strings.Contains(body, "go1.22") {
		t.Fatal("GET /about: response body does not contain the Go version")
	}

	// Verify key structural elements from about.html template
	if !strings.Contains(body, "Version") {
		t.Fatal("GET /about: response body does not contain 'Version' label")
	}
	if !strings.Contains(body, "Build Date") {
		t.Fatal("GET /about: response body does not contain 'Build Date' label")
	}
	if !strings.Contains(body, "Go Version") {
		t.Fatal("GET /about: response body does not contain 'Go Version' label")
	}
	if !strings.Contains(body, "Provider") {
		t.Fatal("GET /about: response body does not contain 'Provider' label")
	}
	if !strings.Contains(body, "Database") {
		t.Fatal("GET /about: response body does not contain 'Database' label")
	}

	// Verify links section
	if !strings.Contains(body, "GitHub Repository") {
		t.Fatal("GET /about: response body does not contain 'GitHub Repository' link")
	}
	if !strings.Contains(body, "Report an Issue") {
		t.Fatal("GET /about: response body does not contain 'Report an Issue' link")
	}
	if !strings.Contains(body, "MIT License") {
		t.Fatal("GET /about: response body does not contain 'MIT License' link")
	}
}

// TestPropertyP21_AboutPageContentIdentical verifies that the About page
// content remains identical regardless of the version, build date, and Go
// version passed to the server. The structural elements (headings, labels,
// links) are always present — only the dynamic values change.
//
// Property P2.1: About page content is identical before and after the navigation restructure
// Validates: Requirements 2.1, 2.2
func TestPropertyP21_AboutPageContentIdentical(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary version strings
		version := rapid.StringMatching(`[a-z0-9]{1,20}`).Draw(t, "version")
		buildDate := rapid.StringMatching(`[0-9]{4}-[0-9]{2}-[0-9]{2}`).Draw(t, "buildDate")
		goVersion := rapid.StringMatching(`go1\.[0-9]{1,2}(\.[0-9]{1,2})?`).Draw(t, "goVersion")

		cfg := config.DefaultConfig()
		srv := NewServer(&stubStore{}, &cfg, slog.Default(), version, buildDate, goVersion, "/tmp/test.db")

		req := httptest.NewRequest(http.MethodGet, "/about", nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /about: expected 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Structural invariants: these elements must always be present
		// regardless of the dynamic values passed to the server.
		invariants := []string{
			"About VocabGen",
			"Version",
			"Build Date",
			"Go Version",
			"Provider",
			"Database",
			"GitHub Repository",
			"Report an Issue",
			"MIT License",
		}
		for _, inv := range invariants {
			if !strings.Contains(body, inv) {
				t.Errorf("About page missing structural element %q for version=%q buildDate=%q goVersion=%q",
					inv, version, buildDate, goVersion)
			}
		}

		// Dynamic values must appear in the rendered output
		if !strings.Contains(body, version) {
			t.Errorf("About page does not contain version %q", version)
		}
		if !strings.Contains(body, buildDate) {
			t.Errorf("About page does not contain buildDate %q", buildDate)
		}
		if !strings.Contains(body, goVersion) {
			t.Errorf("About page does not contain goVersion %q", goVersion)
		}
	})
}

// TestPropertyP22_AboutURLServesAboutPage verifies that the /about URL
// always serves the About page with correct status and content type,
// regardless of the server configuration.
//
// Property P2.2: The /about URL continues to serve the About page
// Validates: Requirements 2.1, 2.2
func TestPropertyP22_AboutURLServesAboutPage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		version := rapid.StringMatching(`[a-z0-9]{1,20}`).Draw(t, "version")

		cfg := config.DefaultConfig()
		srv := NewServer(&stubStore{}, &cfg, slog.Default(), version, "2026-01-01", "go1.22", "/tmp/test.db")

		req := httptest.NewRequest(http.MethodGet, "/about", nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		// /about must always return 200
		if w.Code != http.StatusOK {
			t.Fatalf("GET /about: expected 200, got %d (version=%q)", w.Code, version)
		}

		// /about must always return text/html
		ct := w.Header().Get("Content-Type")
		if ct != "text/html; charset=utf-8" {
			t.Fatalf("GET /about: expected text/html; charset=utf-8, got %q (version=%q)", ct, version)
		}

		// /about must always contain "About VocabGen" (proves it's the About page, not a redirect)
		body := w.Body.String()
		if !strings.Contains(body, "About VocabGen") {
			t.Fatalf("GET /about: response does not contain 'About VocabGen' (version=%q)", version)
		}
	})
}

// --- Test: Help dropdown links in rendered HTML ---

// TestIntegration_HelpDropdownLinks verifies that rendered pages contain the
// Help dropdown markup with the correct links in the correct order.
//
// Validates: Requirements 1.2, 3.1, 3.2
func TestIntegration_HelpDropdownLinks(t *testing.T) {
	srv := newTestServer()

	// Test across multiple pages to ensure the dropdown is present on all of them
	pages := []struct {
		name string
		path string
	}{
		{"lookup", "/"},
		{"batch", "/batch"},
		{"config", "/config"},
		{"about", "/about"},
		{"docs", "/docs"},
		{"update", "/update"},
	}

	for _, pg := range pages {
		t.Run(pg.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, pg.path, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("GET %s: expected 200, got %d", pg.path, w.Code)
			}

			body := w.Body.String()

			// Verify Help dropdown container exists
			if !strings.Contains(body, `id="help-menu"`) {
				t.Fatalf("GET %s: response does not contain help-menu container", pg.path)
			}
			if !strings.Contains(body, `id="help-dropdown"`) {
				t.Fatalf("GET %s: response does not contain help-dropdown panel", pg.path)
			}

			// Verify Help button text
			if !strings.Contains(body, "Help ▾") {
				t.Fatalf("GET %s: response does not contain 'Help ▾' button text", pg.path)
			}

			// Verify all five dropdown links are present
			expectedLinks := []struct {
				href string
				text string
			}{
				{`href="/about"`, "About"},
				{`href="https://github.com/npozs77/VocabGen/issues"`, "Report an Issue"},
				{`href="/docs"`, "Documentation"},
				{`href="/changelog"`, "Changelog"},
				{`href="/update"`, "Check for Update"},
			}
			for _, link := range expectedLinks {
				if !strings.Contains(body, link.href) {
					t.Errorf("GET %s: response does not contain link %s", pg.path, link.href)
				}
				if !strings.Contains(body, link.text) {
					t.Errorf("GET %s: response does not contain link text %q", pg.path, link.text)
				}
			}

			// Verify Report an Issue link has target="_blank" and rel="noopener noreferrer"
			if !strings.Contains(body, `target="_blank"`) {
				t.Errorf("GET %s: Report an Issue link missing target=\"_blank\"", pg.path)
			}
			if !strings.Contains(body, `rel="noopener noreferrer"`) {
				t.Errorf("GET %s: Report an Issue link missing rel=\"noopener noreferrer\"", pg.path)
			}
		})
	}
}

// TestIntegration_HelpDropdownLinkOrder verifies that the five Help dropdown
// links appear in the specified order: About, Report an Issue, Documentation,
// Changelog, Check for Update.
//
// Validates: Requirements 1.2
func TestIntegration_HelpDropdownLinkOrder(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /: expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Find positions of each link text within the dropdown
	orderedTexts := []string{
		"/about",
		"https://github.com/npozs77/VocabGen/issues",
		`href="/docs"`,
		"/changelog",
		"/update",
	}

	prevIdx := -1
	for _, text := range orderedTexts {
		idx := strings.Index(body, text)
		if idx == -1 {
			t.Fatalf("link %q not found in response body", text)
		}
		if idx <= prevIdx {
			t.Fatalf("link %q (at %d) appears before or at same position as previous link (at %d); expected ascending order", text, idx, prevIdx)
		}
		prevIdx = idx
	}
}

// TestIntegration_ReportAnIssueSecureAttributes verifies that the Report an
// Issue link in the Help dropdown has the correct GitHub URL, target="_blank",
// and rel="noopener noreferrer" attributes.
//
// Validates: Requirements 3.1, 3.2
func TestIntegration_ReportAnIssueSecureAttributes(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /: expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// The Report an Issue link should contain all three attributes in the same <a> tag.
	// We check for the combined pattern that appears in base.html.
	issueLink := `href="https://github.com/npozs77/VocabGen/issues" target="_blank" rel="noopener noreferrer"`
	if !strings.Contains(body, issueLink) {
		t.Fatal("Report an Issue link does not contain the expected href + target + rel attributes together")
	}
}

// TestPropertyP12_HelpDropdownContainsFiveLinksInOrder verifies that the Help
// dropdown always contains exactly five links in the specified order regardless
// of which page is rendered.
//
// Property P1.2: Help_Dropdown contains exactly five links in the specified order
// Validates: Requirements 1.2
func TestPropertyP12_HelpDropdownContainsFiveLinksInOrder(t *testing.T) {
	pagePaths := []string{"/", "/batch", "/config", "/about", "/docs", "/update"}

	rapid.Check(t, func(t *rapid.T) {
		// Pick a random page to render
		idx := rapid.IntRange(0, len(pagePaths)-1).Draw(t, "pageIndex")
		path := pagePaths[idx]

		cfg := config.DefaultConfig()
		srv := NewServer(&stubStore{}, &cfg, slog.Default(), "test", "unknown", "go1.22", "/tmp/test.db")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: expected 200, got %d", path, w.Code)
		}

		body := w.Body.String()

		// Extract the help-dropdown section
		dropdownStart := strings.Index(body, `id="help-dropdown"`)
		if dropdownStart == -1 {
			t.Fatalf("GET %s: help-dropdown not found", path)
		}

		// Find the closing </div> for the dropdown
		dropdownSection := body[dropdownStart:]
		closingDiv := strings.Index(dropdownSection, "</div>")
		if closingDiv == -1 {
			t.Fatalf("GET %s: help-dropdown closing tag not found", path)
		}
		dropdownHTML := dropdownSection[:closingDiv]

		// Count the number of <a links in the dropdown
		linkCount := strings.Count(dropdownHTML, "<a ")
		if linkCount != 5 {
			t.Fatalf("GET %s: expected exactly 5 links in help-dropdown, got %d", path, linkCount)
		}

		// Verify order: each link href must appear after the previous one
		orderedHrefs := []string{
			`href="/about"`,
			`href="https://github.com/npozs77/VocabGen/issues"`,
			`href="/docs"`,
			`href="/changelog"`,
			`href="/update"`,
		}

		prevPos := -1
		for _, href := range orderedHrefs {
			pos := strings.Index(dropdownHTML, href)
			if pos == -1 {
				t.Fatalf("GET %s: link %s not found in help-dropdown", path, href)
			}
			if pos <= prevPos {
				t.Fatalf("GET %s: link %s (at %d) is not after previous link (at %d)", path, href, pos, prevPos)
			}
			prevPos = pos
		}
	})
}

// TestPropertyP31_ReportAnIssueOpensCorrectURL verifies that the Report an
// Issue link always points to the correct GitHub issues URL regardless of
// server configuration.
//
// Property P3.1: Report an Issue link opens the correct GitHub issues URL
// Validates: Requirements 3.1
func TestPropertyP31_ReportAnIssueOpensCorrectURL(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		version := rapid.StringMatching(`[a-z0-9]{1,20}`).Draw(t, "version")

		cfg := config.DefaultConfig()
		srv := NewServer(&stubStore{}, &cfg, slog.Default(), version, "2026-01-01", "go1.22", "/tmp/test.db")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /: expected 200, got %d (version=%q)", w.Code, version)
		}

		body := w.Body.String()

		expectedURL := "https://github.com/npozs77/VocabGen/issues"
		if !strings.Contains(body, fmt.Sprintf(`href="%s"`, expectedURL)) {
			t.Fatalf("Report an Issue link does not contain expected URL %q (version=%q)", expectedURL, version)
		}
	})
}

// TestPropertyP32_ReportAnIssueOpensInNewTabSecurely verifies that the Report
// an Issue link always has target="_blank" and rel="noopener noreferrer"
// attributes regardless of server configuration.
//
// Property P3.2: Report an Issue link opens in a new tab with secure attributes
// Validates: Requirements 3.2
func TestPropertyP32_ReportAnIssueOpensInNewTabSecurely(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		version := rapid.StringMatching(`[a-z0-9]{1,20}`).Draw(t, "version")

		cfg := config.DefaultConfig()
		srv := NewServer(&stubStore{}, &cfg, slog.Default(), version, "2026-01-01", "go1.22", "/tmp/test.db")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /: expected 200, got %d (version=%q)", w.Code, version)
		}

		body := w.Body.String()

		// Find the Report an Issue link in the help dropdown
		issueURL := `href="https://github.com/npozs77/VocabGen/issues"`
		issueIdx := strings.Index(body, issueURL)
		if issueIdx == -1 {
			t.Fatalf("Report an Issue link not found (version=%q)", version)
		}

		// Extract the <a> tag containing the issue link
		// Search backward for "<a " and forward for ">"
		tagStart := strings.LastIndex(body[:issueIdx], "<a ")
		if tagStart == -1 {
			t.Fatalf("could not find <a tag start for Report an Issue link (version=%q)", version)
		}
		tagEnd := strings.Index(body[issueIdx:], ">")
		if tagEnd == -1 {
			t.Fatalf("could not find <a tag end for Report an Issue link (version=%q)", version)
		}
		aTag := body[tagStart : issueIdx+tagEnd+1]

		if !strings.Contains(aTag, `target="_blank"`) {
			t.Fatalf("Report an Issue <a> tag missing target=\"_blank\" (version=%q): %s", version, aTag)
		}
		if !strings.Contains(aTag, `rel="noopener noreferrer"`) {
			t.Fatalf("Report an Issue <a> tag missing rel=\"noopener noreferrer\" (version=%q): %s", version, aTag)
		}
	})
}

func TestIntegration_BulkDeleteWords(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid IDs", `{"ids":[1,2,3]}`, http.StatusOK},
		{"empty IDs", `{"ids":[]}`, http.StatusBadRequest},
		{"invalid JSON", `not json`, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/words/bulk", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d; body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_BulkDeleteExpressions(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid IDs", `{"ids":[1,2,3]}`, http.StatusOK},
		{"empty IDs", `{"ids":[]}`, http.StatusBadRequest},
		{"invalid JSON", `{bad}`, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/expressions/bulk", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d; body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_BatchStream_WordList(t *testing.T) {
	srv := newTestServer()

	// Build multipart form with word_list field (no file)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("word_list", "eraan toe zijn\nerachter komen\neraan gaan")
	_ = writer.WriteField("source_language", "nl")
	_ = writer.WriteField("target_language", "hu")
	_ = writer.WriteField("mode", "expressions")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/batch/stream", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	body := w.Body.String()
	// The SSE stream should start with a connected event showing 3 tokens
	if !strings.Contains(body, `"total":3`) {
		t.Fatalf("expected connected event with total:3, got: %s", body)
	}
}

func TestIntegration_BatchStream_NoInput(t *testing.T) {
	srv := newTestServer()

	// Build multipart form with neither file nor word_list
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("source_language", "nl")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/batch/stream", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "CSV file or word list is required") {
		t.Fatalf("expected error about missing input, got: %s", body)
	}
}

func TestIntegration_BatchStream_EmptyWordList(t *testing.T) {
	srv := newTestServer()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("word_list", "  \n  \n  ")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/batch/stream", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	body := w.Body.String()
	// Whitespace-only word_list is treated as empty, so falls through to file check
	if !strings.Contains(body, "CSV file or word list is required") {
		t.Fatalf("expected 'CSV file or word list is required' error, got: %s", body)
	}
}
