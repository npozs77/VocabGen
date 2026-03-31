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

// AnthropicProvider implements Provider for the Anthropic Claude Messages API.
type AnthropicProvider struct {
	apiKey string
	client *http.Client
}

// NewAnthropicProvider creates an AnthropicProvider.
// An API key is always required for the Anthropic API.
func NewAnthropicProvider(opts ProviderOptions) (Provider, error) {
	if opts.APIKey == "" {
		return nil, &ProviderError{
			Provider: "anthropic",
			Message:  "API key is required: set ANTHROPIC_API_KEY environment variable or use --api-key flag",
		}
	}

	return &AnthropicProvider{
		apiKey: opts.APIKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Name returns the provider identifier.
func (p *AnthropicProvider) Name() string { return "anthropic" }

// anthropicRequest is the JSON body for the Anthropic Messages API.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

// anthropicMessage represents a single message in the Messages API request.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the JSON body returned by the Messages API.
type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

// anthropicContentBlock represents a single content block in the Messages API response.
type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Invoke sends a Messages API request and returns the text response.
// It retries once on HTTP 429 (rate limit) with a 1-second delay.
func (p *AnthropicProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	reqBody := anthropicRequest{
		Model:     modelID,
		MaxTokens: 4096,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "anthropic",
			Message:  "failed to marshal request: " + err.Error(),
			Err:      err,
		}
	}

	const url = "https://api.anthropic.com/v1/messages"

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Debug("anthropic: retrying after error", slog.Int("attempt", attempt+1), slog.String("error", lastErr.Error()))
			select {
			case <-ctx.Done():
				return "", &ProviderError{
					Provider: "anthropic",
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
func (p *AnthropicProvider) doRequest(ctx context.Context, url string, body []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", false, &ProviderError{
			Provider: "anthropic",
			Message:  "failed to create request: " + err.Error(),
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "anthropic",
			Message:  "request failed: " + err.Error(),
			Err:      err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "anthropic",
			Message:  "failed to read response body: " + err.Error(),
			Err:      err,
		}
	}

	// Rate limit — retryable.
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", true, &ProviderError{
			Provider: "anthropic",
			Message:  "rate limited (HTTP 429): retries exhausted",
		}
	}

	// Non-2xx status.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return "", false, &ProviderError{
			Provider: "anthropic",
			Message:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet),
		}
	}

	// Parse response JSON.
	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, &ProviderError{
			Provider: "anthropic",
			Message:  "failed to parse response JSON: " + err.Error(),
			Err:      err,
		}
	}

	if len(result.Content) == 0 || strings.TrimSpace(result.Content[0].Text) == "" {
		return "", false, &ProviderError{
			Provider: "anthropic",
			Message:  "empty response from model",
		}
	}

	return result.Content[0].Text, false, nil
}
