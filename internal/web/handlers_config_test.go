package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/user/vocabgen/internal/config"
)

// ollamaRunning returns true if the local Ollama server is reachable.
func ollamaRunning() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func TestValidateProviderEnv(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		baseURL    string
		gcpProject string
		envVars    map[string]string
		wantEmpty  bool // true = no warning expected
		wantSubstr string
		skipIf     bool
	}{
		{
			name:      "openai with env var set",
			provider:  "openai",
			envVars:   map[string]string{"OPENAI_API_KEY": "sk-test"},
			wantEmpty: true,
		},
		{
			name:       "openai without env var",
			provider:   "openai",
			wantSubstr: "OPENAI_API_KEY",
		},
		{
			name:       "openai with Ollama base_url and Ollama not running",
			provider:   "openai",
			baseURL:    "http://localhost:11434/v1",
			wantSubstr: "Ollama server is not reachable",
			skipIf:     ollamaRunning(),
		},
		{
			name:      "openai with non-Ollama base_url skips key check",
			provider:  "openai",
			baseURL:   "http://my-server:8080/v1",
			wantEmpty: true,
		},
		{
			name:      "anthropic with env var set",
			provider:  "anthropic",
			envVars:   map[string]string{"ANTHROPIC_API_KEY": "sk-ant-test"},
			wantEmpty: true,
		},
		{
			name:       "anthropic without env var",
			provider:   "anthropic",
			wantSubstr: "ANTHROPIC_API_KEY",
		},
		{
			name:      "bedrock with AWS_PROFILE set",
			provider:  "bedrock",
			envVars:   map[string]string{"AWS_PROFILE": "vocabgen"},
			wantEmpty: true,
		},
		{
			name:      "bedrock with AWS_ACCESS_KEY_ID set",
			provider:  "bedrock",
			envVars:   map[string]string{"AWS_ACCESS_KEY_ID": "AKIA..."},
			wantEmpty: true,
		},
		{
			name:       "vertexai with gcp_project form field",
			provider:   "vertexai",
			gcpProject: "my-project",
			wantEmpty:  true,
		},
		{
			name:      "vertexai with GCP_PROJECT env var",
			provider:  "vertexai",
			envVars:   map[string]string{"GCP_PROJECT": "my-project"},
			wantEmpty: true,
		},
		{
			name:       "vertexai without project",
			provider:   "vertexai",
			wantSubstr: "GCP project ID",
		},
		{
			name:      "unknown provider passes",
			provider:  "unknown",
			wantEmpty: true,
		},
	}

	envKeys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_ACCESS_KEY_ID", "AWS_PROFILE", "AWS_SESSION_TOKEN", "GCP_PROJECT"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipIf {
				t.Skip("skipped: precondition not met (e.g. Ollama is running locally)")
			}
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			got := validateProviderEnv(tc.provider, tc.baseURL, tc.gcpProject)
			if tc.wantEmpty && got != "" {
				t.Fatalf("expected no warning, got %q", got)
			}
			if !tc.wantEmpty && got == "" {
				t.Fatal("expected a warning, got empty string")
			}
			if tc.wantSubstr != "" && !strings.Contains(got, tc.wantSubstr) {
				t.Fatalf("expected warning to contain %q, got %q", tc.wantSubstr, got)
			}
		})
	}
}

