package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/config"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// 1. TestWriteLocalLLMConfig — table-driven tests for writeLocalLLMConfig
// ---------------------------------------------------------------------------

// TestWriteLocalLLMConfig verifies that writeLocalLLMConfig creates a "local"
// profile with the correct provider, base_url, and model_id values.
//
// Validates: Requirements 59.7, 59.8
func TestWriteLocalLLMConfig(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) // optional pre-existing config
		model        string
		wantProvider string
		wantBaseURL  string
		wantModelID  string
		wantDefault  string
		wantProfiles []string // expected profile names (sorted)
	}{
		{
			name:         "fresh config with no existing file",
			model:        "translategemma",
			wantProvider: "openai",
			wantBaseURL:  "http://localhost:11434/v1",
			wantModelID:  "translategemma",
			wantDefault:  "local",
			wantProfiles: []string{"default", "local"},
		},
		{
			name:  "existing multi-profile config preserves other profiles",
			model: "translategemma",
			setup: func(t *testing.T) {
				t.Helper()
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
			},
			wantProvider: "openai",
			wantBaseURL:  "http://localhost:11434/v1",
			wantModelID:  "translategemma",
			wantDefault:  "local",
			wantProfiles: []string{"local", "prod"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config.SetConfigDirForTest(t.TempDir())
			t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

			if tc.setup != nil {
				tc.setup(t)
			}

			if err := writeLocalLLMConfig(tc.model); err != nil {
				t.Fatalf("writeLocalLLMConfig(%q): %v", tc.model, err)
			}

			// Load the local profile and verify values.
			cfg, err := config.LoadConfigWithProfile("local")
			if err != nil {
				t.Fatalf("LoadConfigWithProfile(local): %v", err)
			}
			if cfg.Provider != tc.wantProvider {
				t.Fatalf("provider: got %q, want %q", cfg.Provider, tc.wantProvider)
			}
			if cfg.BaseURL != tc.wantBaseURL {
				t.Fatalf("base_url: got %q, want %q", cfg.BaseURL, tc.wantBaseURL)
			}
			if cfg.ModelID != tc.wantModelID {
				t.Fatalf("model_id: got %q, want %q", cfg.ModelID, tc.wantModelID)
			}

			// Verify default_profile is "local".
			profiles, def, err := config.ListProfiles()
			if err != nil {
				t.Fatalf("ListProfiles: %v", err)
			}
			if def != tc.wantDefault {
				t.Fatalf("default_profile: got %q, want %q", def, tc.wantDefault)
			}

			// Verify expected profiles exist.
			profileSet := make(map[string]bool, len(profiles))
			for _, p := range profiles {
				profileSet[p] = true
			}
			for _, want := range tc.wantProfiles {
				if !profileSet[want] {
					t.Fatalf("expected profile %q in %v", want, profiles)
				}
			}
			if len(profiles) != len(tc.wantProfiles) {
				t.Fatalf("expected %d profiles, got %d: %v", len(tc.wantProfiles), len(profiles), profiles)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. TestLoadExistingFileConfig — table-driven tests
// ---------------------------------------------------------------------------

// TestLoadExistingFileConfig verifies that loadExistingFileConfig correctly
// handles no config, flat config, and multi-profile config scenarios.
//
// Validates: Requirements 59.7, 58.5
func TestLoadExistingFileConfig(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(t *testing.T)
		wantProfileCount int
		wantSrcLang      string
		wantTgtLang      string
	}{
		{
			name:             "no config file returns defaults",
			wantProfileCount: 1, // "default" profile from ListProfiles
			wantSrcLang:      "nl",
			wantTgtLang:      "hu",
		},
		{
			name: "existing flat config converts correctly",
			setup: func(t *testing.T) {
				t.Helper()
				cfg := config.Config{
					Provider:              "openai",
					ModelID:               "gpt-4o",
					DefaultSourceLanguage: "de",
					DefaultTargetLanguage: "en",
					DBPath:                "~/.vocabgen/vocabgen.db",
				}
				data, err := yaml.Marshal(cfg)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				dir := filepath.Join(t.TempDir(), "dummy") // won't be used
				_ = dir
				// Write directly to the config dir
				cfgPath := config.FilePath()
				_ = os.MkdirAll(filepath.Dir(cfgPath), 0o755)
				if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
					t.Fatalf("write flat config: %v", err)
				}
			},
			wantProfileCount: 1, // "default" profile
			wantSrcLang:      "de",
			wantTgtLang:      "en",
		},
		{
			name: "existing multi-profile config preserves all profiles",
			setup: func(t *testing.T) {
				t.Helper()
				fc := config.FileConfig{
					DefaultProfile: "prod",
					Profiles: map[string]config.ProfileConfig{
						"prod":    {Provider: "bedrock", AWSRegion: "us-east-1"},
						"sandbox": {Provider: "anthropic", ModelID: "claude-sonnet-4-20250514"},
					},
					DefaultSourceLanguage: "it",
					DefaultTargetLanguage: "hu",
					DBPath:                "~/.vocabgen/vocabgen.db",
				}
				if err := config.SaveFileConfig(fc); err != nil {
					t.Fatalf("SaveFileConfig: %v", err)
				}
			},
			wantProfileCount: 2,
			wantSrcLang:      "it",
			wantTgtLang:      "hu",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config.SetConfigDirForTest(t.TempDir())
			t.Cleanup(func() { config.SetConfigDirForTest(os.TempDir()) })

			if tc.setup != nil {
				tc.setup(t)
			}

			fc, err := loadExistingFileConfig()
			if err != nil {
				t.Fatalf("loadExistingFileConfig: %v", err)
			}

			if len(fc.Profiles) != tc.wantProfileCount {
				t.Fatalf("profile count: got %d, want %d (profiles: %v)", len(fc.Profiles), tc.wantProfileCount, fc.Profiles)
			}
			if fc.DefaultSourceLanguage != tc.wantSrcLang {
				t.Fatalf("DefaultSourceLanguage: got %q, want %q", fc.DefaultSourceLanguage, tc.wantSrcLang)
			}
			if fc.DefaultTargetLanguage != tc.wantTgtLang {
				t.Fatalf("DefaultTargetLanguage: got %q, want %q", fc.DefaultTargetLanguage, tc.wantTgtLang)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. TestValidateProviderEnvOllama — Ollama-specific validation
// ---------------------------------------------------------------------------

// TestValidateProviderEnvOllama verifies that validateProviderEnv handles
// Ollama base URLs correctly: skipping API key checks when a base URL is set.
//
// Validates: Requirements 59.11
func TestValidateProviderEnvOllama(t *testing.T) {
	envKeys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_ACCESS_KEY_ID", "AWS_PROFILE", "AWS_SESSION_TOKEN", "GCP_PROJECT"}

	tests := []struct {
		name       string
		provider   string
		baseURL    string
		envVars    map[string]string
		wantEmpty  bool   // true = no warning expected
		wantSubstr string // expected substring in warning (if any)
		skipIf     bool
	}{
		{
			name:       "openai + localhost:11434 base URL: checks Ollama reachability (not API key)",
			provider:   "openai",
			baseURL:    "http://localhost:11434/v1",
			wantSubstr: "Ollama server is not reachable",
			skipIf:     ollamaRunning(),
		},
		{
			name:      "openai + non-Ollama base URL: skips API key check",
			provider:  "openai",
			baseURL:   "http://my-server:8080/v1",
			wantEmpty: true,
		},
		{
			name:       "openai + no base URL + no API key: returns warning",
			provider:   "openai",
			baseURL:    "",
			wantSubstr: "OPENAI_API_KEY",
		},
		{
			name:       "openai + localhost:11434 + API key set: checks Ollama reachability",
			provider:   "openai",
			baseURL:    "http://localhost:11434/v1",
			envVars:    map[string]string{"OPENAI_API_KEY": "sk-test"},
			wantSubstr: "Ollama server is not reachable",
			skipIf:     ollamaRunning(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipIf {
				t.Skip("skipped: Ollama is running locally")
			}
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			got := validateProviderEnv(tc.provider, tc.baseURL, "")
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

// ---------------------------------------------------------------------------
// 4. TestPropertyP23_SetupProducesValidConfig — property test with rapid
// ---------------------------------------------------------------------------

// TestPropertyP23_SetupProducesValidConfig verifies that for any non-empty
// model name, writeLocalLLMConfig always produces a config where provider,
// base_url, and model_id are all non-empty.
//
// **Property 23: Local LLM setup produces valid config**
// **Validates: Requirement 59.7**
func TestPropertyP23_SetupProducesValidConfig(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random non-empty model name.
		model := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9._:-]{0,30}`).Draw(rt, "model")

		config.SetConfigDirForTest(t.TempDir())
		defer config.SetConfigDirForTest(os.TempDir())

		if err := writeLocalLLMConfig(model); err != nil {
			rt.Fatalf("writeLocalLLMConfig(%q): %v", model, err)
		}

		// Load the resulting config.
		cfg, err := config.LoadConfigWithProfile("local")
		if err != nil {
			rt.Fatalf("LoadConfigWithProfile(local): %v", err)
		}

		// Assert provider, base_url, model_id are all non-empty.
		if cfg.Provider == "" {
			rt.Fatal("provider is empty after setup")
		}
		if cfg.BaseURL == "" {
			rt.Fatal("base_url is empty after setup")
		}
		if cfg.ModelID == "" {
			rt.Fatalf("model_id is empty after setup (input model=%q)", model)
		}

		// Verify specific expected values.
		if cfg.Provider != "openai" {
			rt.Fatalf("provider: got %q, want 'openai'", cfg.Provider)
		}
		if cfg.BaseURL != "http://localhost:11434/v1" {
			rt.Fatalf("base_url: got %q, want 'http://localhost:11434/v1'", cfg.BaseURL)
		}
		if cfg.ModelID != model {
			rt.Fatalf("model_id: got %q, want %q", cfg.ModelID, model)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. TestHandleSetupLocalLLM_RouteRegistered — verify route exists
// ---------------------------------------------------------------------------

// TestHandleSetupLocalLLM_RouteRegistered verifies that GET /api/setup/local-llm
// is registered and does not return 404.
//
// Validates: Requirements 59.1, 59.2
func TestHandleSetupLocalLLM_RouteRegistered(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/setup/local-llm", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// The route should be registered — it won't return 404 or 405.
	// It will likely fail because Ollama isn't running, but the route exists.
	if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/setup/local-llm route not registered: got %d", w.Code)
	}
}
