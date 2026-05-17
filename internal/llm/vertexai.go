package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VertexAIProvider implements Provider for Google Vertex AI (Gemini API).
// It uses Application Default Credentials (ADC) for authentication and
// calls the generateContent REST endpoint.
type VertexAIProvider struct {
	project     string
	region      string
	client      *http.Client
	tokenSource oauth2.TokenSource
}

// NewVertexAIProvider creates a VertexAIProvider using Google ADC.
// Requires a GCP project ID (via opts.GCPProject). Region defaults to "us-central1".
func NewVertexAIProvider(opts ProviderOptions) (Provider, error) {
	if opts.GCPProject == "" {
		return nil, &ProviderError{
			Provider: "vertexai",
			Message:  "GCP project ID is required: set GCP_PROJECT environment variable or use --gcp-project flag",
		}
	}

	region := opts.Region
	if region == "" {
		region = "us-central1"
	}

	// Use ADC to get credentials scoped to Vertex AI.
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, &ProviderError{
			Provider: "vertexai",
			Message:  "failed to find Google Application Default Credentials: " + err.Error(),
			Err:      err,
		}
	}

	return &VertexAIProvider{
		project:     opts.GCPProject,
		region:      region,
		client:      &http.Client{Timeout: 120 * time.Second},
		tokenSource: creds.TokenSource,
	}, nil
}

// Name returns the provider identifier.
func (p *VertexAIProvider) Name() string { return "vertexai" }

// vertexaiRequest is the JSON body for the Vertex AI generateContent endpoint.
type vertexaiRequest struct {
	Contents []vertexaiContent `json:"contents"`
}

// vertexaiContent represents a content entry in the request/response.
type vertexaiContent struct {
	Role  string         `json:"role,omitempty"`
	Parts []vertexaiPart `json:"parts"`
}

// vertexaiPart represents a part within a content entry.
type vertexaiPart struct {
	Text string `json:"text"`
}

// vertexaiResponse is the JSON body returned by the generateContent endpoint.
type vertexaiResponse struct {
	Candidates []vertexaiCandidate `json:"candidates"`
}

// vertexaiCandidate represents a single candidate in the response.
type vertexaiCandidate struct {
	Content vertexaiContent `json:"content"`
}

// Invoke sends a generateContent request to Vertex AI and returns the text response.
// It retries once on HTTP 429 (rate limit) with a 1-second delay.
func (p *VertexAIProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	reqBody := vertexaiRequest{
		Contents: []vertexaiContent{
			{
				Role:  "user",
				Parts: []vertexaiPart{{Text: prompt}},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "vertexai",
			Message:  "failed to marshal request: " + err.Error(),
			Err:      err,
		}
	}

	// Vertex AI generateContent endpoint.
	url := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.project, p.region, modelID,
	)

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Debug("vertexai: retrying after error", slog.Int("attempt", attempt+1), slog.String("error", lastErr.Error()))
			select {
			case <-ctx.Done():
				return "", &ProviderError{
					Provider: "vertexai",
					Message:  "context cancelled during retry",
					Err:      ctx.Err(),
				}
			case <-time.After(1 * time.Second):
			}
		}

		text, retryable, err := p.doRequest(ctx, url, bodyBytes)
		if err != nil {
			lastErr = err
			if retryable && attempt == 0 {
				continue
			}
			return "", err
		}
		return text, nil
	}

	return "", lastErr
}

// doRequest performs a single HTTP request and returns the extracted text,
// whether the error is retryable, and any error.
func (p *VertexAIProvider) doRequest(ctx context.Context, url string, body []byte) (string, bool, error) {
	// Get a fresh OAuth2 token.
	token, err := p.tokenSource.Token()
	if err != nil {
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  "failed to obtain access token: " + err.Error(),
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  "failed to create request: " + err.Error(),
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  "request failed: " + err.Error(),
			Err:      err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  "failed to read response body: " + err.Error(),
			Err:      err,
		}
	}

	// Rate limit — retryable.
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", true, &ProviderError{
			Provider: "vertexai",
			Message:  "rate limited (HTTP 429): retries exhausted",
		}
	}

	// Auth failure.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  fmt.Sprintf("authentication failed (HTTP %d): %s", resp.StatusCode, snippet),
		}
	}

	// Non-2xx status.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet),
		}
	}

	// Parse response JSON.
	var result vertexaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  "failed to parse response JSON: " + err.Error(),
			Err:      err,
		}
	}

	// Extract text from the first candidate's parts.
	text := p.extractText(result)
	if text == "" {
		return "", false, &ProviderError{
			Provider: "vertexai",
			Message:  "empty response from model",
		}
	}

	return text, false, nil
}

// extractText concatenates all text parts from the first candidate.
func (p *VertexAIProvider) extractText(resp vertexaiResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	return strings.TrimSpace(sb.String())
}
