package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	return NewServer(store, &cfg, slog.Default(), "test", "unknown", "go1.22", "/tmp/test.db")
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
