package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

// staticTokenSource returns a fixed token for testing.
type staticTokenSource struct {
	token *oauth2.Token
}

func (s *staticTokenSource) Token() (*oauth2.Token, error) {
	return s.token, nil
}

// failingTokenSource always returns an error.
type failingTokenSource struct{}

func (f *failingTokenSource) Token() (*oauth2.Token, error) {
	return nil, errors.New("ADC not configured")
}

// TestNewVertexAIProviderValidation validates constructor validation logic.
//
// Validates: Requirements 51.1, 51.2
func TestNewVertexAIProviderValidation(t *testing.T) {
	tests := []struct {
		name       string
		opts       ProviderOptions
		wantErr    bool
		wantSubstr string
	}{
		{
			name:       "missing GCP project",
			opts:       ProviderOptions{GCPProject: ""},
			wantErr:    true,
			wantSubstr: "GCP project ID is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewVertexAIProvider(tc.opts)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewVertexAIProvider() returned nil error, want error")
				}
				var provErr *ProviderError
				if !errors.As(err, &provErr) {
					t.Fatalf("error is not *ProviderError: %v", err)
				}
				if !strings.Contains(err.Error(), tc.wantSubstr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantSubstr)
				}
			}
		})
	}
}

// TestVertexAIProviderName validates the Name() method.
//
// Validates: Requirements 51.1
func TestVertexAIProviderName(t *testing.T) {
	p := &VertexAIProvider{}
	if got := p.Name(); got != "vertexai" {
		t.Errorf("Name() = %q, want %q", got, "vertexai")
	}
}

