package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"pgregory.net/rapid"
)

// printableASCII covers runes 0x20–0x7E (space through tilde).
var printableASCII = &unicode.RangeTable{
	R16: []unicode.Range16{{Lo: 0x0020, Hi: 0x007E, Stride: 1}},
}

// lowercaseASCII covers runes 'a'–'z'.
var lowercaseASCII = &unicode.RangeTable{
	R16: []unicode.Range16{{Lo: 'a', Hi: 'z', Stride: 1}},
}

// testLogger returns a discarding logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// hashKey is a test helper that generates a bcrypt hash for the given key.
func hashKey(t *testing.T, key string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashKey: %v", err)
	}
	return string(h)
}

// --- LoadUsersConfig tests ---

func TestLoadUsersConfig(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantNil   bool
		wantErr   bool
		wantCount int
	}{
		{
			name:    "file does not exist → nil config, no error",
			content: "", // special case: skip file creation
			wantNil: true,
			wantErr: false,
		},
		{
			name: "valid config with one account",
			content: `service_accounts:
  - name: mcp-server
    key_hash: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
    scope: read-only
`,
			wantNil:   false,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "scope defaults to read-only when omitted",
			content: `service_accounts:
  - name: automation
    key_hash: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
`,
			wantNil:   false,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "missing name → error",
			content: `service_accounts:
  - key_hash: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
`,
			wantErr: true,
		},
		{
			name: "missing key_hash → error",
			content: `service_accounts:
  - name: broken
`,
			wantErr: true,
		},
		{
			name:    "invalid YAML → error",
			content: `[[[not yaml`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "users.yaml")

			if tt.name == "file does not exist → nil config, no error" {
				// Don't create the file
			} else {
				if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
					t.Fatalf("write test file: %v", err)
				}
			}

			cfg, err := LoadUsersConfig(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if cfg != nil {
					t.Fatalf("expected nil config, got %+v", cfg)
				}
				return
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
				return
			}
			if len(cfg.ServiceAccounts) != tt.wantCount {
				t.Fatalf("expected %d accounts, got %d", tt.wantCount, len(cfg.ServiceAccounts))
			}
		})
	}
}

// --- ValidateAPIKey tests ---

func TestValidateAPIKey(t *testing.T) {
	validKey := "test-api-key-12345"
	validHash := hashKey(t, validKey)

	cfg := &UsersConfig{
		ServiceAccounts: []ServiceAccount{
			{Name: "svc-1", KeyHash: validHash, Scope: "read-only"},
		},
	}

	tests := []struct {
		name      string
		cfg       *UsersConfig
		key       string
		wantMatch bool
		wantName  string
	}{
		{
			name:      "nil config → no match",
			cfg:       nil,
			key:       validKey,
			wantMatch: false,
		},
		{
			name:      "empty accounts → no match",
			cfg:       &UsersConfig{},
			key:       validKey,
			wantMatch: false,
		},
		{
			name:      "valid key matches",
			cfg:       cfg,
			key:       validKey,
			wantMatch: true,
			wantName:  "svc-1",
		},
		{
			name:      "wrong key → no match",
			cfg:       cfg,
			key:       "wrong-key",
			wantMatch: false,
		},
		{
			name:      "empty key → no match",
			cfg:       cfg,
			key:       "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateAPIKey(tt.cfg, tt.key)
			if tt.wantMatch {
				if result == nil {
					t.Fatal("expected match, got nil")
					return
				}
				if result.Name != tt.wantName {
					t.Fatalf("expected name %q, got %q", tt.wantName, result.Name)
				}
			} else if result != nil {
				t.Fatalf("expected no match, got %+v", result)
			}
		})
	}
}

// --- Middleware tests ---

