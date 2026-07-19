package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/vocabgen/internal/config"
)

// tagMockStore embeds stubStore and overrides ListDistinctTags for testing.
type tagMockStore struct {
	stubStore
	tags []string
}

func (m *tagMockStore) ListDistinctTags(_ context.Context) ([]string, error) {
	return m.tags, nil
}

func newTagTestServer(store *tagMockStore) *Server {
	cfg := config.DefaultConfig()
	return NewServer(store, &cfg, slog.Default(), "test", "unknown", "go1.22", "/tmp/test.db", nil)
}

func TestHandleListTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		wantLen  int
		wantTags []string
	}{
		{
			name:     "empty database returns empty array",
			tags:     []string{},
			wantLen:  0,
			wantTags: []string{},
		},
		{
			name:     "returns all distinct tags",
			tags:     []string{"B2", "chapter-1", "HS2.2"},
			wantLen:  3,
			wantTags: []string{"B2", "chapter-1", "HS2.2"},
		},
		{
			name:     "single tag",
			tags:     []string{"exam-prep"},
			wantLen:  1,
			wantTags: []string{"exam-prep"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &tagMockStore{tags: tc.tags}
			srv := newTagTestServer(store)

			req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json; charset=utf-8" {
				t.Fatalf("expected application/json; charset=utf-8, got %q", ct)
			}

			var got []string
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatalf("decode json: %v", err)
			}

			if len(got) != tc.wantLen {
				t.Fatalf("expected %d tags, got %d", tc.wantLen, len(got))
			}

			for i, want := range tc.wantTags {
				if i >= len(got) {
					break
				}
				if got[i] != want {
					t.Errorf("tag[%d]: expected %q, got %q", i, want, got[i])
				}
			}
		})
	}
}

// --- Tests for handleDatabasePage (Issue #99) ---

func TestHandleDatabasePage_InitialTags(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		tags         []string // tags available in the store
		wantContains string   // substring expected in HTML response
	}{
		{
			name:         "no params renders page without initial tags",
			url:          "/database",
			tags:         []string{"HS2.1", "HS2.2", "HS3.1"},
			wantContains: `data-initial-tags=""`,
		},
		{
			name:         "tags param passes through directly",
			url:          "/database?tags=HS2.1,HS2.2",
			tags:         []string{"HS2.1", "HS2.2", "HS3.1"},
			wantContains: `data-initial-tags="HS2.1,HS2.2"`,
		},
		{
			name:         "prefix param resolves to matching tags",
			url:          "/database?prefix=HS2",
			tags:         []string{"HS2.1", "HS2.2", "HS3.1"},
			wantContains: `data-initial-tags="HS2.1,HS2.2"`,
		},
		{
			name:         "prefix with no matches results in empty initial tags",
			url:          "/database?prefix=HS9",
			tags:         []string{"HS2.1", "HS3.1"},
			wantContains: `data-initial-tags=""`,
		},
		{
			name:         "prefix does not match without dot separator",
			url:          "/database?prefix=HS2",
			tags:         []string{"HS21", "HS2.1"},
			wantContains: `data-initial-tags="HS2.1"`,
		},
		{
			name:         "tags takes priority over prefix when both provided",
			url:          "/database?tags=HS3.1&prefix=HS2",
			tags:         []string{"HS2.1", "HS2.2", "HS3.1"},
			wantContains: `data-initial-tags="HS3.1"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &tagMockStore{tags: tc.tags}
			srv := newTagTestServer(store)

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			body := w.Body.String()
			if !strings.Contains(body, tc.wantContains) {
				t.Errorf("response body does not contain %q\nbody (first 500): %.500s", tc.wantContains, body)
			}
		})
	}
}

func TestHandleListWords_PrefixParam(t *testing.T) {
	// Verify that /api/words?prefix=HS2 returns the prefix in context (via parseListParams).
	// This is a smoke test that the param flows through — full filtering is tested in db_test.go.
	store := &tagMockStore{tags: []string{"HS2.1", "HS2.2"}}
	srv := newTagTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/words?prefix=HS2", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Should return 200 (JSON or HTML partial depending on headers).
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
