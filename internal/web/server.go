// Package web implements the embedded HTTP server and web UI handlers.
package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/user/vocabgen/internal/config"
	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/llm"
	"github.com/user/vocabgen/internal/service"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	store     db.Store
	cfg       *config.Config
	mux       *http.ServeMux
	logger    *slog.Logger
	version   string
	buildDate string
	goVersion string
}

// pageData is the common data passed to all page templates.
type pageData struct {
	ActivePage string
	Languages  []service.LanguageInfo
	Config     *config.Config
	Version    string
	BuildDate  string
	GoVersion  string
}

// NewServer creates a Server with all routes registered.
func NewServer(store db.Store, cfg *config.Config, logger *slog.Logger, version, buildDate, goVersion string) *Server {
	s := &Server{
		store:     store,
		cfg:       cfg,
		mux:       http.NewServeMux(),
		logger:    logger,
		version:   version,
		buildDate: buildDate,
		goVersion: goVersion,
	}
	s.registerRoutes()
	return s
}

// ListenAndServe starts the HTTP server. Blocks until ctx is cancelled,
// then performs graceful shutdown with a 5-second timeout.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("web server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down web server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// registerRoutes wires all page and API routes onto the mux.
func (s *Server) registerRoutes() {
	// Page routes (serve HTML)
	s.mux.HandleFunc("GET /", s.handlePage("lookup"))
	s.mux.HandleFunc("GET /batch", s.handlePage("batch"))
	s.mux.HandleFunc("GET /config", s.handlePage("config"))
	s.mux.HandleFunc("GET /database", s.handlePage("database"))
	s.mux.HandleFunc("GET /about", s.handlePage("about"))

	// Lookup API
	s.mux.HandleFunc("POST /api/lookup", s.handleLookupJSON)
	s.mux.HandleFunc("POST /api/lookup/html", s.handleLookupHTML)
	s.mux.HandleFunc("POST /api/lookup/resolve", s.handleLookupResolveJSON)
	s.mux.HandleFunc("POST /api/lookup/resolve/html", s.handleLookupResolveHTML)

	// Batch API
	s.mux.HandleFunc("POST /api/batch", s.handleBatchJSON)
	s.mux.HandleFunc("POST /api/batch/html", s.handleBatchHTML)
	s.mux.HandleFunc("GET /api/batch/stream", s.handleBatchStream)

	// Config API
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	s.mux.HandleFunc("GET /api/config/html", s.handleConfigHTML)
	s.mux.HandleFunc("POST /api/test-connection", s.handleTestConnection)

	// Database API
	s.mux.HandleFunc("GET /api/words", s.handleListWords)
	s.mux.HandleFunc("GET /api/expressions", s.handleListExpressions)
	s.mux.HandleFunc("GET /api/words/{id}/edit", s.handleEditWord)
	s.mux.HandleFunc("GET /api/expressions/{id}/edit", s.handleEditExpression)
	s.mux.HandleFunc("PUT /api/words/{id}", s.handleUpdateWord)
	s.mux.HandleFunc("PUT /api/expressions/{id}", s.handleUpdateExpression)
	s.mux.HandleFunc("DELETE /api/words/{id}", s.handleDeleteWord)
	s.mux.HandleFunc("DELETE /api/expressions/{id}", s.handleDeleteExpression)
	s.mux.HandleFunc("POST /api/import", s.handleImport)
	s.mux.HandleFunc("GET /api/export", s.handleExport)

	// Health & Languages
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/languages", s.handleLanguages)
}

// handlePage returns a handler that renders a full page template.
func (s *Server) handlePage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := pageData{
			ActivePage: name,
			Languages:  service.GetSupportedLanguages(),
			Config:     s.cfg,
			Version:    s.version,
			BuildDate:  s.buildDate,
			GoVersion:  s.goVersion,
		}
		if err := renderPage(w, name, data); err != nil {
			s.logger.Error("render page failed", "page", name, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleLanguages returns the supported languages list as JSON.
func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	langs := service.GetSupportedLanguages()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(langs)
}

// createProvider creates an LLM provider from the current config.
// apiKey is optional and used for OpenAI/Anthropic providers.
func (s *Server) createProvider(apiKey string) (llm.Provider, error) {
	constructor, ok := llm.Registry[s.cfg.Provider]
	if !ok {
		return nil, &llm.ProviderError{Provider: s.cfg.Provider, Message: "unknown provider"}
	}
	return constructor(llm.ProviderOptions{
		APIKey:     apiKey,
		BaseURL:    s.cfg.BaseURL,
		Region:     s.cfg.AWSRegion,
		Profile:    s.cfg.AWSProfile,
		GCPProject: s.cfg.GCPProject,
	})
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"detail": msg})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// decodeJSONBody decodes a JSON request body into v.
func decodeJSONBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
