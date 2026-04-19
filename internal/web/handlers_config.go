package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/user/vocabgen/internal/config"
	"github.com/user/vocabgen/internal/service"
)

// handleGetConfig handles GET /api/config — return current config as JSON.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg)
}

// handleGetProfiles handles GET /api/profiles — return available profiles and active profile.
func (s *Server) handleGetProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, defaultProfile, err := config.ListProfiles()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list profiles: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profiles": profiles,
		"active":   defaultProfile,
	})
}

// handleSwitchProfile handles PUT /api/profile/switch — switch active profile.
func (s *Server) handleSwitchProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile string `json:"profile"`
		Source  string `json:"source"`
	}

	ct := r.Header.Get("Content-Type")
	if ct == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form: "+err.Error())
			return
		}
		req.Profile = r.FormValue("profile")
		req.Source = r.FormValue("source")
	}

	if req.Profile == "" {
		writeJSONError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	cfg, err := config.LoadConfigWithProfile(req.Profile)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	*s.cfg = cfg
	s.activeProfile = req.Profile
	s.logger.Info("switched config profile", "profile", req.Profile)

	// When switched from the nav-bar profile switcher, return just the
	// updated switcher partial so HTMX can swap it in-place.
	if req.Source == "nav" {
		profiles, _, _ := config.ListProfiles()
		data := struct {
			ActiveProfile string
			Profiles      []string
		}{
			ActiveProfile: s.activeProfile,
			Profiles:      profiles,
		}
		_ = renderPartial(w, "profile_switcher", data)
		return
	}

	// Re-render the full config form with the new profile's values.
	// This replaces the two-step approach (PUT then GET) that was prone to
	// stale form values leaking into the re-render request (#28).
	profiles, _, _ := config.ListProfiles()
	data := struct {
		Config        *config.Config
		Languages     []service.LanguageInfo
		Profiles      []string
		ActiveProfile string
		StatusMessage template.HTML
	}{
		Config:        &cfg,
		Languages:     service.GetSupportedLanguages(),
		Profiles:      profiles,
		ActiveProfile: s.activeProfile,
		StatusMessage: template.HTML(`<p class="text-green-600 text-sm mt-1">Switched to profile "` + req.Profile + `".</p>`),
	}
	_ = renderPartial(w, "config_form", data)
}

// handleCreateProfile handles POST /api/profiles — create a new profile by copying an existing one.
func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<p class="text-red-600 text-sm mt-1">Invalid form data.</p>`))
		return
	}

	name := r.FormValue("name")
	sourceProfile := r.FormValue("source_profile")

	if name == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<p class="text-red-600 text-sm mt-1">Profile name is required.</p>`))
		return
	}

	if err := config.CreateProfile(name, sourceProfile); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<p class="text-red-600 text-sm mt-1">` + err.Error() + `</p>`))
		return
	}

	// Reload in-memory config with the new profile.
	cfg, err := config.LoadConfigWithProfile(name)
	if err != nil {
		s.logger.Error("reload config after profile creation failed", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<p class="text-red-600 text-sm mt-1">Profile created but failed to reload: ` + err.Error() + `</p>`))
		return
	}
	*s.cfg = cfg
	s.activeProfile = name
	s.logger.Info("created config profile", "profile", name, "source", sourceProfile)

	// Return an HX-Trigger header to reload the config form.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("HX-Trigger", "profileCreated")
	_, _ = w.Write([]byte(`<p class="text-green-600 text-sm mt-1">Profile "` + name + `" created.</p>`))
}

// handlePutConfig handles PUT /api/config — update config file.
func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var updated config.Config

	ct := r.Header.Get("Content-Type")
	if ct == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	} else {
		// Form-encoded (HTMX)
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form: "+err.Error())
			return
		}
		updated = config.Config{
			Provider:              r.FormValue("provider"),
			AWSProfile:            r.FormValue("aws_profile"),
			AWSRegion:             r.FormValue("aws_region"),
			ModelID:               r.FormValue("model_id"),
			BaseURL:               r.FormValue("base_url"),
			GCPProject:            r.FormValue("gcp_project"),
			GCPRegion:             r.FormValue("gcp_region"),
			DefaultSourceLanguage: r.FormValue("default_source_language"),
			DefaultTargetLanguage: r.FormValue("default_target_language"),
			DBPath:                s.cfg.DBPath, // preserve DB path
		}
	}

	// Apply defaults for empty required fields
	if updated.Provider == "" {
		updated.Provider = "bedrock"
	}
	if updated.DBPath == "" {
		updated.DBPath = s.cfg.DBPath
	}

	// Validate provider credentials are available via environment.
	if warning := validateProviderEnv(updated.Provider, updated.BaseURL, updated.GCPProject); warning != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<p class="text-red-600 text-sm mt-1">` + warning + `</p>`))
		return
	}

	if err := config.SaveConfig(updated, s.activeProfile); err != nil {
		s.logger.Error("save config failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	// Update in-memory config
	*s.cfg = updated

	s.logger.Info("config saved", "profile", s.activeProfile, "provider", updated.Provider)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<p class="text-green-600 text-sm mt-1">Configuration saved to profile "` + s.activeProfile + `".</p>`))
}