func TestPutConfig_EnvVarValidation(t *testing.T) {
	tests := []struct {
		name       string
		form       string
		envVars    map[string]string
		wantStatus int
		wantSubstr string // expected in response body
		skipIf     bool
	}{
		{
			name:       "openai missing env var returns error",
			form:       "provider=openai&model_id=gpt-4o&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "OPENAI_API_KEY",
		},
		{
			name:       "openai with env var saves ok",
			form:       "provider=openai&model_id=gpt-4o&default_source_language=nl&default_target_language=hu",
			envVars:    map[string]string{"OPENAI_API_KEY": "sk-test"},
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "openai with Ollama base_url checks reachability",
			form:       "provider=openai&model_id=translategemma&base_url=http://localhost:11434/v1&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "Ollama server is not reachable",
			skipIf:     ollamaRunning(),
		},
		{
			name:       "openai with non-Ollama base_url skips key check",
			form:       "provider=openai&model_id=gpt-4o&base_url=http://my-server:8080/v1&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "anthropic missing env var returns error",
			form:       "provider=anthropic&model_id=claude-sonnet-4-20250514&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "ANTHROPIC_API_KEY",
		},
		{
			name:       "anthropic with env var saves ok",
			form:       "provider=anthropic&model_id=claude-sonnet-4-20250514&default_source_language=nl&default_target_language=hu",
			envVars:    map[string]string{"ANTHROPIC_API_KEY": "sk-ant-test"},
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "vertexai missing project returns error",
			form:       "provider=vertexai&model_id=gemini-pro&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "GCP project ID",
		},
		{
			name:       "vertexai with gcp_project field saves ok",
			form:       "provider=vertexai&model_id=gemini-pro&gcp_project=my-proj&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "bedrock with AWS_PROFILE saves ok",
			form:       "provider=bedrock&aws_profile=vocabgen&aws_region=us-east-1&default_source_language=nl&default_target_language=hu",
			envVars:    map[string]string{"AWS_PROFILE": "vocabgen"},
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
	}

	envKeys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_ACCESS_KEY_ID", "AWS_PROFILE", "AWS_SESSION_TOKEN", "GCP_PROJECT"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipIf {
				t.Skip("skipped: precondition not met (e.g. Ollama is running locally)")
			}
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			srv := newTestServer()
			req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(tc.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d; body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
			if tc.wantSubstr != "" && !strings.Contains(w.Body.String(), tc.wantSubstr) {
				t.Fatalf("expected body to contain %q, got %q", tc.wantSubstr, w.Body.String())
			}
		})
	}
}