func TestMiddleware(t *testing.T) {
	validKey := "middleware-test-key"
	validHash := hashKey(t, validKey)

	cfg := &UsersConfig{
		ServiceAccounts: []ServiceAccount{
			{Name: "reader", KeyHash: validHash, Scope: "read-only"},
		},
	}

	// A simple next handler that returns 200 OK.
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	logger := testLogger()

	tests := []struct {
		name         string
		cfg          *UsersConfig
		method       string
		path         string
		authHeader   string
		hxRequest    bool
		secFetchSite string
		wantStatus   int
	}{
		{
			name:       "nil config → open access, any request passes",
			cfg:        nil,
			method:     "GET",
			path:       "/api/words",
			authHeader: "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "health endpoint always exempt",
			cfg:        cfg,
			method:     "GET",
			path:       "/api/health",
			authHeader: "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "HTMX request exempt (web UI same-origin)",
			cfg:        cfg,
			method:     "GET",
			path:       "/api/tags",
			authHeader: "",
			hxRequest:  true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "HTMX POST request exempt (web UI form submit)",
			cfg:        cfg,
			method:     "POST",
			path:       "/api/lookup/html",
			authHeader: "",
			hxRequest:  true,
			wantStatus: http.StatusOK,
		},
		{
			name:         "same-origin fetch exempt (Sec-Fetch-Site)",
			cfg:          cfg,
			method:       "GET",
			path:         "/api/tags",
			authHeader:   "",
			secFetchSite: "same-origin",
			wantStatus:   http.StatusOK,
		},
		{
			name:       "non-API route passes through (HTML page)",
			cfg:        cfg,
			method:     "GET",
			path:       "/batch",
			authHeader: "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "API route without auth header → 401",
			cfg:        cfg,
			method:     "GET",
			path:       "/api/words",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "API route with invalid key → 401",
			cfg:        cfg,
			method:     "GET",
			path:       "/api/words",
			authHeader: "Bearer wrong-key",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "API route with malformed header → 401",
			cfg:        cfg,
			method:     "GET",
			path:       "/api/words",
			authHeader: "Basic dXNlcjpwYXNz",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid key GET → 200",
			cfg:        cfg,
			method:     "GET",
			path:       "/api/words",
			authHeader: "Bearer " + validKey,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid key POST on read-only → 403",
			cfg:        cfg,
			method:     "POST",
			path:       "/api/lookup",
			authHeader: "Bearer " + validKey,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "valid key PUT on read-only → 403",
			cfg:        cfg,
			method:     "PUT",
			path:       "/api/words/1",
			authHeader: "Bearer " + validKey,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "valid key DELETE on read-only → 403",
			cfg:        cfg,
			method:     "DELETE",
			path:       "/api/words/1",
			authHeader: "Bearer " + validKey,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Middleware(tt.cfg, logger, okHandler)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.hxRequest {
				req.Header.Set("HX-Request", "true")
			}
			if tt.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSite)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tt.wantStatus, rec.Code, rec.Body.String())
			}

			// Verify error responses are JSON.
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				ct := rec.Header().Get("Content-Type")
				if ct != "application/json; charset=utf-8" {
					t.Fatalf("expected JSON content-type, got %q", ct)
				}
				var body map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode JSON error body: %v", err)
				}
				if body["detail"] == "" {
					t.Fatal("expected non-empty 'detail' in error response")
				}
			}
		})
	}
}

// TestMiddlewareReadWriteScope verifies that accounts with "read-write" scope
// can access write endpoints.
func TestMiddlewareReadWriteScope(t *testing.T) {
	rwKey := "rw-key-123"
	rwHash := hashKey(t, rwKey)

	cfg := &UsersConfig{
		ServiceAccounts: []ServiceAccount{
			{Name: "admin-bot", KeyHash: rwHash, Scope: "read-write"},
		},
	}

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(cfg, testLogger(), okHandler)

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/words", nil)
			req.Header.Set("Authorization", "Bearer "+rwKey)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("read-write account %s: expected 200, got %d", method, rec.Code)
			}
		})
	}
}

// --- Property-based tests ---

// TestPropertyValidateAPIKeyMatchesOwnHash verifies that for all valid
// bcrypt key hashes, ValidateAPIKey returns a match with the original key.
func TestPropertyValidateAPIKeyMatchesOwnHash(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random key (1-72 bytes — bcrypt max input).
		key := rapid.StringOfN(rapid.RuneFrom(nil, printableASCII), 1, 72, -1).Draw(t, "key")

		hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("bcrypt hash: %v", err)
		}

		cfg := &UsersConfig{
			ServiceAccounts: []ServiceAccount{
				{Name: "test", KeyHash: string(hash), Scope: "read-only"},
			},
		}

		result := ValidateAPIKey(cfg, key)
		if result == nil {
			t.Fatal("expected key to match its own hash")
			return
		}
		if result.Name != "test" {
			t.Fatalf("expected name 'test', got %q", result.Name)
		}
	})
}

// TestPropertyValidateAPIKeyRejectsRandomKeys verifies that random strings
// not matching any configured hash are rejected.
func TestPropertyValidateAPIKeyRejectsRandomKeys(t *testing.T) {
	// Pre-generate a known key and its hash.
	knownKey := "known-static-key-for-property-test"
	knownHash, err := bcrypt.GenerateFromPassword([]byte(knownKey), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}

	cfg := &UsersConfig{
		ServiceAccounts: []ServiceAccount{
			{Name: "known", KeyHash: string(knownHash), Scope: "read-only"},
		},
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random key that is NOT the known key.
		candidate := rapid.StringOfN(rapid.RuneFrom(nil, printableASCII), 1, 72, -1).Draw(t, "candidate")
		if candidate == knownKey {
			t.Skip("generated the known key by chance, skip")
		}

		result := ValidateAPIKey(cfg, candidate)
		if result != nil {
			t.Fatalf("expected no match for random key %q, got %+v", candidate, result)
		}
	})
}

// TestPropertyMiddlewareOpenAccessPassesAll verifies that when UsersConfig
// is nil, all requests pass through regardless of headers.
func TestPropertyMiddlewareOpenAccessPassesAll(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(nil, testLogger(), okHandler)

	methods := []string{"GET", "POST", "PUT", "DELETE"}

	rapid.Check(t, func(t *rapid.T) {
		method := rapid.SampledFrom(methods).Draw(t, "method")
		path := "/api/" + rapid.StringOfN(rapid.RuneFrom(nil, lowercaseASCII), 1, 20, -1).Draw(t, "path")
		authHeader := rapid.StringOfN(rapid.RuneFrom(nil, printableASCII), 0, 100, -1).Draw(t, "auth")

		req := httptest.NewRequest(method, path, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("open access: expected 200 for %s %s, got %d", method, path, rec.Code)
		}
	})
}