// handleConfigHTML handles GET /api/config/html — render config form partial.
func (s *Server) handleConfigHTML(w http.ResponseWriter, r *http.Request) {
	// If provider query param is set, use it for conditional fields preview
	cfg := *s.cfg
	origProvider := cfg.Provider
	if p := r.URL.Query().Get("provider"); p != "" {
		cfg.Provider = p
	}
	// Also check form values (from hx-include)
	if p := r.FormValue("provider"); p != "" {
		cfg.Provider = p
	}
	// Clear model ID when provider changes so the placeholder hint shows.
	if cfg.Provider != origProvider {
		cfg.ModelID = ""
	}

	profiles, _, _ := config.ListProfiles()

	data := struct {
		Config        *config.Config
		Languages     []service.LanguageInfo
		Profiles      []string
		ActiveProfile string
		StatusMessage template.HTML
	}{
		Config:        &cfg,
		Languages:     service.GetSupportedLanguages(),
		Profiles:      profiles,
		ActiveProfile: s.activeProfile,
	}
	_ = renderPartial(w, "config_form", data)
}

// handleTestConnection handles POST /api/test-connection.
func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	provider, err := s.createProvider()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<div class="bg-red-50 border border-red-200 text-red-700 rounded p-3 text-sm">` +
			"Failed to create provider: " + err.Error() + `</div>`))
		return
	}

	// Try a minimal invocation to test the connection
	_, err = provider.Invoke(r.Context(), "Say hello in one word.", s.cfg.ModelID)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<div class="bg-red-50 border border-red-200 text-red-700 rounded p-3 text-sm">` +
			"Connection failed: " + err.Error() + `</div>`))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<div class="bg-green-50 border border-green-200 text-green-700 rounded p-3 text-sm">` +
		"Connection successful! Provider: " + provider.Name() + `</div>`))
}

// validateProviderEnv checks that the required environment variables or
// credentials are available for the given provider. Returns an empty string
// if everything looks good, or a user-facing warning message otherwise.
func validateProviderEnv(provider, baseURL, gcpProject string) string {
	switch provider {
	case "openai":
		if os.Getenv("OPENAI_API_KEY") == "" && baseURL == "" {
			return "OPENAI_API_KEY environment variable is not set. Set it before starting the server: export OPENAI_API_KEY=sk-..."
		}
		// When base URL points to a local Ollama server, verify it's reachable.
		if strings.Contains(baseURL, "localhost:11434") {
			if msg := checkOllamaReachable(); msg != "" {
				return msg
			}
		}
	case "anthropic":
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return "ANTHROPIC_API_KEY environment variable is not set. Set it before starting the server: export ANTHROPIC_API_KEY=sk-ant-..."
		}
	case "bedrock":
		// Lightweight check: look for any common AWS credential source.
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" && os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_SESSION_TOKEN") == "" {
			home, _ := os.UserHomeDir()
			if home != "" {
				if _, err := os.Stat(home + "/.aws/credentials"); err != nil {
					return "No AWS credentials found. Set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY environment variables, configure an AWS profile, or use an IAM role."
				}
			}
		}
	case "vertexai":
		if gcpProject == "" && os.Getenv("GCP_PROJECT") == "" {
			return "GCP project ID is required for Vertex AI. Set the GCP_PROJECT environment variable or fill in the GCP Project field."
		}
	}
	return ""
}

// checkOllamaReachable performs a lightweight HTTP check against the local
// Ollama server. Returns an empty string if reachable, or a user-facing
// warning message if not.
func checkOllamaReachable() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return "Ollama server is not reachable at localhost:11434. Start it with: ollama serve"
	}
	defer func() { _ = resp.Body.Close() }()
	return ""
}

// handleProfileSwitcherPartial renders the profile switcher partial for the nav bar.
// Used by HTMX to refresh the switcher after a profile switch.
func (s *Server) handleProfileSwitcherPartial(w http.ResponseWriter, _ *http.Request) {
	profiles, _, _ := config.ListProfiles()
	data := struct {
		ActiveProfile string
		Profiles      []string
	}{
		ActiveProfile: s.activeProfile,
		Profiles:      profiles,
	}
	_ = renderPartial(w, "profile_switcher", data)
}
