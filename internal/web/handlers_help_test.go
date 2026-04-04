package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIntegration_UpdateRoute verifies that GET /update returns 200 with text/html
// and contains expected update page content (version, OS/arch info).
//
// Validates: Requirement 5.1
func TestIntegration_UpdateRoute(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/update", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /update: expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("GET /update: expected text/html; charset=utf-8, got %q", ct)
	}

	body := w.Body.String()

	// The update page should display the current version
	if !strings.Contains(body, "test") {
		t.Fatal("GET /update: response does not contain the version string")
	}

	// The update page should contain update-related content
	if !strings.Contains(body, "Update") && !strings.Contains(body, "update") {
		t.Fatal("GET /update: response does not contain update-related content")
	}
}

// TestIntegration_ChangelogRoute verifies that GET /changelog returns 200 with
// text/html and contains rendered changelog content.
//
// Validates: Requirements 9.1, 9.2
func TestIntegration_ChangelogRoute(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/changelog", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /changelog: expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("GET /changelog: expected text/html; charset=utf-8, got %q", ct)
	}

	body := w.Body.String()

	// The changelog page should contain the "Changelog" heading
	if !strings.Contains(body, "Changelog") {
		t.Fatal("GET /changelog: response does not contain 'Changelog' heading")
	}

	// The changelog should contain rendered CHANGELOG.md content.
	// CHANGELOG.md follows Keep a Changelog format, so it should contain version entries.
	if !strings.Contains(body, "Changed") && !strings.Contains(body, "Added") && !strings.Contains(body, "Fixed") {
		t.Fatal("GET /changelog: response does not contain changelog section markers (Added/Changed/Fixed)")
	}
}

// TestIntegration_UpdateCheckAPI verifies that GET /api/update/check returns 200
// with an HTML partial. The handler calls the GitHub API, so the response may
// contain either update info or an error message — both are valid and render
// the update_result partial with a 200 status.
//
// Validates: Requirement 5.2
func TestIntegration_UpdateCheckAPI(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/update/check: expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("GET /api/update/check: expected text/html; charset=utf-8, got %q", ct)
	}

	body := w.Body.String()

	// The partial should contain one of the three states:
	// - "up to date" (green)
	// - "Update available" (blue)
	// - "Update check failed" (yellow)
	hasUpToDate := strings.Contains(body, "up to date")
	hasUpdateAvailable := strings.Contains(body, "Update available")
	hasCheckFailed := strings.Contains(body, "Update check failed")

	if !hasUpToDate && !hasUpdateAvailable && !hasCheckFailed {
		t.Fatalf("GET /api/update/check: response does not contain any expected state (up to date / Update available / Update check failed); body: %s", body)
	}
}

// TestIntegration_UpdateDismissAPI verifies that POST /api/update/dismiss
// returns 200 and subsequent page renders no longer show the update banner.
//
// Validates: Requirement 7.3
func TestIntegration_UpdateDismissAPI(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/update/dismiss", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/update/dismiss: expected 200, got %d", w.Code)
	}

	// After dismissal, the updater should report dismissed=true
	if !srv.updater.isDismissed() {
		t.Fatal("POST /api/update/dismiss: updater should be dismissed after call")
	}
}

// TestIntegration_UpdateAndChangelogRoutes_TableDriven is a table-driven test
// that verifies all update and changelog related routes return expected status
// codes and content types.
//
// Validates: Requirements 5.1, 7.3, 9.1, 9.2
func TestIntegration_UpdateAndChangelogRoutes_TableDriven(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCT     string
	}{
		{
			name:       "GET /update returns HTML",
			method:     http.MethodGet,
			path:       "/update",
			wantStatus: http.StatusOK,
			wantCT:     "text/html; charset=utf-8",
		},
		{
			name:       "GET /changelog returns HTML",
			method:     http.MethodGet,
			path:       "/changelog",
			wantStatus: http.StatusOK,
			wantCT:     "text/html; charset=utf-8",
		},
		{
			name:       "GET /api/update/check returns HTML partial",
			method:     http.MethodGet,
			path:       "/api/update/check",
			wantStatus: http.StatusOK,
			wantCT:     "text/html; charset=utf-8",
		},
		{
			name:       "POST /api/update/dismiss returns 200",
			method:     http.MethodPost,
			path:       "/api/update/dismiss",
			wantStatus: http.StatusOK,
			wantCT:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d; body: %s", tc.wantStatus, w.Code, w.Body.String())
			}

			if tc.wantCT != "" {
				ct := w.Header().Get("Content-Type")
				if ct != tc.wantCT {
					t.Fatalf("expected Content-Type %q, got %q", tc.wantCT, ct)
				}
			}
		})
	}
}