// TestGetProfiles_ReturnsList tests GET /api/profiles returns profile list.
//
// Validates: Requirement 58.7
func TestGetProfiles_ReturnsList(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body struct {
		Profiles []string `json:"profiles"`
		Active   string   `json:"active"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	// With default flat config, should return ["default"]
	if len(body.Profiles) == 0 {
		t.Fatal("expected at least one profile")
	}
	if body.Active == "" {
		t.Fatal("expected non-empty active profile")
	}
}

// TestGetProfiles_MultiProfile tests GET /api/profiles with multi-profile config.
//
// Validates: Requirement 58.7
func TestGetProfiles_MultiProfile(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":  {Provider: "bedrock", AWSRegion: "us-east-1"},
			"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body struct {
		Profiles []string `json:"profiles"`
		Active   string   `json:"active"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if body.Active != "prod" {
		t.Fatalf("expected active 'prod', got %q", body.Active)
	}
	if len(body.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d: %v", len(body.Profiles), body.Profiles)
	}
}

// TestSwitchProfile_ChangesActiveConfig tests PUT /api/profile/switch.
//
// Validates: Requirement 58.8
func TestSwitchProfile_ChangesActiveConfig(t *testing.T) {
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

	// Switch to "local" profile.
	body := `{"profile":"local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	// The response is now the full re-rendered config form HTML.
	if !strings.Contains(w.Body.String(), "config-save-form") {
		t.Fatalf("expected config form HTML in response, got: %s", w.Body.String())
	}

	// Verify in-memory config was updated.
	if srv.cfg.Provider != "openai" {
		t.Fatalf("expected provider 'openai' after switch, got %q", srv.cfg.Provider)
	}
	if srv.cfg.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("expected base_url from local profile, got %q", srv.cfg.BaseURL)
	}
	if srv.cfg.ModelID != "translategemma" {
		t.Fatalf("expected model_id 'translategemma', got %q", srv.cfg.ModelID)
	}
}

// TestSwitchProfile_InvalidProfile tests PUT /api/profile/switch with bad profile name.
//
// Validates: Requirement 58.4
func TestSwitchProfile_InvalidProfile(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod": {Provider: "bedrock"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	body := `{"profile":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("expected 'not found' in error, got: %s", w.Body.String())
	}
}

// TestSwitchProfile_EmptyProfile tests PUT /api/profile/switch with empty profile.
func TestSwitchProfile_EmptyProfile(t *testing.T) {
	srv := newTestServer()

	body := `{"profile":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestConfigFormRendersWithProfileSelector tests that the config form HTML
// includes the profile selector when multiple profiles exist.
//
// Validates: Requirement 58.7
func TestConfigFormRendersWithProfileSelector(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":  {Provider: "bedrock", AWSRegion: "us-east-1"},
			"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/config/html", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "profile_selector") {
		t.Fatal("expected profile_selector in config form HTML")
	}
}

// TestConfigFormAlwaysShowsProfileSelector tests that the profile selector
// is always visible, even when only one profile exists (flat config).
func TestConfigFormAlwaysShowsProfileSelector(t *testing.T) {
	// Write a flat config directly (bypass SaveConfig's multi-profile detection).
	dir, _ := os.MkdirTemp("", "vocabgen-flat-*")
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() {
		config.SetConfigDirForTest(os.TempDir()) // restore to shared test dir
		_ = os.RemoveAll(dir)
	})

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfg, ""); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/config/html", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "profile_selector") {
		t.Fatal("expected profile_selector to always be visible, even for single-profile config")
	}
	if !strings.Contains(body, "default") {
		t.Fatal("expected 'default' profile name in selector")
	}
}

// TestCreateProfile_ValidName tests POST /api/profiles with a valid new profile name.
//
// Validates: Requirements 58.10, 58.11
func TestCreateProfile_ValidName(t *testing.T) {
	// Write a multi-profile config so we have a known source.
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod": {Provider: "bedrock", AWSRegion: "us-east-1", ModelID: "claude-v1"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	body := "name=sandbox&source_profile=prod"
	req := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "sandbox") {
		t.Fatalf("expected success message mentioning 'sandbox', got: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "created") {
		t.Fatalf("expected 'created' in response, got: %s", w.Body.String())
	}

	// Verify in-memory config was updated to the new profile.
	if srv.cfg.Provider != "bedrock" {
		t.Fatalf("expected provider 'bedrock' (copied from prod), got %q", srv.cfg.Provider)
	}

	// Verify the profile was actually persisted.
	profiles, def, err := config.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if def != "sandbox" {
		t.Fatalf("expected default 'sandbox', got %q", def)
	}
	found := false
	for _, p := range profiles {
		if p == "sandbox" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'sandbox' in profiles, got %v", profiles)
	}
}

// TestCreateProfile_DuplicateName tests POST /api/profiles with a name that already exists.
//
// Validates: Requirement 58.12
func TestCreateProfile_DuplicateName(t *testing.T) {
	fc := config.FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]config.ProfileConfig{
			"prod":  {Provider: "bedrock"},
			"local": {Provider: "openai"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()

	body := "name=prod&source_profile=local"
	req := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already exists") {
		t.Fatalf("expected 'already exists' in error, got: %s", w.Body.String())
	}
}

// TestCreateProfile_EmptyName tests POST /api/profiles with an empty name.
//
// Validates: Requirement 58.10
func TestCreateProfile_EmptyName(t *testing.T) {
	srv := newTestServer()

	body := "name=&source_profile=default"
	req := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "required") {
		t.Fatalf("expected 'required' in error, got: %s", w.Body.String())
	}
}

// TestAPIRoutesRegistered_CreateProfile verifies POST /api/profiles is registered.
func TestAPIRoutesRegistered_CreateProfile(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/profiles", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Should not be 404 or 405 — route is registered.
	if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/profiles route not registered: got %d", w.Code)
	}
}

// TestProfileSwitch_ConfigHTMLReflectsNewProfile verifies that after switching
// profiles via PUT /api/profile/switch, a subsequent GET /api/config/html
// (without stale form params) returns the new profile's provider and model.
// This is the regression test for GitHub issue #28.
func TestProfileSwitch_ConfigHTMLReflectsNewProfile(t *testing.T) {
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

	// Switch to "local" profile — response now contains the re-rendered form.
	body := `{"profile":"local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("switch: expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	html := w.Body.String()

	// The rendered form should reflect the "local" profile's provider (openai).
	if !strings.Contains(html, `value="openai"`) {
		t.Fatal("expected config form to contain openai provider after switching to local profile")
	}
	// Should contain the local profile's model ID.
	if !strings.Contains(html, "translategemma") {
		t.Fatal("expected config form to contain model ID 'translategemma' from local profile")
	}
	// Should NOT contain bedrock-specific fields (AWS Region input).
	if strings.Contains(html, "aws_region") {
		t.Fatal("expected config form to NOT contain bedrock fields after switching to openai profile")
	}
}

// TestProfileSwitch_StaleProviderParamIgnored verifies that if a GET
// /api/config/html request includes a stale provider query param (simulating
// the old broken behavior), the server-side config still wins when no
// provider param is sent. This ensures the fix for #28 is robust.
func TestProfileSwitch_StaleProviderParamIgnored(t *testing.T) {
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

	// Switch to "local" (openai).
	body := `{"profile":"local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("switch: expected 200, got %d", w.Code)
	}

	// Simulate the OLD broken behavior: GET with stale provider=bedrock param.
	req = httptest.NewRequest(http.MethodGet, "/api/config/html?provider=bedrock", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("config/html: expected 200, got %d", w.Code)
	}

	html := w.Body.String()

	// The provider override IS expected to work (it's used by the provider
	// <select> onchange for field preview). So bedrock fields should appear.
	// This test documents that the provider param override is intentional
	// for the provider selector — the fix is that profile switching no
	// longer sends this param.
	if !strings.Contains(html, "aws_region") {
		t.Fatal("expected provider query param to still work for provider selector preview")
	}
}

// TestListDatabases_ReturnsDBFiles tests GET /api/databases returns .db files.
func TestListDatabases_ReturnsDBFiles(t *testing.T) {
	// Create a temp dir with some .db files and non-.db files.
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	// Create test files.
	for _, name := range []string{"vocabgen.db", "vocabgen-dev.db", "config.yaml", "notes.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("test"), 0o644); err != nil {
			t.Fatalf("create test file %s: %v", name, err)
		}
	}

	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var databases []string
	if err := json.NewDecoder(w.Body).Decode(&databases); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	// Should only contain .db files.
	if len(databases) != 2 {
		t.Fatalf("expected 2 databases, got %d: %v", len(databases), databases)
	}
	hasVocabgen := false
	hasVocabgenDev := false
	for _, db := range databases {
		if db == "vocabgen.db" {
			hasVocabgen = true
		}
		if db == "vocabgen-dev.db" {
			hasVocabgenDev = true
		}
	}
	if !hasVocabgen || !hasVocabgenDev {
		t.Fatalf("expected vocabgen.db and vocabgen-dev.db, got %v", databases)
	}
}

// TestListDatabases_EmptyDir tests GET /api/databases with no .db files.
func TestListDatabases_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var databases []string
	if err := json.NewDecoder(w.Body).Decode(&databases); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	if len(databases) != 0 {
		t.Fatalf("expected empty list, got %v", databases)
	}
}

// TestListDatabases_SkipsDirectories tests that directories ending in .db are not listed.
func TestListDatabases_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	// Create a directory named "fake.db" and a real file "real.db".
	if err := os.Mkdir(dir+"/fake.db", 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(dir+"/real.db", []byte("test"), 0o644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var databases []string
	if err := json.NewDecoder(w.Body).Decode(&databases); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	if len(databases) != 1 || databases[0] != "real.db" {
		t.Fatalf("expected [real.db], got %v", databases)
	}
}

// TestPutConfig_NewDBName_Valid tests that a valid new DB name is accepted.
func TestPutConfig_NewDBName_Valid(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	t.Setenv("AWS_PROFILE", "test")

	srv := newTestServer()
	form := "provider=bedrock&aws_region=us-east-1&default_source_language=nl&default_target_language=hu&db_path=&db_path_new_name=my-new-course"
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Configuration saved") {
		t.Fatalf("expected success message, got: %s", w.Body.String())
	}
	// Verify the db_path was set to the new file path.
	if !strings.Contains(srv.cfg.DBPath, "my-new-course.db") {
		t.Fatalf("expected db_path to contain 'my-new-course.db', got %q", srv.cfg.DBPath)
	}
	// Verify the .db file was actually created on disk.
	if _, err := os.Stat(srv.cfg.DBPath); os.IsNotExist(err) {
		t.Fatalf("expected database file to be created at %q, but it does not exist", srv.cfg.DBPath)
	}
}

// TestPutConfig_NewDBName_Conflict tests that a conflicting new DB name is rejected.
func TestPutConfig_NewDBName_Conflict(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	// Create an existing database file.
	if err := os.WriteFile(dir+"/existing.db", []byte("data"), 0o644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	t.Setenv("AWS_PROFILE", "test")

	srv := newTestServer()
	form := "provider=bedrock&aws_region=us-east-1&default_source_language=nl&default_target_language=hu&db_path=&db_path_new_name=existing"
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already exists") {
		t.Fatalf("expected conflict error, got: %s", w.Body.String())
	}
}

// TestPutConfig_NewDBName_Invalid tests that an invalid new DB name is rejected.
func TestPutConfig_NewDBName_Invalid(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	t.Setenv("AWS_PROFILE", "test")

	tests := []struct {
		name    string
		dbName  string
		wantErr string
	}{
		{"spaces", "my course", "Invalid database name"},
		{"special chars", "db@home!", "Invalid database name"},
		{"path traversal", "../evil", "Invalid database name"},
		{"slash", "path/to/db", "Invalid database name"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer()
			form := "provider=bedrock&aws_region=us-east-1&default_source_language=nl&default_target_language=hu&db_path=&db_path_new_name=" + tc.dbName
			req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if !strings.Contains(w.Body.String(), tc.wantErr) {
				t.Fatalf("expected %q in response, got: %s", tc.wantErr, w.Body.String())
			}
		})
	}
}

// TestPutConfig_SelectExistingDB tests that selecting an existing DB from the dropdown works.
func TestPutConfig_SelectExistingDB(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	t.Setenv("AWS_PROFILE", "test")

	srv := newTestServer()
	form := "provider=bedrock&aws_region=us-east-1&default_source_language=nl&default_target_language=hu&db_path=" + dir + "/vocabgen-dev.db"
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Configuration saved") {
		t.Fatalf("expected success message, got: %s", w.Body.String())
	}
	if srv.cfg.DBPath != dir+"/vocabgen-dev.db" {
		t.Fatalf("expected db_path %q, got %q", dir+"/vocabgen-dev.db", srv.cfg.DBPath)
	}
}

// TestIsValidDBName tests the isValidDBName helper function.
func TestIsValidDBName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple name", "vocabgen", true},
		{"with hyphen", "my-course", true},
		{"with underscore", "my_course", true},
		{"with digits", "course2024", true},
		{"with .db suffix", "vocabgen.db", true},
		{"empty", "", false},
		{"just .db", ".db", false},
		{"spaces", "my course", false},
		{"special chars", "db@home", false},
		{"path separator", "path/db", false},
		{"dots in name", "my.course", false},
		{"backslash", "path\\db", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidDBName(tc.input)
			if got != tc.want {
				t.Fatalf("isValidDBName(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestPutConfig_NewDBPath_FallbackWhenNew tests that db_path="__new__" without a name falls back to current.
func TestPutConfig_NewDBPath_FallbackWhenNew(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	t.Setenv("AWS_PROFILE", "test")

	srv := newTestServer()
	originalDBPath := srv.cfg.DBPath

	// Send __new__ as db_path but no db_path_new_name — should fall back to current.
	form := "provider=bedrock&aws_region=us-east-1&default_source_language=nl&default_target_language=hu&db_path=__new__&db_path_new_name="
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if srv.cfg.DBPath != originalDBPath {
		t.Fatalf("expected db_path to remain %q, got %q", originalDBPath, srv.cfg.DBPath)
	}
}

// TestSwitchProfile_IncludesDatabases tests that profile switch re-renders with database picker.
func TestSwitchProfile_IncludesDatabases(t *testing.T) {
	dir := t.TempDir()
	config.SetConfigDirForTest(dir)
	t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

	// Create a .db file so it shows in the picker.
	if err := os.WriteFile(dir+"/vocabgen.db", []byte{}, 0o644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Write a multi-profile config.
	fc := config.FileConfig{
		DefaultProfile: "default",
		Profiles: map[string]config.ProfileConfig{
			"default": {Provider: "bedrock", AWSRegion: "us-east-1"},
			"local":   {Provider: "openai", BaseURL: "http://localhost:11434/v1", ModelID: "test"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                dir + "/vocabgen.db",
	}
	if err := config.SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	srv := newTestServer()
	srv.cfg.DBPath = dir + "/vocabgen.db"

	form := "profile=local"
	req := httptest.NewRequest(http.MethodPut, "/api/profile/switch", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	// The response should contain the database picker with "Create new" option.
	body := w.Body.String()
	if !strings.Contains(body, "Create new") {
		t.Fatalf("expected 'Create new' in profile switch response, got: %s", body)
	}
	if !strings.Contains(body, "Save") {
		t.Fatalf("expected 'Save' button in profile switch response, got: %s", body)
	}
}
