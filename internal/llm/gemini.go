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
)

// GeminiProvider implements Provider for the Google Gemini API (direct, via API key).
// Uses the generativelanguage.googleapis.com REST endpoint.
// Distinct from VertexAIProvider which uses GCP infrastructure and ADC.
type GeminiProvider struct {
	apiKey string
	client *http.Client
}

// NewGeminiProvider creates a GeminiProvider using a Gemini API key.
// Returns error if API key is empty.
func NewGeminiProvider(opts ProviderOptions) (Provider, error) {
	if opts.APIKey == "" {
		return nil, &ProviderError{
			Provider: "gemini",
			Message:  "API key is required: set GEMINI_API_KEY environment variable or use --api-key flag",
		}
	}

	return &GeminiProvider{
		apiKey: opts.APIKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Name returns the provider identifier.
func (p *GeminiProvider) Name() string { return "gemini" }

// geminiRequest is the JSON body for the Gemini generateContent endpoint.
type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

// geminiContent represents a content entry in the request.
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart represents a part within a content entry.
type geminiPart struct {
	Text string `json:"text"`
}

// geminiResponse is the JSON body returned by the generateContent endpoint.
type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

// geminiCandidate represents a single candidate in the response.
type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

// Invoke sends a generateContent request to the Gemini API and returns the text response.
// It retries once on HTTP 429 (rate limit) with a 1-second delay.
func (p *GeminiProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: prompt}},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "gemini",
			Message:  "failed to marshal request: " + err.Error(),
			Err:      err,
		}
	}

	// Gemini REST API endpoint with API key as query parameter.
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		modelID, p.apiKey,
	)

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Debug("gemini: retrying after error", slog.Int("attempt", attempt+1), slog.String("error", lastErr.Error()))
			select {
			case <-ctx.Done():
				return "", &ProviderError{
					Provider: "gemini",
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
func (p *GeminiProvider) doRequest(ctx context.Context, url string, body []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", false, &ProviderError{
			Provider: "gemini",
			Message:  "failed to create request: " + err.Error(),
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "gemini",
			Message:  "request failed: " + err.Error(),
			Err:      err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "gemini",
			Message:  "failed to read response body: " + err.Error(),
			Err:      err,
		}
	}

	// Rate limit — retryable.
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", true, &ProviderError{
			Provider: "gemini",
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
			Provider: "gemini",
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
			Provider: "gemini",
			Message:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet),
		}
	}

	// Parse response JSON.
	var result geminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, &ProviderError{
			Provider: "gemini",
			Message:  "failed to parse response JSON: " + err.Error(),
			Err:      err,
		}
	}

	// Extract text from the first candidate's parts.
	text := p.extractText(result)
	if text == "" {
		return "", false, &ProviderError{
			Provider: "gemini",
			Message:  "empty response from model",
		}
	}

	return text, false, nil
}

// extractText concatenates all text parts from the first candidate.
func (p *GeminiProvider) extractText(resp geminiResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	return strings.TrimSpace(sb.String())
}
