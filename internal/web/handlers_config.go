package web

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/user/vocabgen/internal/config"
	"github.com/user/vocabgen/internal/service"
)

// handleGetConfig handles GET /api/config — return current config as JSON.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg)
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

	if err := config.SaveConfig(updated); err != nil {
		s.logger.Error("save config failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	// Update in-memory config
	*s.cfg = updated

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<p class="text-green-600 text-sm mt-1">Configuration saved.</p>`))
}

// handleConfigHTML handles GET /api/config/html — render config form partial.
func (s *Server) handleConfigHTML(w http.ResponseWriter, r *http.Request) {
	// If provider query param is set, use it for conditional fields preview
	cfg := *s.cfg
	if p := r.URL.Query().Get("provider"); p != "" {
		cfg.Provider = p
	}
	// Also check form values (from hx-include)
	if p := r.FormValue("provider"); p != "" {
		cfg.Provider = p
	}

	data := struct {
		Config    *config.Config
		Languages []service.LanguageInfo
	}{
		Config:    &cfg,
		Languages: service.GetSupportedLanguages(),
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