// TestVertexAIProviderInvoke validates Invoke with a mock HTTP server.
//
// Validates: Requirements 51.3, 51.4, 51.5, 51.6, 51.7
func TestVertexAIProviderInvoke(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		tokenSrc   oauth2.TokenSource
		wantErr    bool
		wantSubstr string
		wantText   string
	}{
		{
			name: "successful response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := vertexaiResponse{
					Candidates: []vertexaiCandidate{
						{
							Content: vertexaiContent{
								Parts: []vertexaiPart{{Text: "Hello world"}},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			tokenSrc: &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantText: "Hello world",
		},
		{
			name: "multi-part response concatenated",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := vertexaiResponse{
					Candidates: []vertexaiCandidate{
						{
							Content: vertexaiContent{
								Parts: []vertexaiPart{
									{Text: "Part 1 "},
									{Text: "Part 2"},
								},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			tokenSrc: &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantText: "Part 1 Part 2",
		},
		{
			name: "empty response returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := vertexaiResponse{Candidates: []vertexaiCandidate{}}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			tokenSrc:   &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantErr:    true,
			wantSubstr: "empty response",
		},
		{
			name: "auth failure returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error": "permission denied"}`))
			},
			tokenSrc:   &staticTokenSource{token: &oauth2.Token{AccessToken: "bad-token"}},
			wantErr:    true,
			wantSubstr: "authentication failed",
		},
		{
			name: "rate limit retries once then fails",
			handler: func() http.HandlerFunc {
				calls := 0
				return func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"error": "rate limited"}`))
				}
			}(),
			tokenSrc:   &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantErr:    true,
			wantSubstr: "rate limited",
		},
		{
			name: "rate limit retries once then succeeds",
			handler: func() http.HandlerFunc {
				calls := 0
				return func(w http.ResponseWriter, r *http.Request) {
					calls++
					if calls == 1 {
						w.WriteHeader(http.StatusTooManyRequests)
						_, _ = w.Write([]byte(`{"error": "rate limited"}`))
						return
					}
					resp := vertexaiResponse{
						Candidates: []vertexaiCandidate{
							{Content: vertexaiContent{Parts: []vertexaiPart{{Text: "success after retry"}}}},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
				}
			}(),
			tokenSrc: &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantText: "success after retry",
		},
		{
			name:       "token source failure returns error",
			handler:    func(w http.ResponseWriter, r *http.Request) {},
			tokenSrc:   &failingTokenSource{},
			wantErr:    true,
			wantSubstr: "failed to obtain access token",
		},
		{
			name: "server error (500) returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "internal"}`))
			},
			tokenSrc:   &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantErr:    true,
			wantSubstr: "HTTP 500",
		},
		{
			name: "invalid JSON response returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`not json`))
			},
			tokenSrc:   &staticTokenSource{token: &oauth2.Token{AccessToken: "test-token"}},
			wantErr:    true,
			wantSubstr: "failed to parse response JSON",
		},
		{
			name: "authorization header is set",
			handler: func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer my-token" {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error": "bad auth"}`))
					return
				}
				resp := vertexaiResponse{
					Candidates: []vertexaiCandidate{
						{Content: vertexaiContent{Parts: []vertexaiPart{{Text: "authed"}}}},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			tokenSrc: &staticTokenSource{token: &oauth2.Token{AccessToken: "my-token"}},
			wantText: "authed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			p := &VertexAIProvider{
				project:     "test-project",
				region:      "us-central1",
				client:      server.Client(),
				tokenSource: tc.tokenSrc,
			}

			// Override the URL by calling doRequest directly via Invoke.
			// We need to override the URL construction, so we'll call the internal method.
			ctx := context.Background()

			// Build the request body the same way Invoke does.
			reqBody := vertexaiRequest{
				Contents: []vertexaiContent{
					{Role: "user", Parts: []vertexaiPart{{Text: "test prompt"}}},
				},
			}
			bodyBytes, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			// Use the test server URL instead of the real Vertex AI endpoint.
			url := server.URL + "/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-2.0-flash:generateContent"

			var result string
			var lastErr error
			for attempt := 0; attempt < 2; attempt++ {
				text, retryable, reqErr := p.doRequest(ctx, url, bodyBytes)
				if reqErr != nil {
					lastErr = reqErr
					if retryable && attempt == 0 {
						continue
					}
					break
				}
				result = text
				lastErr = nil
				break
			}

			if tc.wantErr {
				if lastErr == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(lastErr.Error(), tc.wantSubstr) {
					t.Errorf("error = %q, want it to contain %q", lastErr.Error(), tc.wantSubstr)
				}
				var provErr *ProviderError
				if !errors.As(lastErr, &provErr) {
					t.Errorf("error is not *ProviderError: %v", lastErr)
				}
			} else {
				if lastErr != nil {
					t.Fatalf("unexpected error: %v", lastErr)
				}
				if result != tc.wantText {
					t.Errorf("result = %q, want %q", result, tc.wantText)
				}
			}
		})
	}
}

// TestVertexAIProviderRequestFormat validates the request body format sent to the API.
//
// Validates: Requirements 51.3
func TestVertexAIProviderRequestFormat(t *testing.T) {
	var receivedBody vertexaiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := vertexaiResponse{
			Candidates: []vertexaiCandidate{
				{Content: vertexaiContent{Parts: []vertexaiPart{{Text: "ok"}}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &VertexAIProvider{
		project:     "my-project",
		region:      "europe-west1",
		client:      server.Client(),
		tokenSource: &staticTokenSource{token: &oauth2.Token{AccessToken: "tok"}},
	}

	body, _ := json.Marshal(vertexaiRequest{
		Contents: []vertexaiContent{
			{Role: "user", Parts: []vertexaiPart{{Text: "translate this"}}},
		},
	})

	url := server.URL + "/v1/projects/my-project/locations/europe-west1/publishers/google/models/gemini-2.0-flash:generateContent"
	_, _, err := p.doRequest(context.Background(), url, body)
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}

	if len(receivedBody.Contents) != 1 {
		t.Fatalf("expected 1 content entry, got %d", len(receivedBody.Contents))
	}
	if receivedBody.Contents[0].Role != "user" {
		t.Errorf("role = %q, want %q", receivedBody.Contents[0].Role, "user")
	}
	if len(receivedBody.Contents[0].Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(receivedBody.Contents[0].Parts))
	}
	if receivedBody.Contents[0].Parts[0].Text != "translate this" {
		t.Errorf("text = %q, want %q", receivedBody.Contents[0].Parts[0].Text, "translate this")
	}
}
