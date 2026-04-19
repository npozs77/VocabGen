package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/config"
)

// TestProfileSwitcher_VisibleOnPages verifies that the active profile name
// appears in the rendered HTML on lookup, batch, and other pages.
//
// Validates: Issue #44 — Active profile name is visible on lookup, batch pages.
func TestProfileSwitcher_VisibleOnPages(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name string
		path string
	}{
		{"lookup", "/"},
		{"batch", "/batch"},
		{"config", "/config"},
		{"database", "/database"},
		{"about", "/about"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			body := w.Body.String()

			// The profile switcher should be present in the nav bar.
			if !strings.Contains(body, "profile-switcher") {
				t.Fatal("response does not contain profile-switcher element")
			}

			// The active profile name should appear.
			if !strings.Contains(body, srv.activeProfile) {
				t.Fatalf("response does not contain active profile name %q", srv.activeProfile)
			}
		})
	}
}

// TestProfileSwitcher_NavSwitch verifies that switching profiles via the nav-bar
// switcher (source=nav) returns the profile_switcher partial, not the config form.
//
// Validates: Issue #44 — Profile switch takes effect immediately for subsequent lookups.
func TestProfileSwitcher_NavSwitch(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":  {Provider: "bedrock", AWSRegion: "us-east-1", ModelID: "claude-v1"},
			"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1", ModelID: "translategemma"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	// Switch to "local" via nav-bar context.
	body := `{"profile":"local","source":"nav"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	respBody := w.Body.String()

	// Should return the profile_switcher partial, not the config form.
	if strings.Contains(respBody, "config-save-form") {
		t.Fatal("nav switch should return profile_switcher partial, not config form")
	}
	if !strings.Contains(respBody, "profile-switcher") {
		t.Fatal("response does not contain profile-switcher element")
	}

	// The new active profile should be highlighted.
	if !strings.Contains(respBody, "local") {
		t.Fatal("response does not contain the switched profile name 'local'")
	}

	// Verify in-memory state was updated.
	if srv.activeProfile != "local" {
		t.Fatalf("expected activeProfile 'local', got %q", srv.activeProfile)
	}
	if srv.cfg.Provider != "openai" {
		t.Fatalf("expected provider 'openai' after switch, got %q", srv.cfg.Provider)
	}
}

// TestProfileSwitcher_ConfigSwitch verifies that switching profiles without
// source=nav still returns the config form (backward compatibility).
//
// Validates: Issue #44 — Regression prevention for config page profile switching.
func TestProfileSwitcher_ConfigSwitch(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":  {Provider: "bedrock", AWSRegion: "us-east-1", ModelID: "claude-v1"},
			"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1", ModelID: "translategemma"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	// Switch without source=nav (config page behavior).
	body := `{"profile":"local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	// Should return the config form, not the profile switcher partial.
	if !strings.Contains(w.Body.String(), "config-save-form") {
		t.Fatal("config switch should return config form HTML")
	}
}

// TestProfileSwitcherPartial_Route verifies that GET /api/profile/switcher
// returns the profile switcher partial with the current active profile.
//
// Validates: Issue #44 — Profile switcher partial endpoint.
func TestProfileSwitcherPartial_Route(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/profile/switcher", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html; charset=utf-8, got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "profile-switcher") {
		t.Fatal("response does not contain profile-switcher element")
	}
	if !strings.Contains(body, srv.activeProfile) {
		t.Fatalf("response does not contain active profile name %q", srv.activeProfile)
	}
}

// TestProfileSwitcher_AllProfilesListed verifies that the profile switcher
// lists all available profiles with the active one highlighted.
//
// Validates: Issue #44 — Current profile is visually highlighted/distinguished.
func TestProfileSwitcher_AllProfilesListed(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":    {Provider: "bedrock", AWSRegion: "us-east-1", ModelID: "claude-v1"},
			"local":   {Provider: "openai", BaseURL: "http://localhost:11434/v1", ModelID: "translategemma"},
			"sandbox": {Provider: "anthropic", ModelID: "claude-3"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// All profiles should be listed in the switcher.
	for _, profile := range []string{"prod", "local", "sandbox"} {
		if !strings.Contains(body, profile) {
			t.Fatalf("response does not contain profile %q", profile)
		}
	}

	// The active profile should have the highlighted class.
	if !strings.Contains(body, "text-blue-600 font-medium bg-blue-50") {
		t.Fatal("response does not contain highlighted active profile styling")
	}
}

// TestProfileSwitcher_ReflectsAfterSwitch verifies that after switching profiles
// via the nav switcher, subsequent page renders show the new active profile.
//
// Validates: Issue #44 — Profile switch takes effect immediately.
func TestProfileSwitcher_ReflectsAfterSwitch(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":  {Provider: "bedrock", AWSRegion: "us-east-1", ModelID: "claude-v1"},
			"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1", ModelID: "translategemma"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	// Switch to "local" via nav.
	switchBody := `{"profile":"local","source":"nav"}`
	switchReq := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(switchBody))
	switchReq.Header.Set("Content-Type", "application/json")
	switchW := httptest.NewRecorder()
	srv.mux.ServeHTTP(switchW, switchReq)

	if switchW.Code != http.StatusOK {
		t.Fatalf("switch: expected 200, got %d", switchW.Code)
	}

	// Now render the lookup page — it should show "local" as active.
	pageReq := httptest.NewRequest(http.MethodGet, "/", nil)
	pageW := httptest.NewRecorder()
	srv.mux.ServeHTTP(pageW, pageReq)

	if pageW.Code != http.StatusOK {
		t.Fatalf("page: expected 200, got %d", pageW.Code)
	}

	body := pageW.Body.String()
	if !strings.Contains(body, "profile-active-name") {
		t.Fatal("page does not contain profile-active-name element")
	}
	// The active profile name span should contain "local".
	if !strings.Contains(body, ">local<") {
		t.Fatal("page does not show 'local' as the active profile after switch")
	}
}
