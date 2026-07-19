package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// Middleware returns an http.Handler that enforces API-key authentication
// on /api/* routes (except /api/health) when cfg is non-nil.
// When cfg is nil, all requests pass through unchanged (open access).
//
// Authentication rules:
//   - GET /api/health is always exempt (Docker HEALTHCHECK, monitoring).
//   - HTMX requests (HX-Request: true) are exempt — they originate from the web UI.
//   - Same-origin browser fetches (Sec-Fetch-Site: same-origin) are exempt.
//   - Non-API routes (HTML pages) pass through unauthenticated in Phase 1.
//   - Valid Bearer token → request proceeds.
//   - Invalid/missing Bearer token → 401 JSON.
//   - Valid token but write method (POST/PUT/DELETE) on a read-only account → 403 JSON.
func Middleware(cfg *UsersConfig, logger *slog.Logger, next http.Handler) http.Handler {
	// No config → open access, skip all auth logic.
	if cfg == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Only protect /api/* routes (except health).
		if !strings.HasPrefix(path, "/api/") || path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Same-origin browser requests come from the web UI — exempt them.
		// This covers both HTMX calls (HX-Request: true) and plain fetch()
		// from our templates. External callers (curl, MCP server) won't have
		// these headers. Sec-Fetch-Site is set by all modern browsers.
		if r.Header.Get("HX-Request") == "true" || r.Header.Get("Sec-Fetch-Site") == "same-origin" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract Bearer token.
		token := extractBearerToken(r)
		if token == "" {
			logger.Warn("auth: missing or malformed Authorization header",
				"path", path, "method", r.Method, "remote", r.RemoteAddr)
			writeAuthError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		// Validate key against configured service accounts.
		account := ValidateAPIKey(cfg, token)
		if account == nil {
			logger.Warn("auth: invalid API key",
				"path", path, "method", r.Method, "remote", r.RemoteAddr)
			writeAuthError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		// Scope enforcement: read-only accounts can only use GET.
		if account.Scope == "read-only" && r.Method != http.MethodGet {
			logger.Warn("auth: write attempt by read-only account",
				"account", account.Name, "path", path, "method", r.Method)
			writeAuthError(w, http.StatusForbidden, "read-only access: write operations are not permitted")
			return
		}

		// Authenticated — proceed.
		next.ServeHTTP(w, r)
	})
}

// extractBearerToken parses the Authorization header for a Bearer token.
// Returns empty string if the header is absent or malformed.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

// writeAuthError writes a JSON error response for authentication failures.
func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": msg})
}
