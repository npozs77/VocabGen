package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/user/vocabgen/internal/config"
	"github.com/user/vocabgen/internal/db"
)

// TestMain sets up a temp config dir for all web tests so SaveConfig
// never touches the real ~/.vocabgen/config.yaml.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "vocabgen-web-test-*")
	if err != nil {
		panic(err)
	}
	config.SetConfigDirForTest(tmpDir)
	code := m.Run()
	config.SetConfigDirForTest("")
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// stubStore implements db.Store with no-op methods for testing.
type stubStore struct{}

func (s *stubStore) FindWord(ctx context.Context, word, sourceLang string) (*db.WordRow, error) {
	return nil, nil
}
func (s *stubStore) FindExpression(ctx context.Context, expr, sourceLang string) (*db.ExpressionRow, error) {
	return nil, nil
}
func (s *stubStore) GetWord(ctx context.Context, id int64) (*db.WordRow, error) {
	return &db.WordRow{ID: id, Word: "stub", SourceLanguage: "nl", TargetLanguage: "hu", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}, nil
}
func (s *stubStore) GetExpression(ctx context.Context, id int64) (*db.ExpressionRow, error) {
	return &db.ExpressionRow{ID: id, Expression: "stub", SourceLanguage: "nl", TargetLanguage: "hu", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}, nil
}
func (s *stubStore) FindWords(ctx context.Context, word, sourceLang string) ([]db.WordRow, error) {
	return nil, nil
}
func (s *stubStore) FindExpressions(ctx context.Context, expr, sourceLang string) ([]db.ExpressionRow, error) {
	return nil, nil
}
func (s *stubStore) InsertWord(ctx context.Context, row *db.WordRow) error { return nil }
func (s *stubStore) InsertExpression(ctx context.Context, row *db.ExpressionRow) error {
	return nil
}
func (s *stubStore) ListWords(ctx context.Context, filter db.ListFilter) ([]db.WordRow, int, error) {
	return nil, 0, nil
}
func (s *stubStore) ListExpressions(ctx context.Context, filter db.ListFilter) ([]db.ExpressionRow, int, error) {
	return nil, 0, nil
}
func (s *stubStore) UpdateWord(ctx context.Context, id int64, row *db.WordRow) error { return nil }
func (s *stubStore) UpdateExpression(ctx context.Context, id int64, row *db.ExpressionRow) error {
	return nil
}
func (s *stubStore) DeleteWord(ctx context.Context, id int64) error       { return nil }
func (s *stubStore) DeleteExpression(ctx context.Context, id int64) error { return nil }
func (s *stubStore) ImportWords(ctx context.Context, rows []db.WordRow) (int, int, int, error) {
	return 0, 0, 0, nil
}
func (s *stubStore) ImportExpressions(ctx context.Context, rows []db.ExpressionRow) (int, int, int, error) {
	return 0, 0, 0, nil
}
func (s *stubStore) Close() error                                        { return nil }
func (s *stubStore) BackupTo(ctx context.Context, destPath string) error { return nil }
func (s *stubStore) RestoreFrom(ctx context.Context, srcPath string) error {
	return nil
}

func newTestServer() *Server {
	cfg := config.DefaultConfig()
	return NewServer(&stubStore{}, &cfg, slog.Default(), "test", "unknown", "go1.22")
}

func TestNewServer(t *testing.T) {
	srv := newTestServer()
	if srv.store == nil {
		t.Fatal("store should not be nil")
	}
	if srv.cfg == nil {
		t.Fatal("cfg should not be nil")
	}
	if srv.mux == nil {
		t.Fatal("mux should not be nil")
	}
	if srv.logger == nil {
		t.Fatal("logger should not be nil")
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Fatalf("expected application/json; charset=utf-8, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestPageRoutes(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name string
		path string
	}{
		{"lookup", "/"},
		{"batch", "/batch"},
		{"config", "/config"},
		{"database", "/database"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			ct := w.Header().Get("Content-Type")
			if ct != "text/html; charset=utf-8" {
				t.Fatalf("expected text/html; charset=utf-8, got %q", ct)
			}
		})
	}
}

func TestAPIRoutesRegistered(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int // 0 means "any non-404/405"
	}{
		{"get-config", http.MethodGet, "/api/config", http.StatusOK},
		{"config-html", http.MethodGet, "/api/config/html", http.StatusOK},
		{"languages", http.MethodGet, "/api/languages", http.StatusOK},
		{"health", http.MethodGet, "/api/health", http.StatusOK},
		{"words", http.MethodGet, "/api/words", http.StatusOK},
		{"expressions", http.MethodGet, "/api/expressions", http.StatusOK},
		{"batch-stream", http.MethodGet, "/api/batch/stream", http.StatusOK},
		// POST without body → expect 400 (bad request) not 404/501
		{"lookup", http.MethodPost, "/api/lookup", http.StatusBadRequest},
		{"lookup-html", http.MethodPost, "/api/lookup/html", 0},
		{"lookup-resolve", http.MethodPost, "/api/lookup/resolve", http.StatusBadRequest},
		{"lookup-resolve-html", http.MethodPost, "/api/lookup/resolve/html", 0},
		// Multipart endpoints without body → 413 (MaxBytesReader on nil body)
		{"batch", http.MethodPost, "/api/batch", 0},
		{"batch-html", http.MethodPost, "/api/batch/html", 0},
		{"import", http.MethodPost, "/api/import", 0},
		// PUT/DELETE with stub store → 200
		{"put-word", http.MethodPut, "/api/words/1", http.StatusOK},
		{"put-expression", http.MethodPut, "/api/expressions/1", http.StatusOK},
		{"delete-word", http.MethodDelete, "/api/words/1", http.StatusOK},
		{"delete-expression", http.MethodDelete, "/api/expressions/1", http.StatusOK},
		// Export with empty store → 200 (empty xlsx)
		{"export", http.MethodGet, "/api/export", http.StatusOK},
		// Config update with empty body
		{"put-config", http.MethodPut, "/api/config", 0},
		// Test connection with no provider configured
		{"test-connection", http.MethodPost, "/api/test-connection", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if tc.wantStatus != 0 {
				if w.Code != tc.wantStatus {
					t.Fatalf("expected %d, got %d", tc.wantStatus, w.Code)
				}
			} else {
				// Just verify the route is registered (not 404 or 405)
				if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
					t.Fatalf("route not registered: got %d", w.Code)
				}
			}
		})
	}
}

func TestListenAndServe_GracefulShutdown(t *testing.T) {
	srv := newTestServer()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx, ":0")
	}()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger graceful shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error from graceful shutdown, got: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("shutdown did not complete within timeout")
	}
}
